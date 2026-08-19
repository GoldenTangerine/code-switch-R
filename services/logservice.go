package services

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const timeLayout = "2006-01-02 15:04:05"
const requestLogUnreadWhereClause = "TRIM(COALESCE(error_read_at, '')) = ''"

const providerPerformanceCacheTTL = 20 * time.Second
const providerPerformanceCacheMaxEntries = 512
const providerLatestPerformanceSampleLimit = 5
const providerPerformanceTrendBucketSize = 15 * time.Minute
// 兼容夏令时回拨产生的 25 小时自然日，同时限制异常范围的内存占用。
const providerPerformanceTrendMaxBucketCount = 100
const fiveHourQuotaWindowDuration = 5 * time.Hour

var requestLogListSelectFields = []string{
	"id",
	"platform",
	"model",
	"requested_model",
	"mapped_model",
	"model_mapping_pattern",
	"model_mapping_target",
	"model_override",
	"model_route_captured",
	"session_preferred_provider_id",
	"session_preferred_provider",
	"session_provider_route",
	"session_identity_source",
	"response_model",
	"reasoning_effort",
	"reasoning_effort_source",
	"user_agent",
	"provider_id",
	"provider",
	"price_source",
	"http_code",
	"request_outcome",
	"outcome_reason",
	"input_tokens",
	"output_tokens",
	"cache_create_tokens",
	"ephemeral_5m_tokens",
	"ephemeral_1h_tokens",
	"cache_read_tokens",
	"reasoning_tokens",
	"is_stream",
	"duration_sec",
	"first_token_sec",
	"proxy_prepare_ms",
	"dns_ms",
	"connect_ms",
	"tls_ms",
	"upstream_ttfb_ms",
	"proxy_stream_delay_ms",
	"connection_reused",
	"stream_last_event",
	"stream_terminal_event",
	"stream_error_kind",
	"error_message",
	"error_source",
	"stream_compaction_requested",
	"stream_compaction_observed",
	"stream_bytes",
	"upstream_protocol",
	"input_cost",
	"output_cost",
	"reasoning_cost",
	"cache_create_cost",
	"cache_read_cost",
	"ephemeral_5m_cost",
	"ephemeral_1h_cost",
	"total_cost",
	"group_multiplier",
	"has_pricing",
	"matched_pricing_model",
	"provider_pricing_available",
	"provider_quota_type",
	"provider_input_usd_per_m",
	"provider_output_usd_per_m",
	"provider_per_call_unified",
	"provider_per_call_input",
	"provider_per_call_output",
	"provider_per_call_unified_set",
	"provider_per_call_input_set",
	"provider_per_call_output_set",
	"error_read_at",
	"data_source",
	"source_record_id",
	"session_id",
	"dedup_core",
	"created_at",
}

var requestLogFailureListSelectFields = append(
	append([]string{}, requestLogListSelectFields...),
	"response_body",
	"response_body_truncated",
)

var requestLogPayloadDetailSelectFields = []string{
	"id",
	"request_body",
	"response_body",
	"request_body_truncated",
	"response_body_truncated",
}

type LogService struct {
	modelPricing *ModelPricingService

	providerPerformanceCacheMu sync.Mutex
	providerPerformanceCache   map[string]providerPerformanceCacheEntry
}

type providerPerformanceStat struct {
	ProviderID       string
	Provider         string
	AvgFirstTokenSec float64
	AvgTokensPerSec  float64
	TTFTSampleCount  int64
	TPSSampleCount   int64
}

type providerPerformanceCacheEntry struct {
	ExpiresAt time.Time
	Stats     map[string]providerPerformanceStat
}

type requestLogSelecter interface {
	Selects(...xdb.Option) ([]xdb.Record, error)
}

type requestLogCounter interface {
	Count(...xdb.Option) (int64, error)
}

type requestLogSelectCounter interface {
	requestLogSelecter
	requestLogCounter
}

type RequestLogPageResult struct {
	Items  []ReqeustLog `json:"items"`
	Total  int64        `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type RequestLogPayloadDetail struct {
	ID                    int64  `json:"id"`
	RequestBody           string `json:"request_body"`
	ResponseBody          string `json:"response_body"`
	RequestBodyTruncated  bool   `json:"request_body_truncated"`
	ResponseBodyTruncated bool   `json:"response_body_truncated"`
}

type FiveHourQuotaStatus struct {
	Active      bool    `json:"active"`
	WindowStart string  `json:"window_start"`
	NextReset   string  `json:"next_reset"`
	Used        float64 `json:"used"`
}

func (ls *LogService) CostSince(start string, platform string) (float64, error) {
	startTime, err := parseTimeInput(start)
	if err != nil {
		return 0, err
	}

	db, err := xdb.DB("default")
	if err != nil {
		return 0, err
	}

	query := `SELECT COALESCE(SUM(total_cost), 0) FROM request_log WHERE created_at >= ? AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	args := []interface{}{startTime.UTC().Format(timeLayout)}
	if platform != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}

	total := 0.0
	if err := db.QueryRow(query, args...).Scan(&total); err != nil {
		if isNoSuchTableErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}

func (ls *LogService) ResolveFiveHourQuotaStatus(platform string) (FiveHourQuotaStatus, error) {
	return ls.resolveFiveHourQuotaStatusAt(platform, time.Now())
}

func (ls *LogService) resolveFiveHourQuotaStatusAt(platform string, now time.Time) (FiveHourQuotaStatus, error) {
	status := FiveHourQuotaStatus{}

	db, err := xdb.DB("default")
	if err != nil {
		return status, err
	}

	state, err := queryFiveHourQuotaCycleState(db, platform)
	if err != nil {
		if isNoSuchTableErr(err) {
			return status, nil
		}
		return status, err
	}
	if state.WindowStart.IsZero() || state.NextReset.IsZero() {
		return status, nil
	}

	if !now.UTC().Before(state.NextReset) {
		return status, nil
	}

	status.Active = true
	status.WindowStart = state.WindowStart.In(time.Local).Format(timeLayout)
	status.NextReset = state.NextReset.In(time.Local).Format(timeLayout)
	status.Used = normalizeBudgetRawUsed(state.Used)
	return status, nil
}

func queryLatestRequestLogCreatedAt(db *sql.DB, platform string) (time.Time, error) {
	query := `SELECT MAX(created_at) FROM request_log WHERE ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	args := make([]interface{}, 0, 1)
	if strings.TrimSpace(platform) != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}

	var raw sql.NullString
	if err := db.QueryRow(query, args...).Scan(&raw); err != nil {
		return time.Time{}, err
	}
	return parseStoredRequestLogTime(raw)
}

func queryLatestFiveHourQuotaWindowStart(db *sql.DB, platform string) (time.Time, error) {
	initialWhere := ` WHERE ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	nextWhere := ` AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	args := make([]interface{}, 0, 2)
	if strings.TrimSpace(platform) != "" {
		initialWhere += ` AND platform = ?`
		nextWhere += ` AND platform = ?`
		args = append(args, platform, platform)
	}

	query := `
		WITH RECURSIVE cycle_starts(start_at) AS (
			SELECT MIN(created_at) FROM request_log` + initialWhere + `
			UNION ALL
			SELECT (
				SELECT MIN(created_at)
				FROM request_log
				WHERE created_at >= datetime(cycle_starts.start_at, '+5 hours')` + nextWhere + `
			)
			FROM cycle_starts
			WHERE cycle_starts.start_at IS NOT NULL
		)
		SELECT start_at
		FROM cycle_starts
		WHERE start_at IS NOT NULL
		ORDER BY start_at DESC
		LIMIT 1
	`

	var raw sql.NullString
	if err := db.QueryRow(query, args...).Scan(&raw); err != nil {
		return time.Time{}, err
	}
	return parseStoredRequestLogTime(raw)
}

func queryRequestLogCostBetween(db *sql.DB, platform string, start time.Time, end time.Time) (float64, error) {
	query := `SELECT COALESCE(SUM(total_cost), 0) FROM request_log WHERE created_at >= ? AND created_at < ? AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	args := []interface{}{start.UTC().Format(timeLayout), end.UTC().Format(timeLayout)}
	if strings.TrimSpace(platform) != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}

	total := 0.0
	if err := db.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func parseStoredRequestLogTime(value sql.NullString) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, nil
	}

	raw := strings.TrimSpace(value.String)
	if raw == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		timeLayout,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05-0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), nil
		}
		if parsed, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return parsed.UTC(), nil
		}
	}

	if normalized := strings.Replace(raw, " ", "T", 1); normalized != raw {
		if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
			return parsed.UTC(), nil
		}
	}

	if len(raw) >= len("2006-01-02") {
		if parsed, err := time.ParseInLocation("2006-01-02", raw[:10], time.UTC); err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid request_log created_at: %s", raw)
}

func NewLogService(modelPricing *ModelPricingService) *LogService {
	return &LogService{
		modelPricing:             modelPricing,
		providerPerformanceCache: make(map[string]providerPerformanceCacheEntry),
	}
}

func (ls *LogService) GetRequestLogPayload(id int64) (RequestLogPayloadDetail, error) {
	if id <= 0 {
		return RequestLogPayloadDetail{}, fmt.Errorf("invalid log id")
	}
	model := xdb.New("request_log")
	records, err := model.Selects(
		xdb.Field(requestLogPayloadDetailSelectFields...),
		xdb.WhereEq("id", id),
		xdb.Limit(1),
	)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return RequestLogPayloadDetail{}, nil
		}
		return RequestLogPayloadDetail{}, err
	}
	if len(records) == 0 {
		return RequestLogPayloadDetail{}, nil
	}
	record := records[0]
	return RequestLogPayloadDetail{
		ID:                    record.GetInt64("id"),
		RequestBody:           record.GetString("request_body"),
		ResponseBody:          record.GetString("response_body"),
		RequestBodyTruncated:  record.GetBool("request_body_truncated"),
		ResponseBodyTruncated: record.GetBool("response_body_truncated"),
	}, nil
}

func selectRecordsByProviderRef(selecter requestLogSelecter, baseOptions []xdb.Option, providerRef string) ([]xdb.Record, error) {
	if selecter == nil {
		return nil, fmt.Errorf("nil selecter")
	}
	providerRef = strings.TrimSpace(providerRef)
	if providerRef == "" {
		records, err := selecter.Selects(baseOptions...)
		if errors.Is(err, xdb.ErrNotFound) {
			return []xdb.Record{}, nil
		}
		return records, err
	}

	byIDOptions := append([]xdb.Option{}, baseOptions...)
	byIDOptions = append(byIDOptions, xdb.WhereEq("provider_id", providerRef))
	records, err := selecter.Selects(byIDOptions...)
	if err == nil && len(records) > 0 {
		return records, nil
	}
	if err != nil && !errors.Is(err, xdb.ErrNotFound) && !isNoSuchTableErr(err) {
		return nil, err
	}

	byNameOptions := append([]xdb.Option{}, baseOptions...)
	byNameOptions = append(byNameOptions, xdb.WhereEq("provider", providerRef))
	records, err = selecter.Selects(byNameOptions...)
	if errors.Is(err, xdb.ErrNotFound) {
		return []xdb.Record{}, nil
	}
	return records, err
}

func countRecordsByProviderRef(counter requestLogCounter, baseOptions []xdb.Option, providerRef string) (int64, error) {
	if counter == nil {
		return 0, fmt.Errorf("nil counter")
	}
	providerRef = strings.TrimSpace(providerRef)
	if providerRef == "" {
		total, err := counter.Count(baseOptions...)
		if errors.Is(err, xdb.ErrNotFound) {
			return 0, nil
		}
		return total, err
	}

	byIDOptions := append([]xdb.Option{}, baseOptions...)
	byIDOptions = append(byIDOptions, xdb.WhereEq("provider_id", providerRef))
	total, err := counter.Count(byIDOptions...)
	if err == nil && total > 0 {
		return total, nil
	}
	if err != nil && !errors.Is(err, xdb.ErrNotFound) && !isNoSuchTableErr(err) {
		return 0, err
	}

	byNameOptions := append([]xdb.Option{}, baseOptions...)
	byNameOptions = append(byNameOptions, xdb.WhereEq("provider", providerRef))
	total, err = counter.Count(byNameOptions...)
	if errors.Is(err, xdb.ErrNotFound) {
		return 0, nil
	}
	return total, err
}

func normalizeRequestLogListLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

func normalizeRequestLogListOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func buildPricingModelFilterOption(pricingModel string) xdb.Option {
	pricingModel = strings.TrimSpace(pricingModel)
	return xdb.Where(
		"COALESCE(NULLIF(TRIM(matched_pricing_model), ''), TRIM(model))",
		"=",
		pricingModel,
	)
}

func buildRequestLogFilterOptions(platform string, pricingModel string, startAt string, endAt string) ([]xdb.Option, error) {
	return buildRequestLogFilterOptionsV3(platform, pricingModel, startAt, endAt, LogDataSourceModeProxy)
}

func buildRequestLogFilterOptionsV3(platform string, pricingModel string, startAt string, endAt string, sourceMode LogDataSourceMode) ([]xdb.Option, error) {
	options := make([]xdb.Option, 0, 4)
	options = append(options, requestLogSourceFilterOption(sourceMode))
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	if strings.TrimSpace(pricingModel) != "" {
		options = append(options, buildPricingModelFilterOption(pricingModel))
	}
	if strings.TrimSpace(startAt) != "" {
		parsed, err := parseTimeInput(startAt)
		if err != nil {
			return nil, err
		}
		options = append(options, xdb.WhereGte("created_at", parsed.UTC().Format(timeLayout)))
	}
	if strings.TrimSpace(endAt) != "" {
		parsed, err := parseTimeInput(endAt)
		if err != nil {
			return nil, err
		}
		options = append(options, xdb.WhereLt("created_at", parsed.UTC().Format(timeLayout)))
	}
	return options, nil
}

func buildFailedRequestLogFilterOptions(platform string, startAt string, endAt string) ([]xdb.Option, error) {
	options, err := buildRequestLogFilterOptions(platform, "", startAt, endAt)
	if err != nil {
		return nil, err
	}
	return append(options, xdb.WhereRaw(requestLogFailureWhereClause(""))), nil
}

func requestLogFailureWhereClause(tableAlias string) string {
	prefix := ""
	if strings.TrimSpace(tableAlias) != "" {
		prefix = strings.TrimSpace(tableAlias) + "."
	}
	outcome, fallback := requestLogOutcomeSQLParts(prefix + "request_outcome")
	return "(" + outcome + " = '" + requestOutcomeFailure + "' OR (" + fallback + " AND COALESCE(" + prefix + "http_code, 0) >= 400))"
}

func requestLogSuccessWhereClause(tableAlias string, legacyMinimumCode int) string {
	prefix := ""
	if strings.TrimSpace(tableAlias) != "" {
		prefix = strings.TrimSpace(tableAlias) + "."
	}
	outcome, fallback := requestLogOutcomeSQLParts(prefix + "request_outcome")
	return fmt.Sprintf("(%s = '%s' OR (%s AND COALESCE(%shttp_code, 0) >= %d AND COALESCE(%shttp_code, 0) < 400))", outcome, requestOutcomeSuccess, fallback, prefix, legacyMinimumCode, prefix)
}

func requestLogOutcomeSQLParts(column string) (string, string) {
	outcome := "TRIM(COALESCE(" + column + ", ''))"
	fallback := outcome + " NOT IN ('" + requestOutcomeSuccess + "', '" + requestOutcomeFailure + "', '" + requestOutcomeExcluded + "')"
	return outcome, fallback
}

func normalizedRequestLogOutcome(value string) string {
	switch strings.TrimSpace(value) {
	case requestOutcomeSuccess:
		return requestOutcomeSuccess
	case requestOutcomeFailure:
		return requestOutcomeFailure
	case requestOutcomeExcluded:
		return requestOutcomeExcluded
	default:
		return ""
	}
}

func resolvedRequestLogOutcome(logEntry ReqeustLog) string {
	outcome := normalizedRequestLogOutcome(logEntry.RequestOutcome)
	switch outcome {
	case requestOutcomeSuccess, requestOutcomeFailure, requestOutcomeExcluded:
		return outcome
	default:
		if logEntry.HttpCode >= 400 {
			return requestOutcomeFailure
		}
		return requestOutcomeSuccess
	}
}

func buildRequestLogList(records []xdb.Record, pricingSnapshot *modelpricing.Service) []ReqeustLog {
	logs := make([]ReqeustLog, 0, len(records))
	for _, record := range records {
		createdAtLocal, _ := parseCreatedAt(record)
		createdAtValue := record.GetString("created_at")
		if !createdAtLocal.IsZero() {
			createdAtValue = createdAtLocal.Format(timeLayout)
		}
		logEntry := ReqeustLog{
			ID:                         record.GetInt64("id"),
			Platform:                   record.GetString("platform"),
			Model:                      record.GetString("model"),
			RequestedModel:             record.GetString("requested_model"),
			MappedModel:                record.GetString("mapped_model"),
			ModelMappingPattern:        record.GetString("model_mapping_pattern"),
			ModelMappingTarget:         record.GetString("model_mapping_target"),
			ModelOverride:              record.GetString("model_override"),
			ModelRouteCaptured:         record.GetBool("model_route_captured"),
			SessionPreferredProviderID: record.GetString("session_preferred_provider_id"),
			SessionPreferredProvider:   record.GetString("session_preferred_provider"),
			SessionProviderRoute:       record.GetString("session_provider_route"),
			SessionIdentitySource:      record.GetString("session_identity_source"),
			ResponseModel:              record.GetString("response_model"),
			ReasoningEffort:            record.GetString("reasoning_effort"),
			ReasoningEffortSource:      record.GetString("reasoning_effort_source"),
			UserAgent:                  record.GetString("user_agent"),
			ProviderID:                 record.GetString("provider_id"),
			Provider:                   record.GetString("provider"),
			PriceSource:                record.GetString("price_source"),
			HttpCode:                   record.GetInt("http_code"),
			RequestOutcome:             record.GetString("request_outcome"),
			OutcomeReason:              record.GetString("outcome_reason"),
			InputTokens:                record.GetInt("input_tokens"),
			OutputTokens:               record.GetInt("output_tokens"),
			CacheCreateTokens:          record.GetInt("cache_create_tokens"),
			Ephemeral5mTokens:          record.GetInt("ephemeral_5m_tokens"),
			Ephemeral1hTokens:          record.GetInt("ephemeral_1h_tokens"),
			CacheReadTokens:            record.GetInt("cache_read_tokens"),
			ReasoningTokens:            record.GetInt("reasoning_tokens"),
			CreatedAt:                  createdAtValue,
			IsStream:                   record.GetBool("is_stream"),
			DurationSec:                record.GetFloat64("duration_sec"),
			FirstTokenSec:              record.GetFloat64("first_token_sec"),
			ProxyPrepareMs:             record.GetFloat64("proxy_prepare_ms"),
			DNSMs:                      record.GetFloat64("dns_ms"),
			ConnectMs:                  record.GetFloat64("connect_ms"),
			TLSMs:                      record.GetFloat64("tls_ms"),
			UpstreamTTFBMs:             record.GetFloat64("upstream_ttfb_ms"),
			ProxyStreamDelayMs:         record.GetFloat64("proxy_stream_delay_ms"),
			ConnectionReused:           record.GetBool("connection_reused"),
			StreamLastEvent:            record.GetString("stream_last_event"),
			StreamTerminalEvent:        record.GetString("stream_terminal_event"),
			StreamErrorKind:            record.GetString("stream_error_kind"),
			ErrorMessage:               record.GetString("error_message"),
			ErrorSource:                record.GetString("error_source"),
			StreamCompactionRequested:  record.GetBool("stream_compaction_requested"),
			StreamCompactionObserved:   record.GetBool("stream_compaction_observed"),
			StreamBytes:                record.GetInt64("stream_bytes"),
			UpstreamProtocol:           record.GetString("upstream_protocol"),
			InputCost:                  record.GetFloat64("input_cost"),
			OutputCost:                 record.GetFloat64("output_cost"),
			ReasoningCost:              record.GetFloat64("reasoning_cost"),
			CacheCreateCost:            record.GetFloat64("cache_create_cost"),
			CacheReadCost:              record.GetFloat64("cache_read_cost"),
			Ephemeral5mCost:            record.GetFloat64("ephemeral_5m_cost"),
			Ephemeral1hCost:            record.GetFloat64("ephemeral_1h_cost"),
			TotalCost:                  record.GetFloat64("total_cost"),
			GroupMultiplier:            record.GetFloat64("group_multiplier"),
			HasPricing:                 record.GetBool("has_pricing"),
			MatchedPricingModel:        record.GetString("matched_pricing_model"),
			EffectivePricingModel:      resolveStoredPricingModel(record.GetString("matched_pricing_model"), record.GetString("model")),
			ProviderPricingAvailable:   record.GetBool("provider_pricing_available"),
			ProviderQuotaType:          record.GetInt("provider_quota_type"),
			ProviderInputUSDPerM:       record.GetFloat64("provider_input_usd_per_m"),
			ProviderOutputUSDPerM:      record.GetFloat64("provider_output_usd_per_m"),
			ProviderPerCallUnified:     record.GetFloat64("provider_per_call_unified"),
			ProviderPerCallInput:       record.GetFloat64("provider_per_call_input"),
			ProviderPerCallOutput:      record.GetFloat64("provider_per_call_output"),
			ProviderPerCallUnifiedSet:  record.GetBool("provider_per_call_unified_set"),
			ProviderPerCallInputSet:    record.GetBool("provider_per_call_input_set"),
			ProviderPerCallOutputSet:   record.GetBool("provider_per_call_output_set"),
			ErrorReadAt:                record.GetString("error_read_at"),
			DataSource:                 record.GetString("data_source"),
			SourceRecordID:             record.GetString("source_record_id"),
			SessionID:                  record.GetString("session_id"),
			DedupCore:                  record.GetString("dedup_core"),
			ResponseBody:               record.GetString("response_body"),
			ResponseBodyTruncated:      record.GetBool("response_body_truncated"),
		}
		applyLogPricing(pricingSnapshot, &logEntry)
		logs = append(logs, logEntry)
	}
	return logs
}

func parseRequestLogLocalTime(value string) (time.Time, bool) {
	parsed, err := parseTimeInput(value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func (ls *LogService) loadRequestLogsForAggregation(
	platform string,
	provider string,
	pricingModel string,
	start time.Time,
	end time.Time,
) ([]ReqeustLog, error) {
	return ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, start, end, LogDataSourceModeProxy)
}

func (ls *LogService) loadRequestLogsForAggregationV3(
	platform string,
	provider string,
	pricingModel string,
	start time.Time,
	end time.Time,
	sourceMode LogDataSourceMode,
) ([]ReqeustLog, error) {
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.Field(requestLogListSelectFields...),
		xdb.WhereGte("created_at", start.UTC().Format(timeLayout)),
		xdb.WhereLt("created_at", end.UTC().Format(timeLayout)),
		requestLogSourceFilterOption(sourceMode),
	}
	if strings.TrimSpace(platform) != "" {
		options = append(options, xdb.WhereEq("platform", strings.TrimSpace(platform)))
	}
	if strings.TrimSpace(pricingModel) != "" {
		options = append(options, buildPricingModelFilterOption(pricingModel))
	}
	records, err := selectRecordsByProviderRef(model, options, provider)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return []ReqeustLog{}, nil
		}
		return nil, err
	}
	return buildRequestLogList(records, ls.resolvePricingSnapshot()), nil
}

func normalizedProviderDisplayName(providerName string) string {
	trimmed := strings.TrimSpace(providerName)
	if trimmed == "" {
		return "(unknown)"
	}
	return trimmed
}

func providerStatMapKey(providerID string, providerName string) string {
	normalizedID := strings.TrimSpace(providerID)
	if normalizedID != "" {
		return "id:" + normalizedID
	}
	normalizedName := strings.ToLower(normalizedProviderDisplayName(providerName))
	return "name:" + normalizedName
}

func cloneProviderPerformanceMap(source map[string]providerPerformanceStat) map[string]providerPerformanceStat {
	if len(source) == 0 {
		return map[string]providerPerformanceStat{}
	}
	cloned := make(map[string]providerPerformanceStat, len(source))
	for key, stat := range source {
		cloned[key] = stat
	}
	return cloned
}

func defaultRangeEnd(start time.Time) time.Time {
	if start.Equal(startOfDay(start)) {
		return startOfDay(start).AddDate(0, 0, 1)
	}
	return start.Add(24 * time.Hour)
}

func resolveAggregationRange(startAt string, endAt string) (time.Time, time.Time, error) {
	start, err := parseTimeInput(startAt)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	end := time.Time{}
	if strings.TrimSpace(endAt) != "" {
		end, err = parseTimeInput(endAt)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	} else {
		end = defaultRangeEnd(start)
	}

	return start, end, nil
}

func resolveSummaryVisibleEnd(start time.Time, end time.Time, now time.Time) time.Time {
	if now.After(start) && now.Before(end) {
		return now
	}
	return end
}

func buildSummaryComparisonRange(start time.Time, end time.Time, visibleEnd time.Time) (time.Time, time.Time, bool) {
	if !start.Before(end) {
		return time.Time{}, time.Time{}, false
	}

	if !visibleEnd.After(start) {
		visibleEnd = end
	}
	if !visibleEnd.After(start) {
		return time.Time{}, time.Time{}, false
	}

	visibleDuration := visibleEnd.Sub(start)
	if visibleDuration <= 0 {
		return time.Time{}, time.Time{}, false
	}

	if end.Sub(start) <= 24*time.Hour {
		return start.Add(-24 * time.Hour), visibleEnd.Add(-24 * time.Hour), true
	}

	return start.Add(-visibleDuration), start, true
}

func calculateLogTotalTokens(logEntry ReqeustLog) int64 {
	return int64(logEntry.InputTokens) + int64(logEntry.OutputTokens) + int64(logEntry.CacheReadTokens)
}

func resolveStoredPricingModel(matchedPricingModel string, model string) string {
	if pricingModel := strings.TrimSpace(matchedPricingModel); pricingModel != "" {
		return pricingModel
	}
	return strings.TrimSpace(model)
}

func resolveLogPricingModel(logEntry ReqeustLog) string {
	if pricingModel := strings.TrimSpace(logEntry.EffectivePricingModel); pricingModel != "" {
		return pricingModel
	}
	if pricingModel := strings.TrimSpace(logEntry.MatchedPricingModel); pricingModel != "" {
		return pricingModel
	}
	if model := strings.TrimSpace(logEntry.Model); model != "" {
		return model
	}
	return "—"
}

func buildProviderPerformanceCacheKey(platformKey string, providerRef string, pricingModel string, startUTCKey string, endUTCKey string) string {
	return strings.Join([]string{
		platformKey,
		strings.TrimSpace(providerRef),
		strings.TrimSpace(pricingModel),
		startUTCKey,
		endUTCKey,
	}, "|")
}

func (ls *LogService) getProviderPerformanceCache(cacheKey string, now time.Time) (map[string]providerPerformanceStat, bool) {
	if ls == nil {
		return nil, false
	}

	ls.providerPerformanceCacheMu.Lock()
	defer ls.providerPerformanceCacheMu.Unlock()

	cached, ok := ls.providerPerformanceCache[cacheKey]
	if !ok {
		return nil, false
	}
	if !now.Before(cached.ExpiresAt) {
		delete(ls.providerPerformanceCache, cacheKey)
		return nil, false
	}
	return cloneProviderPerformanceMap(cached.Stats), true
}

func (ls *LogService) setProviderPerformanceCache(cacheKey string, stats map[string]providerPerformanceStat, now time.Time) {
	if ls == nil {
		return
	}

	ls.providerPerformanceCacheMu.Lock()
	defer ls.providerPerformanceCacheMu.Unlock()

	if ls.providerPerformanceCache == nil {
		ls.providerPerformanceCache = make(map[string]providerPerformanceCacheEntry)
	}
	reserveSlots := 0
	if _, exists := ls.providerPerformanceCache[cacheKey]; !exists {
		reserveSlots = 1
	}
	ls.compactProviderPerformanceCacheLocked(now, reserveSlots)
	ls.providerPerformanceCache[cacheKey] = providerPerformanceCacheEntry{
		ExpiresAt: now.Add(providerPerformanceCacheTTL),
		Stats:     cloneProviderPerformanceMap(stats),
	}
}

func (ls *LogService) compactProviderPerformanceCacheLocked(now time.Time, reserveSlots int) {
	if ls == nil || ls.providerPerformanceCache == nil {
		return
	}

	for key, entry := range ls.providerPerformanceCache {
		if !now.Before(entry.ExpiresAt) {
			delete(ls.providerPerformanceCache, key)
		}
	}

	maxEntries := providerPerformanceCacheMaxEntries - reserveSlots
	if maxEntries < 1 {
		maxEntries = 1
	}
	if len(ls.providerPerformanceCache) <= maxEntries {
		return
	}

	type cacheSnapshot struct {
		Key       string
		ExpiresAt time.Time
	}

	snapshots := make([]cacheSnapshot, 0, len(ls.providerPerformanceCache))
	for key, entry := range ls.providerPerformanceCache {
		snapshots = append(snapshots, cacheSnapshot{
			Key:       key,
			ExpiresAt: entry.ExpiresAt,
		})
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].ExpiresAt.Before(snapshots[j].ExpiresAt)
	})

	overflow := len(ls.providerPerformanceCache) - maxEntries
	if overflow <= 0 {
		return
	}
	for i := 0; i < overflow && i < len(snapshots); i++ {
		delete(ls.providerPerformanceCache, snapshots[i].Key)
	}
}

func (ls *LogService) ListRequestLogs(platform string, provider string, limit int) ([]ReqeustLog, error) {
	return ls.ListRequestLogsV2(platform, provider, limit, "", "")
}

func (ls *LogService) ListRequestLogsV2(platform string, provider string, limit int, startAt string, endAt string) ([]ReqeustLog, error) {
	limit = normalizeRequestLogListLimit(limit)
	model := xdb.New("request_log")
	filterOptions, err := buildRequestLogFilterOptions(platform, "", startAt, endAt)
	if err != nil {
		return nil, err
	}
	options := append([]xdb.Option{
		xdb.Field(requestLogListSelectFields...),
		xdb.OrderByDesc("created_at"),
		xdb.OrderByDesc("id"),
		xdb.Limit(limit),
	}, filterOptions...)
	records, err := selectRecordsByProviderRef(model, options, provider)
	if err != nil {
		return nil, err
	}
	pricingSnapshot := ls.resolvePricingSnapshot()
	return buildRequestLogList(records, pricingSnapshot), nil
}

func (ls *LogService) ListRequestLogsPageV2(platform string, provider string, pricingModel string, limit int, offset int, startAt string, endAt string) (RequestLogPageResult, error) {
	return ls.ListRequestLogsPageV3(platform, provider, pricingModel, string(LogDataSourceModeProxy), limit, offset, startAt, endAt)
}

func (ls *LogService) ListRequestLogsPageV3(platform string, provider string, pricingModel string, sourceMode string, limit int, offset int, startAt string, endAt string) (RequestLogPageResult, error) {
	result := RequestLogPageResult{
		Items:  []ReqeustLog{},
		Limit:  normalizeRequestLogListLimit(limit),
		Offset: normalizeRequestLogListOffset(offset),
	}

	model := xdb.New("request_log")
	filterOptions, err := buildRequestLogFilterOptionsV3(platform, pricingModel, startAt, endAt, normalizeLogDataSourceMode(sourceMode))
	if err != nil {
		return result, err
	}

	total, err := countRecordsByProviderRef(model, filterOptions, provider)
	if err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	result.Total = total
	if total == 0 {
		return result, nil
	}

	selectOptions := append([]xdb.Option{
		xdb.Field(requestLogListSelectFields...),
		xdb.OrderByDesc("created_at"),
		xdb.OrderByDesc("id"),
		xdb.Limit(result.Limit),
		xdb.Offset(result.Offset),
	}, filterOptions...)
	records, err := selectRecordsByProviderRef(model, selectOptions, provider)
	if err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	result.Items = buildRequestLogList(records, ls.resolvePricingSnapshot())
	return result, nil
}

func (ls *LogService) ListFailedRequestLogsPageV2(platform string, provider string, limit int, offset int, startAt string, endAt string) (RequestLogPageResult, error) {
	return ls.listFailedRequestLogsPageV2(platform, provider, limit, offset, startAt, endAt, false)
}

func (ls *LogService) ListUnreadFailedRequestLogsPageV2(platform string, provider string, limit int, offset int, startAt string, endAt string) (RequestLogPageResult, error) {
	return ls.listFailedRequestLogsPageV2(platform, provider, limit, offset, startAt, endAt, true)
}

func (ls *LogService) listFailedRequestLogsPageV2(platform string, provider string, limit int, offset int, startAt string, endAt string, unreadOnly bool) (RequestLogPageResult, error) {
	result := RequestLogPageResult{
		Items:  []ReqeustLog{},
		Limit:  normalizeRequestLogListLimit(limit),
		Offset: normalizeRequestLogListOffset(offset),
	}

	model := xdb.New("request_log")
	filterOptions, err := buildFailedRequestLogFilterOptions(platform, startAt, endAt)
	if err != nil {
		return result, err
	}
	if unreadOnly {
		filterOptions = append(filterOptions, xdb.WhereRaw(requestLogUnreadWhereClause))
	}

	total, err := countRecordsByProviderRef(model, filterOptions, provider)
	if err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	result.Total = total
	if total == 0 {
		return result, nil
	}

	selectOptions := append([]xdb.Option{
		xdb.Field(requestLogFailureListSelectFields...),
		xdb.OrderByDesc("created_at"),
		xdb.OrderByDesc("id"),
		xdb.Limit(result.Limit),
		xdb.Offset(result.Offset),
	}, filterOptions...)
	records, err := selectRecordsByProviderRef(model, selectOptions, provider)
	if err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	result.Items = buildRequestLogList(records, ls.resolvePricingSnapshot())
	return result, nil
}

func (ls *LogService) resolvePricingSnapshot() *modelpricing.Service {
	if ls != nil && ls.modelPricing != nil {
		if svc := ls.modelPricing.Service(); svc != nil {
			return svc
		}
	}
	svc, err := modelpricing.DefaultService()
	if err != nil {
		return nil
	}
	return svc
}

func normalizeRequestLogPriceSource(source string, totalCost float64) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case requestLogPriceSourceProviderAPI:
		return requestLogPriceSourceProviderAPI
	case requestLogPriceSourceBuiltin:
		return requestLogPriceSourceBuiltin
	case requestLogPriceSourceNone:
		if totalCost != 0 {
			return requestLogPriceSourceBuiltin
		}
		return requestLogPriceSourceNone
	default:
		if totalCost != 0 {
			return requestLogPriceSourceBuiltin
		}
		return requestLogPriceSourceNone
	}
}

func normalizeStoredPricingSource(logEntry *ReqeustLog) {
	if logEntry == nil || logEntry.PriceSource != requestLogPriceSourceNone {
		return
	}
	logEntry.PriceSource = requestLogPriceSourceBuiltin
}

func hasStoredBreakdownCost(logEntry *ReqeustLog) bool {
	if logEntry == nil {
		return false
	}
	return logEntry.InputCost != 0 ||
		logEntry.OutputCost != 0 ||
		logEntry.ReasoningCost != 0 ||
		logEntry.CacheCreateCost != 0 ||
		logEntry.CacheReadCost != 0 ||
		logEntry.Ephemeral5mCost != 0 ||
		logEntry.Ephemeral1hCost != 0
}

func storedBreakdownTotalCost(logEntry *ReqeustLog) float64 {
	if logEntry == nil {
		return 0
	}
	cacheCreateCost := logEntry.CacheCreateCost
	if cacheCreateCost == 0 {
		cacheCreateCost = logEntry.Ephemeral5mCost + logEntry.Ephemeral1hCost
	}
	return logEntry.InputCost +
		logEntry.OutputCost +
		logEntry.ReasoningCost +
		cacheCreateCost +
		logEntry.CacheReadCost
}

func applyLogPricing(pricing *modelpricing.Service, logEntry *ReqeustLog) {
	if logEntry == nil {
		return
	}

	logEntry.PriceSource = normalizeRequestLogPriceSource(logEntry.PriceSource, logEntry.TotalCost)
	if hasStoredBreakdownCost(logEntry) {
		if logEntry.TotalCost == 0 {
			logEntry.TotalCost = storedBreakdownTotalCost(logEntry)
		}
		logEntry.HasPricing = true
		normalizeStoredPricingSource(logEntry)
		return
	}
	if logEntry.TotalCost != 0 {
		logEntry.HasPricing = true
		normalizeStoredPricingSource(logEntry)
		return
	}
	if logEntry.HasPricing {
		normalizeStoredPricingSource(logEntry)
		return
	}

	pricingModelCandidates := buildRequestLogPricingModelCandidates(
		logEntry.ResponseModel,
		logEntry.MatchedPricingModel,
		logEntry.RequestedModel,
	)

	if logEntry.PriceSource == requestLogPriceSourceProviderAPI {
		if logEntry.ProviderPricingAvailable {
			logEntry.HasPricing = true
			return
		}
		if pricing == nil || len(pricingModelCandidates) == 0 {
			logEntry.HasPricing = true
			return
		}
	} else if pricing == nil || len(pricingModelCandidates) == 0 {
		return
	}

	usage := buildRequestLogUsageSnapshot(
		logEntry.InputTokens,
		logEntry.OutputTokens,
		logEntry.ReasoningTokens,
		logEntry.CacheCreateTokens,
		logEntry.Ephemeral5mTokens,
		logEntry.Ephemeral1hTokens,
		logEntry.CacheReadTokens,
	)

	var breakdown modelpricing.CostBreakdown
	resolvedPricingCandidate := ""
	for _, pricingModelCandidate := range pricingModelCandidates {
		current := pricing.CalculateCost(pricingModelCandidate, usage)
		if !current.HasPricing {
			continue
		}
		breakdown = current
		resolvedPricingCandidate = pricingModelCandidate
		break
	}
	if !breakdown.HasPricing {
		if logEntry.PriceSource == requestLogPriceSourceProviderAPI {
			logEntry.HasPricing = true
		}
		return
	}

	logEntry.InputCost = breakdown.InputCost
	logEntry.OutputCost = breakdown.OutputCost
	logEntry.ReasoningCost = breakdown.ReasoningCost
	logEntry.CacheCreateCost = breakdown.CacheCreateCost
	logEntry.CacheReadCost = breakdown.CacheReadCost
	logEntry.Ephemeral5mCost = breakdown.Ephemeral5mCost
	logEntry.Ephemeral1hCost = breakdown.Ephemeral1hCost
	logEntry.GroupMultiplier = breakdown.GroupMultiplier
	logEntry.HasPricing = true
	if logEntry.PriceSource == requestLogPriceSourceNone {
		logEntry.PriceSource = requestLogPriceSourceBuiltin
	}
	resolvedPricingModel := strings.TrimSpace(breakdown.PricingModel)
	if resolvedPricingModel == "" {
		resolvedPricingModel = resolvedPricingCandidate
	}
	if breakdown.TotalCost > 0 && logEntry.PriceSource != requestLogPriceSourceProviderAPI {
		logEntry.TotalCost = breakdown.TotalCost
		logEntry.PriceSource = requestLogPriceSourceBuiltin
	}
	if resolvedPricingModel != "" &&
		logEntry.PriceSource != requestLogPriceSourceProviderAPI &&
		!strings.EqualFold(strings.TrimSpace(logEntry.Model), resolvedPricingModel) {
		logEntry.MatchedPricingModel = resolvedPricingModel
	}
}

func (ls *LogService) ListProviders(platform string) ([]string, error) {
	refs, err := ls.ListProviderRefs(platform)
	if err != nil {
		return nil, err
	}
	nameSet := make(map[string]struct{})
	providers := make([]string, 0, len(refs))
	for _, ref := range refs {
		name := strings.TrimSpace(ref.Provider)
		if name == "" {
			continue
		}
		if _, exists := nameSet[name]; exists {
			continue
		}
		nameSet[name] = struct{}{}
		providers = append(providers, name)
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i] < providers[j]
	})
	return providers, nil
}

type LogProviderRef struct {
	ProviderID string `json:"provider_id,omitempty"`
	Provider   string `json:"provider"`
}

type logProviderRefCandidate struct {
	ProviderID string
	Provider   string
	LatestAt   string
}

func (ls *LogService) ListProviderRefs(platform string) ([]LogProviderRef, error) {
	return ls.ListProviderRefsV2(platform, string(LogDataSourceModeProxy))
}

func (ls *LogService) ListProviderRefsV2(platform string, sourceMode string) ([]LogProviderRef, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	query := `
		SELECT provider_id, provider, MAX(created_at) AS latest_at
		FROM request_log
		WHERE TRIM(COALESCE(provider, '')) <> ''
	`
	args := make([]interface{}, 0, 1)
	query += " AND " + requestLogSourceWhereClause(normalizeLogDataSourceMode(sourceMode), "request_log")
	if strings.TrimSpace(platform) != "" {
		query += " AND platform = ?"
		args = append(args, platform)
	}
	query += " GROUP BY provider_id, provider"

	rows, err := db.Query(query, args...)
	if err != nil {
		if isNoSuchTableErr(err) {
			return []LogProviderRef{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	candidates := make([]logProviderRefCandidate, 0, 64)
	for rows.Next() {
		var providerID sql.NullString
		var providerName sql.NullString
		var latestAt sql.NullString
		if err := rows.Scan(&providerID, &providerName, &latestAt); err != nil {
			return nil, err
		}
		candidates = append(candidates, logProviderRefCandidate{
			ProviderID: strings.TrimSpace(providerID.String),
			Provider:   strings.TrimSpace(providerName.String),
			LatestAt:   strings.TrimSpace(latestAt.String),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return mergeProviderRefsFromCandidates(candidates), nil
}

func mergeProviderRefsFromCandidates(candidates []logProviderRefCandidate) []LogProviderRef {
	nameToIDs := make(map[string]map[string]struct{})
	for _, candidate := range candidates {
		if candidate.ProviderID == "" || candidate.Provider == "" {
			continue
		}
		nameKey := strings.ToLower(candidate.Provider)
		idSet := nameToIDs[nameKey]
		if idSet == nil {
			idSet = make(map[string]struct{})
			nameToIDs[nameKey] = idSet
		}
		idSet[candidate.ProviderID] = struct{}{}
	}

	type refSnapshot struct {
		Ref      LogProviderRef
		LatestAt string
	}
	providerByRef := make(map[string]refSnapshot)
	for _, candidate := range candidates {
		providerID := candidate.ProviderID
		name := candidate.Provider
		latestAt := candidate.LatestAt
		if providerID == "" && name == "" {
			continue
		}

		refKey := providerID
		if refKey == "" {
			nameKey := strings.ToLower(name)
			if idSet := nameToIDs[nameKey]; len(idSet) == 1 {
				for onlyID := range idSet {
					refKey = onlyID
				}
				providerID = refKey
			} else {
				refKey = "name:" + nameKey
			}
		}

		current := providerByRef[refKey]
		if current.Ref.ProviderID == "" && providerID != "" {
			current.Ref.ProviderID = providerID
		}
		if shouldReplaceProviderDisplayName(current.LatestAt, latestAt, current.Ref.Provider, name) {
			current.Ref.Provider = name
			current.LatestAt = latestAt
		}
		if current.LatestAt == "" && latestAt != "" {
			current.LatestAt = latestAt
		}
		providerByRef[refKey] = current
	}

	refs := make([]LogProviderRef, 0, len(providerByRef))
	for _, snapshot := range providerByRef {
		ref := snapshot.Ref
		if strings.TrimSpace(ref.Provider) == "" {
			ref.Provider = strings.TrimSpace(ref.ProviderID)
		}
		if strings.TrimSpace(ref.Provider) == "" {
			continue
		}
		refs = append(refs, ref)
	}

	sort.Slice(refs, func(i, j int) bool {
		leftName := strings.TrimSpace(refs[i].Provider)
		rightName := strings.TrimSpace(refs[j].Provider)
		if leftName == rightName {
			return strings.TrimSpace(refs[i].ProviderID) < strings.TrimSpace(refs[j].ProviderID)
		}
		return leftName < rightName
	})
	return refs
}

func shouldReplaceProviderDisplayName(currentLatestAt, candidateLatestAt, currentName, candidateName string) bool {
	candidateName = strings.TrimSpace(candidateName)
	if candidateName == "" {
		return false
	}
	currentName = strings.TrimSpace(currentName)
	if currentName == "" {
		return true
	}
	currentLatestAt = strings.TrimSpace(currentLatestAt)
	candidateLatestAt = strings.TrimSpace(candidateLatestAt)
	if candidateLatestAt != "" && (currentLatestAt == "" || candidateLatestAt > currentLatestAt) {
		return true
	}
	if candidateLatestAt == currentLatestAt && len(candidateName) > len(currentName) {
		return true
	}
	return false
}

func (ls *LogService) HeatmapStats(days int) ([]HeatmapStat, error) {
	if days <= 0 {
		days = 30
	}
	totalHours := days * 24
	if totalHours <= 0 {
		totalHours = 24
	}
	rangeEnd := startOfHour(time.Now())
	rangeStart := rangeEnd
	if totalHours > 1 {
		rangeStart = rangeEnd.Add(-time.Duration(totalHours-1) * time.Hour)
	}

	stats, err := ls.heatmapStatsFromHourlyTable(rangeStart, rangeEnd, totalHours)
	if err == nil {
		if len(stats) > 0 {
			return stats, nil
		}
		return ls.heatmapStatsFromRequestLog(rangeStart, totalHours)
	}
	if !isNoSuchTableErr(err) {
		return nil, err
	}
	return ls.heatmapStatsFromRequestLog(rangeStart, totalHours)
}

func (ls *LogService) heatmapStatsFromHourlyTable(
	rangeStart time.Time,
	rangeEnd time.Time,
	totalHours int,
) ([]HeatmapStat, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
		SELECT
			bucket_start,
			COALESCE(SUM(total_requests), 0) AS total_requests,
			COALESCE(SUM(input_tokens), 0) AS input_tokens,
			COALESCE(SUM(output_tokens), 0) AS output_tokens,
			COALESCE(SUM(cache_read_tokens), 0) AS cache_read_tokens,
			COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
			COALESCE(SUM(total_cost), 0) AS total_cost
		FROM %s
		WHERE bucket_start >= ? AND bucket_start <= ?
		GROUP BY bucket_start
		ORDER BY bucket_start DESC
		LIMIT ?
	`, requestLogStatsHourlyTable)
	rows, err := db.Query(
		query,
		rangeStart.Format(timeLayout),
		rangeEnd.Format(timeLayout),
		totalHours,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make([]HeatmapStat, 0, totalHours)
	for rows.Next() {
		var bucketStart string
		stat := HeatmapStat{}
		if err := rows.Scan(
			&bucketStart,
			&stat.TotalRequests,
			&stat.InputTokens,
			&stat.OutputTokens,
			&stat.CacheReadTokens,
			&stat.ReasoningTokens,
			&stat.TotalCost,
		); err != nil {
			return nil, err
		}
		populateHeatmapTotalTokens(&stat)
		bucketStart = strings.TrimSpace(bucketStart)
		if bucketStart == "" {
			continue
		}
		if parsed, parseErr := time.ParseInLocation(timeLayout, bucketStart, time.Local); parseErr == nil {
			stat.Day = parsed.Format("2006-01-02 15")
		} else if len(bucketStart) >= len("2006-01-02 15") {
			stat.Day = bucketStart[:len("2006-01-02 15")]
		} else {
			stat.Day = bucketStart
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func (ls *LogService) heatmapStatsFromRequestLog(
	rangeStart time.Time,
	totalHours int,
) ([]HeatmapStat, error) {
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.WhereGe("created_at", rangeStart.UTC().Format(timeLayout)),
		xdb.Field(
			"input_tokens",
			"output_tokens",
			"cache_read_tokens",
			"reasoning_tokens",
			"total_cost",
			"created_at",
		),
		xdb.OrderByDesc("created_at"),
	}
	records, err := model.Selects(options...)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return []HeatmapStat{}, nil
		}
		return nil, err
	}
	hourBuckets := map[int64]*HeatmapStat{}
	for _, record := range records {
		createdAt, _ := parseCreatedAt(record)
		if createdAt.IsZero() {
			continue
		}
		hourStart := startOfHour(createdAt)
		hourKey := hourStart.Unix()
		bucket := hourBuckets[hourKey]
		if bucket == nil {
			bucket = &HeatmapStat{Day: hourStart.Format("2006-01-02 15")}
			hourBuckets[hourKey] = bucket
		}
		bucket.TotalRequests++
		input := record.GetInt("input_tokens")
		output := record.GetInt("output_tokens")
		cacheRead := record.GetInt("cache_read_tokens")
		reasoning := record.GetInt("reasoning_tokens")
		bucket.InputTokens += int64(input)
		bucket.OutputTokens += int64(output)
		bucket.CacheReadTokens += int64(cacheRead)
		bucket.ReasoningTokens += int64(reasoning)
		populateHeatmapTotalTokens(bucket)
		bucket.TotalCost += record.GetFloat64("total_cost")
	}
	if len(hourBuckets) == 0 {
		return []HeatmapStat{}, nil
	}
	hourKeys := make([]int64, 0, len(hourBuckets))
	for key := range hourBuckets {
		hourKeys = append(hourKeys, key)
	}
	sort.Slice(hourKeys, func(i, j int) bool {
		return hourKeys[i] < hourKeys[j]
	})
	stats := make([]HeatmapStat, 0, min(len(hourKeys), totalHours))
	for i := len(hourKeys) - 1; i >= 0 && len(stats) < totalHours; i-- {
		stats = append(stats, *hourBuckets[hourKeys[i]])
	}
	return stats, nil
}

func (ls *LogService) StatsSince(platform string) (LogStats, error) {
	seriesStart := startOfDay(time.Now())
	seriesEnd := seriesStart.AddDate(0, 0, 1)
	return ls.StatsRangeV2(platform, "", "", seriesStart.Format(timeLayout), seriesEnd.Format(timeLayout))
}

func (ls *LogService) StatsRangeV2(platform string, provider string, pricingModel string, startAt string, endAt string) (LogStats, error) {
	return ls.StatsRangeV3(platform, provider, pricingModel, string(LogDataSourceModeProxy), startAt, endAt)
}

func (ls *LogService) StatsRangeV3(platform string, provider string, pricingModel string, sourceMode string, startAt string, endAt string) (LogStats, error) {
	stats := LogStats{
		Series: make([]LogStatsSeries, 0),
	}

	start, end, err := resolveAggregationRange(startAt, endAt)
	if err != nil {
		return stats, err
	}

	if !start.Before(end) {
		return stats, nil
	}

	duration := end.Sub(start)
	useDayBuckets := duration > 48*time.Hour
	bucketSize := time.Hour

	bucketStarts := make([]time.Time, 0)
	if useDayBuckets {
		bucketSize = 24 * time.Hour
		for t := startOfDay(start); t.Before(end); t = t.AddDate(0, 0, 1) {
			bucketStarts = append(bucketStarts, t)
		}
		if len(bucketStarts) == 0 {
			dayStart := startOfDay(start)
			bucketStarts = append(bucketStarts, dayStart)
		}
	} else {
		bucketCount := int(duration / bucketSize)
		if duration%bucketSize != 0 {
			bucketCount++
		}
		if bucketCount <= 0 {
			bucketCount = 1
		}
		bucketStarts = make([]time.Time, 0, bucketCount)
		for i := 0; i < bucketCount; i++ {
			bucketStarts = append(bucketStarts, start.Add(time.Duration(i)*bucketSize))
		}
	}

	bucketCount := len(bucketStarts)
	seriesBuckets := make([]*LogStatsSeries, bucketCount)
	for i, bucketTime := range bucketStarts {
		seriesBuckets[i] = &LogStatsSeries{
			Day: bucketTime.Format(timeLayout),
		}
	}

	logs, err := ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, start, end, normalizeLogDataSourceMode(sourceMode))
	if err != nil {
		return stats, err
	}

	for _, logEntry := range logs {
		createdAt, ok := parseRequestLogLocalTime(logEntry.CreatedAt)
		if !ok || createdAt.Before(start) || !createdAt.Before(end) {
			continue
		}

		bucketIndex := -1
		if useDayBuckets {
			bucketIndex = int(startOfDay(createdAt).Sub(startOfDay(start)) / (24 * time.Hour))
		} else {
			bucketIndex = int(createdAt.Sub(start) / bucketSize)
		}
		if bucketIndex < 0 || bucketIndex >= bucketCount {
			continue
		}

		bucket := seriesBuckets[bucketIndex]
		if bucket == nil {
			continue
		}

		total := int64(1)
		input := int64(logEntry.InputTokens)
		output := int64(logEntry.OutputTokens)
		reasoning := int64(logEntry.ReasoningTokens)
		cacheCreate := int64(logEntry.CacheCreateTokens)
		cacheRead := int64(logEntry.CacheReadTokens)
		costTotal := logEntry.TotalCost

		bucket.TotalRequests += total
		bucket.InputTokens += input
		bucket.OutputTokens += output
		bucket.ReasoningTokens += reasoning
		bucket.CacheCreateTokens += cacheCreate
		bucket.CacheReadTokens += cacheRead
		bucket.TotalCost += costTotal

		stats.TotalRequests += total
		stats.InputTokens += input
		stats.OutputTokens += output
		stats.ReasoningTokens += reasoning
		stats.CacheCreateTokens += cacheCreate
		stats.CacheReadTokens += cacheRead
		stats.CostTotal += costTotal
		stats.CostInput += logEntry.InputCost
		stats.CostOutput += logEntry.OutputCost + logEntry.ReasoningCost
		stats.CostCacheCreate += logEntry.CacheCreateCost
		stats.CostCacheRead += logEntry.CacheReadCost
	}

	stats.Series = make([]LogStatsSeries, 0, bucketCount)
	for i := 0; i < bucketCount; i++ {
		if bucket := seriesBuckets[i]; bucket != nil {
			stats.Series = append(stats.Series, *bucket)
		} else {
			bucketTime := bucketStarts[i]
			stats.Series = append(stats.Series, LogStatsSeries{
				Day: bucketTime.Format(timeLayout),
			})
		}
	}

	return stats, nil
}

// ProviderPerformanceTrend15m 返回指定时间范围内的供应商 15 分钟性能均值。
func (ls *LogService) ProviderPerformanceTrend15m(platform string, provider string, startAt string, endAt string) ([]ProviderPerformanceTrendPoint, error) {
	start, end, err := resolveAggregationRange(startAt, endAt)
	if err != nil {
		return nil, err
	}
	if !start.Before(end) {
		return []ProviderPerformanceTrendPoint{}, nil
	}

	bucketCount := int(end.Sub(start) / providerPerformanceTrendBucketSize)
	if end.Sub(start)%providerPerformanceTrendBucketSize != 0 {
		bucketCount++
	}
	if bucketCount <= 0 {
		return []ProviderPerformanceTrendPoint{}, nil
	}
	if bucketCount > providerPerformanceTrendMaxBucketCount {
		return nil, fmt.Errorf("provider performance trend range exceeds %d buckets", providerPerformanceTrendMaxBucketCount)
	}

	points := make([]ProviderPerformanceTrendPoint, bucketCount)
	for index := range points {
		points[index].BucketStart = start.Add(time.Duration(index) * providerPerformanceTrendBucketSize).Format(timeLayout)
	}

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	startUTCKey := start.UTC().Format(timeLayout)
	endUTCKey := end.UTC().Format(timeLayout)
	platformKey := strings.TrimSpace(platform)
	providerRef := strings.TrimSpace(provider)
	providerColumn := ""
	if providerRef != "" {
		existsQuery := `SELECT 1 FROM request_log WHERE created_at >= ? AND created_at < ? AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
		existsArgs := []interface{}{startUTCKey, endUTCKey}
		if platformKey != "" {
			existsQuery += " AND platform = ?"
			existsArgs = append(existsArgs, platformKey)
		}
		existsQuery += " AND provider_id = ? LIMIT 1"
		existsArgs = append(existsArgs, providerRef)

		var exists int
		switch existsErr := db.QueryRow(existsQuery, existsArgs...).Scan(&exists); {
		case existsErr == nil:
			providerColumn = "provider_id"
		case errors.Is(existsErr, sql.ErrNoRows):
			providerColumn = "provider"
		case isNoSuchTableErr(existsErr) || strings.Contains(existsErr.Error(), "no such column"):
			return points, nil
		default:
			return nil, existsErr
		}
	}

	bucketSeconds := int64(providerPerformanceTrendBucketSize / time.Second)
	query := `
		SELECT
			CAST((CAST(strftime('%s', created_at) AS INTEGER) - CAST(strftime('%s', ?) AS INTEGER)) / ? AS INTEGER) AS bucket_index,
			COALESCE(AVG(CASE WHEN first_token_sec > 0 THEN first_token_sec END), 0) AS avg_first_token_sec,
			COALESCE(AVG(CASE WHEN output_tokens > 0 AND duration_sec > 0 THEN CAST(output_tokens AS REAL) / duration_sec END), 0) AS avg_tokens_per_sec,
			COALESCE(SUM(CASE WHEN first_token_sec > 0 THEN 1 ELSE 0 END), 0) AS ttft_sample_count,
			COALESCE(SUM(CASE WHEN output_tokens > 0 AND duration_sec > 0 THEN 1 ELSE 0 END), 0) AS tps_sample_count
		FROM request_log
		WHERE created_at >= ? AND created_at < ?
			AND ` + requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log") + `
			AND COALESCE(is_stream, 0) = 1
			AND ` + requestLogSuccessWhereClause("", 0)
	queryArgs := []interface{}{startUTCKey, bucketSeconds, startUTCKey, endUTCKey}
	if platformKey != "" {
		query += " AND platform = ?"
		queryArgs = append(queryArgs, platformKey)
	}
	if providerColumn != "" {
		query += " AND " + providerColumn + " = ?"
		queryArgs = append(queryArgs, providerRef)
	}
	query += " GROUP BY bucket_index ORDER BY bucket_index"

	rows, err := db.Query(query, queryArgs...)
	if err != nil {
		if isNoSuchTableErr(err) || strings.Contains(err.Error(), "no such column") {
			return points, nil
		}
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var bucketIndex int
		point := ProviderPerformanceTrendPoint{}
		if err := rows.Scan(
			&bucketIndex,
			&point.AvgFirstTokenSec,
			&point.AvgTokensPerSec,
			&point.TTFTSampleCount,
			&point.TPSSampleCount,
		); err != nil {
			return nil, err
		}
		if bucketIndex < 0 || bucketIndex >= bucketCount {
			continue
		}
		point.BucketStart = points[bucketIndex].BucketStart
		points[bucketIndex] = point
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return points, nil
}

func (ls *LogService) SummaryRangeV2(platform string, provider string, pricingModel string, startAt string, endAt string) (LogSummary, error) {
	return ls.SummaryRangeV3(platform, provider, pricingModel, string(LogDataSourceModeProxy), startAt, endAt)
}

func (ls *LogService) SummaryRangeV3(platform string, provider string, pricingModel string, sourceMode string, startAt string, endAt string) (LogSummary, error) {
	const activityBucketCount = 12

	summary := LogSummary{
		ActivityPoints: make([]float64, activityBucketCount),
	}

	start, end, err := resolveAggregationRange(startAt, endAt)
	if err != nil {
		return summary, err
	}
	if !start.Before(end) {
		return summary, nil
	}

	normalizedSourceMode := normalizeLogDataSourceMode(sourceMode)
	logs, err := ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, start, end, normalizedSourceMode)
	if err != nil {
		return summary, err
	}

	for _, logEntry := range logs {
		totalTokens := calculateLogTotalTokens(logEntry)

		summary.TotalRequests++
		summary.InputTokens += int64(logEntry.InputTokens)
		summary.OutputTokens += int64(logEntry.OutputTokens)
		summary.CacheReadTokens += int64(logEntry.CacheReadTokens)
		summary.TotalTokens += totalTokens
		summary.CostTotal += logEntry.TotalCost
		summary.CostInput += logEntry.InputCost
		summary.CostCacheRead += logEntry.CacheReadCost

		switch resolvedRequestLogOutcome(logEntry) {
		case requestOutcomeFailure:
			summary.FailedRequests++
		case requestOutcomeExcluded:
			summary.ExcludedRequests++
		default:
			summary.SuccessfulRequests++
		}

		if totalTokens > summary.PeakTokens {
			summary.PeakTokens = totalTokens
		}
	}

	evaluatedRequests := summary.SuccessfulRequests + summary.FailedRequests
	if evaluatedRequests > 0 {
		summary.SuccessRate = float64(summary.SuccessfulRequests) / float64(evaluatedRequests)
	}
	if summary.TotalRequests > 0 {
		summary.AvgTokensPerRequest = float64(summary.TotalTokens) / float64(summary.TotalRequests)
	}

	if summary.InputTokens > 0 && summary.CacheReadTokens > 0 {
		effectiveInputRate := summary.CostInput / float64(summary.InputTokens)
		estimatedUncachedCost := float64(summary.CacheReadTokens) * effectiveInputRate
		if estimatedUncachedCost > summary.CostCacheRead {
			summary.SavedCostEstimate = estimatedUncachedCost - summary.CostCacheRead
		}
	}

	now := time.Now().In(time.Local)
	visibleEnd := resolveSummaryVisibleEnd(start, end, now)
	if !visibleEnd.After(start) {
		visibleEnd = end
	}
	if visibleEnd.After(start) {
		summary.ProjectedDailyCost = summary.CostTotal / visibleEnd.Sub(start).Hours() * 24
		if math.IsNaN(summary.ProjectedDailyCost) || math.IsInf(summary.ProjectedDailyCost, 0) {
			summary.ProjectedDailyCost = 0
		}
	}

	if compareStart, compareEnd, ok := buildSummaryComparisonRange(start, end, visibleEnd); ok && compareStart.Before(compareEnd) {
		previousLogs, compareErr := ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, compareStart, compareEnd, normalizedSourceMode)
		if compareErr != nil {
			return summary, compareErr
		}
		summary.ComparisonAvailable = true
		for _, logEntry := range previousLogs {
			summary.PreviousCostTotal += logEntry.TotalCost
		}
	}

	activityEnd := visibleEnd
	if !activityEnd.After(start) {
		activityEnd = end
	}
	activityStart := activityEnd.Add(-time.Minute)
	if activityStart.Before(start) {
		activityStart = start
	}

	activityDuration := activityEnd.Sub(activityStart)
	if activityDuration > 0 {
		bucketCounts := make([]int64, activityBucketCount)
		activityRequests := int64(0)
		peakBucketCount := int64(0)
		activityDurationSeconds := activityDuration.Seconds()
		bucketDurationSeconds := activityDurationSeconds / float64(activityBucketCount)
		if bucketDurationSeconds <= 0 {
			bucketDurationSeconds = 1
		}

		for _, logEntry := range logs {
			createdAt, ok := parseRequestLogLocalTime(logEntry.CreatedAt)
			if !ok || createdAt.Before(activityStart) || !createdAt.Before(activityEnd) {
				continue
			}

			offset := createdAt.Sub(activityStart)
			bucketIndex := int((float64(offset) / float64(activityDuration)) * float64(activityBucketCount))
			if bucketIndex < 0 {
				bucketIndex = 0
			}
			if bucketIndex >= activityBucketCount {
				bucketIndex = activityBucketCount - 1
			}

			bucketCounts[bucketIndex]++
			activityRequests++
			if bucketCounts[bucketIndex] > peakBucketCount {
				peakBucketCount = bucketCounts[bucketIndex]
			}
		}

		for i, count := range bucketCounts {
			summary.ActivityPoints[i] = float64(count) / bucketDurationSeconds
		}
		if activityDurationSeconds > 0 {
			summary.ActivityAvgQPS = float64(activityRequests) / activityDurationSeconds
		}
		summary.ActivityPeakQPS = float64(peakBucketCount) / bucketDurationSeconds
	}

	return summary, nil
}

func (ls *LogService) ProviderDailyStats(platform string) ([]ProviderDailyStat, error) {
	start := startOfDay(time.Now())
	end := start.AddDate(0, 0, 1)
	stats, err := ls.ProviderStatsRangeV2(platform, "", "", start.Format(timeLayout), end.Format(timeLayout))
	if err != nil {
		return nil, err
	}
	for i := range stats {
		stats[i].AvgFirstTokenSec = 0
		stats[i].AvgTokensPerSec = 0
		stats[i].TTFTSampleCount = 0
		stats[i].TPSSampleCount = 0
	}

	latestPerformance, err := ls.latestProviderPerformance(platform)
	if err != nil {
		return nil, err
	}

	statsByProvider := make(map[string]int, len(stats)+len(latestPerformance))
	for i := range stats {
		statsByProvider[providerStatMapKey(stats[i].ProviderID, stats[i].Provider)] = i
	}
	for statKey, performance := range latestPerformance {
		statIndex, exists := statsByProvider[statKey]
		if !exists {
			stats = append(stats, ProviderDailyStat{
				ProviderID: performance.ProviderID,
				Provider:   normalizedProviderDisplayName(performance.Provider),
			})
			statIndex = len(stats) - 1
			statsByProvider[statKey] = statIndex
		}
		stat := &stats[statIndex]
		stat.AvgFirstTokenSec = performance.AvgFirstTokenSec
		stat.AvgTokensPerSec = performance.AvgTokensPerSec
		stat.TTFTSampleCount = performance.TTFTSampleCount
		stat.TPSSampleCount = performance.TPSSampleCount
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].TotalRequests == stats[j].TotalRequests {
			return stats[i].Provider < stats[j].Provider
		}
		return stats[i].TotalRequests > stats[j].TotalRequests
	})
	return stats, nil
}

func (ls *LogService) latestProviderPerformance(platform string) (map[string]providerPerformanceStat, error) {
	platformKey := strings.TrimSpace(platform)
	cacheKey := "latest-provider-performance|" + platformKey
	now := time.Now()
	if cached, ok := ls.getProviderPerformanceCache(cacheKey, now); ok {
		return cached, nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	query := `
		WITH valid_samples AS (
			SELECT
				id,
				created_at,
				TRIM(COALESCE(provider_id, '')) AS provider_id,
				CASE
					WHEN TRIM(COALESCE(provider, '')) = '' THEN '(unknown)'
					ELSE TRIM(provider)
				END AS provider_name,
				CASE
					WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN 'id:' || TRIM(provider_id)
					ELSE 'name:' || LOWER(CASE
						WHEN TRIM(COALESCE(provider, '')) = '' THEN '(unknown)'
						ELSE TRIM(provider)
					END)
				END AS provider_stat_key,
				first_token_sec,
				CAST(output_tokens AS REAL) / duration_sec AS tokens_per_sec
			FROM request_log
			WHERE COALESCE(is_stream, 0) = 1
				AND ` + requestLogSuccessWhereClause("", 200) + `
				AND first_token_sec > 0
				AND output_tokens > 0
				AND duration_sec > 0
				AND (TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> '')
	`
	args := make([]interface{}, 0, 2)
	if platformKey != "" {
		query += " AND platform = ?"
		args = append(args, platformKey)
	}
	query += `
		), ranked_samples AS (
			SELECT
				*,
				ROW_NUMBER() OVER (
					PARTITION BY provider_stat_key
					ORDER BY created_at DESC, id DESC
				) AS sample_rank
			FROM valid_samples
		)
		SELECT
			provider_stat_key,
			MAX(CASE WHEN sample_rank = 1 THEN provider_id ELSE '' END) AS provider_id,
			MAX(CASE WHEN sample_rank = 1 THEN provider_name ELSE '' END) AS provider_name,
			AVG(first_token_sec) AS avg_first_token_sec,
			AVG(tokens_per_sec) AS avg_tokens_per_sec,
			COUNT(*) AS sample_count
		FROM ranked_samples
		WHERE sample_rank <= ?
		GROUP BY provider_stat_key
	`
	args = append(args, providerLatestPerformanceSampleLimit)

	rows, err := db.Query(query, args...)
	if err != nil {
		if isNoSuchTableErr(err) || strings.Contains(err.Error(), "no such column") {
			return map[string]providerPerformanceStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	statsByProvider := make(map[string]providerPerformanceStat, 16)
	for rows.Next() {
		var statKey sql.NullString
		var providerID sql.NullString
		var providerName sql.NullString
		var avgFirstTokenSec sql.NullFloat64
		var avgTokensPerSec sql.NullFloat64
		var sampleCount sql.NullInt64
		if err := rows.Scan(
			&statKey,
			&providerID,
			&providerName,
			&avgFirstTokenSec,
			&avgTokensPerSec,
			&sampleCount,
		); err != nil {
			return nil, err
		}

		normalizedKey := strings.TrimSpace(statKey.String)
		if normalizedKey == "" {
			continue
		}
		statsByProvider[normalizedKey] = providerPerformanceStat{
			ProviderID:       strings.TrimSpace(providerID.String),
			Provider:         normalizedProviderDisplayName(providerName.String),
			AvgFirstTokenSec: avgFirstTokenSec.Float64,
			AvgTokensPerSec:  avgTokensPerSec.Float64,
			TTFTSampleCount:  sampleCount.Int64,
			TPSSampleCount:   sampleCount.Int64,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	ls.setProviderPerformanceCache(cacheKey, statsByProvider, now)
	return statsByProvider, nil
}

func (ls *LogService) CountProviderUnreadFailedRequestLogs(platform string, providerID string, provider string) (ProviderUnreadFailedCountResult, error) {
	result := ProviderUnreadFailedCountResult{}
	target, err := normalizeProviderLogStorageTarget(platform, providerID, provider)
	if err != nil {
		return result, err
	}

	whereClause, args := buildProviderUnreadFailedRequestLogWhereClause(target)
	db, err := xdb.DB("default")
	if err != nil {
		return result, err
	}

	var count int64
	if err := db.QueryRow("SELECT COUNT(*) FROM request_log WHERE "+whereClause, args...).Scan(&count); err != nil {
		if isNoSuchTableErr(err) {
			return result, nil
		}
		return result, err
	}
	result.UnreadFailedRequests = count
	return result, nil
}

func (ls *LogService) ProviderUnreadFailedStats(platform string) ([]ProviderUnreadFailedStat, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	providerIdentityExpr := `CASE
		WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
		WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
		ELSE ''
	END`
	providerNameExpr := `CASE
		WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider)
		WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id)
		ELSE '(unknown)'
	END`
	query := `
		SELECT
			` + providerIdentityExpr + ` AS provider_id,
			MAX(` + providerNameExpr + `) AS provider,
			COUNT(*) AS unread_failed_requests
		FROM request_log
		WHERE ` + requestLogFailureWhereClause("") + `
			AND ` + requestLogUnreadWhereClause + `
			AND (TRIM(COALESCE(provider_id, '')) <> '' OR TRIM(COALESCE(provider, '')) <> '')
	`
	args := make([]any, 0, 1)
	if strings.TrimSpace(platform) != "" {
		query += " AND platform = ?"
		args = append(args, strings.TrimSpace(platform))
	}
	query += " GROUP BY " + providerIdentityExpr

	rows, err := db.Query(query, args...)
	if err != nil {
		if isNoSuchTableErr(err) || strings.Contains(err.Error(), "no such column") {
			return []ProviderUnreadFailedStat{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	stats := make([]ProviderUnreadFailedStat, 0, 16)
	for rows.Next() {
		var providerID sql.NullString
		var providerName sql.NullString
		var unreadFailedRequests sql.NullInt64
		if err := rows.Scan(&providerID, &providerName, &unreadFailedRequests); err != nil {
			return nil, err
		}

		normalizedProviderID := strings.TrimSpace(providerID.String)
		if normalizedProviderID == "" {
			continue
		}
		normalizedProvider := normalizedProviderDisplayName(providerName.String)
		stats = append(stats, ProviderUnreadFailedStat{
			ProviderID:           normalizedProviderID,
			Provider:             normalizedProvider,
			UnreadFailedRequests: unreadFailedRequests.Int64,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].UnreadFailedRequests == stats[j].UnreadFailedRequests {
			return stats[i].Provider < stats[j].Provider
		}
		return stats[i].UnreadFailedRequests > stats[j].UnreadFailedRequests
	})

	return stats, nil
}

func (ls *LogService) ProviderStatsRangeV2(platform string, provider string, pricingModel string, startAt string, endAt string) ([]ProviderDailyStat, error) {
	return ls.ProviderStatsRangeV3(platform, provider, pricingModel, string(LogDataSourceModeProxy), startAt, endAt)
}

func (ls *LogService) ProviderStatsRangeV3(platform string, provider string, pricingModel string, sourceMode string, startAt string, endAt string) ([]ProviderDailyStat, error) {
	start, end, err := resolveAggregationRange(startAt, endAt)
	if err != nil {
		return nil, err
	}

	if !start.Before(end) {
		return []ProviderDailyStat{}, nil
	}

	duration := end.Sub(start)
	startUTCKey := start.UTC().Format(timeLayout)
	endUTCKey := end.UTC().Format(timeLayout)
	platformKey := strings.TrimSpace(platform)
	normalizedSourceMode := normalizeLogDataSourceMode(sourceMode)
	logs, err := ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, start, end, normalizedSourceMode)
	if err != nil {
		return nil, err
	}

	statMap := map[string]*ProviderDailyStat{}
	durationSums := map[string]float64{}
	for _, logEntry := range logs {
		providerID := strings.TrimSpace(logEntry.ProviderID)
		providerName := normalizedProviderDisplayName(logEntry.Provider)
		statKey := providerStatMapKey(providerID, providerName)

		stat := statMap[statKey]
		if stat == nil {
			stat = &ProviderDailyStat{
				ProviderID: providerID,
				Provider:   providerName,
			}
			statMap[statKey] = stat
		}
		if stat.ProviderID == "" {
			stat.ProviderID = providerID
		}
		if stat.Provider == "" || stat.Provider == "(unknown)" {
			stat.Provider = providerName
		}

		stat.TotalRequests++
		switch resolvedRequestLogOutcome(logEntry) {
		case requestOutcomeFailure:
			stat.FailedRequests++
			if strings.TrimSpace(logEntry.ErrorReadAt) == "" {
				stat.UnreadFailedRequests++
			}
		case requestOutcomeExcluded:
			stat.ExcludedRequests++
		default:
			stat.SuccessfulRequests++
		}
		stat.InputTokens += int64(logEntry.InputTokens)
		stat.OutputTokens += int64(logEntry.OutputTokens)
		stat.ReasoningTokens += int64(logEntry.ReasoningTokens)
		stat.CacheCreateTokens += int64(logEntry.CacheCreateTokens)
		stat.CacheReadTokens += int64(logEntry.CacheReadTokens)
		stat.CostTotal += logEntry.TotalCost
		if logEntry.DurationSec > 0 {
			durationSums[statKey] += logEntry.DurationSec
			stat.DurationSampleCount++
		}
	}

	if duration <= 48*time.Hour {
		providerRef := strings.TrimSpace(provider)
		cacheKey := buildProviderPerformanceCacheKey(platformKey, providerRef, pricingModel+"|source:"+string(normalizedSourceMode), startUTCKey, endUTCKey)
		now := time.Now()
		performanceMap, cached := ls.getProviderPerformanceCache(cacheKey, now)
		if !cached {
			db, err := xdb.DB("default")
			if err != nil {
				return nil, err
			}

			queryProviderPerformance := func(providerColumn string, providerValue string) (map[string]providerPerformanceStat, error) {
				query := `
						SELECT
						CASE
							WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN 'id:' || TRIM(provider_id)
							ELSE 'name:' || LOWER(CASE
								WHEN TRIM(COALESCE(provider, '')) = '' THEN '(unknown)'
								ELSE TRIM(provider)
							END)
						END AS provider_stat_key,
							COALESCE(AVG(
								CASE
									WHEN COALESCE(is_stream, 0) = 1 AND first_token_sec > 0 THEN first_token_sec
									ELSE NULL
								END
							), 0) AS avg_first_token_sec,
							COALESCE(SUM(
								CASE
									WHEN COALESCE(is_stream, 0) = 1 AND first_token_sec > 0 THEN 1
									ELSE 0
								END
							), 0) AS ttft_sample_count,
							COALESCE(AVG(
								CASE
									WHEN COALESCE(is_stream, 0) = 1
										AND output_tokens > 0
										AND duration_sec > 0
									THEN CAST(output_tokens AS REAL) / duration_sec
									ELSE NULL
								END
							), 0) AS avg_tokens_per_sec,
							COALESCE(SUM(
								CASE
									WHEN COALESCE(is_stream, 0) = 1
										AND output_tokens > 0
										AND duration_sec > 0
									THEN 1
									ELSE 0
								END
							), 0) AS tps_sample_count
						FROM request_log
						WHERE created_at >= ? AND created_at < ?
					`
				args := make([]interface{}, 0, 4)
				args = append(args, startUTCKey, endUTCKey)
				query += " AND " + requestLogSuccessWhereClause("", 0)
				query += " AND " + requestLogSourceWhereClause(normalizedSourceMode, "request_log")
				if platformKey != "" {
					query += " AND platform = ?"
					args = append(args, platformKey)
				}
				if providerColumn != "" {
					query += " AND " + providerColumn + " = ?"
					args = append(args, providerValue)
				}
				if pricingModelKey := strings.TrimSpace(pricingModel); pricingModelKey != "" {
					query += " AND COALESCE(NULLIF(TRIM(matched_pricing_model), ''), TRIM(model)) = ?"
					args = append(args, pricingModelKey)
				}
				query += " GROUP BY provider_stat_key"

				rows, err := db.Query(query, args...)
				if err != nil {
					if isNoSuchTableErr(err) || strings.Contains(err.Error(), "no such column") {
						return map[string]providerPerformanceStat{}, nil
					}
					return nil, err
				}
				defer rows.Close()

				statsByProvider := make(map[string]providerPerformanceStat, 16)
				for rows.Next() {
					var providerStatKey sql.NullString
					var avgFirstTokenSec sql.NullFloat64
					var ttftSampleCount sql.NullInt64
					var avgTokensPerSec sql.NullFloat64
					var tpsSampleCount sql.NullInt64
					if err := rows.Scan(
						&providerStatKey,
						&avgFirstTokenSec,
						&ttftSampleCount,
						&avgTokensPerSec,
						&tpsSampleCount,
					); err != nil {
						return nil, err
					}

					statKey := strings.TrimSpace(providerStatKey.String)
					if statKey == "" {
						continue
					}

					firstTokenSec := 0.0
					if avgFirstTokenSec.Valid && avgFirstTokenSec.Float64 > 0 {
						firstTokenSec = avgFirstTokenSec.Float64
					}
					tokensPerSec := 0.0
					if avgTokensPerSec.Valid && avgTokensPerSec.Float64 > 0 {
						tokensPerSec = avgTokensPerSec.Float64
					}

					statsByProvider[statKey] = providerPerformanceStat{
						AvgFirstTokenSec: firstTokenSec,
						AvgTokensPerSec:  tokensPerSec,
						TTFTSampleCount:  ttftSampleCount.Int64,
						TPSSampleCount:   tpsSampleCount.Int64,
					}
				}
				if err := rows.Err(); err != nil {
					return nil, err
				}
				return statsByProvider, nil
			}

			if providerRef == "" {
				performanceMap, err = queryProviderPerformance("", "")
				if err != nil {
					return nil, err
				}
			} else {
				performanceMap, err = queryProviderPerformance("provider_id", providerRef)
				if err != nil {
					return nil, err
				}
				if len(performanceMap) == 0 {
					performanceMap, err = queryProviderPerformance("provider", providerRef)
					if err != nil {
						return nil, err
					}
				}
			}
			ls.setProviderPerformanceCache(cacheKey, performanceMap, now)
		}

		for statKey, performance := range performanceMap {
			stat := statMap[statKey]
			if stat == nil {
				continue
			}
			stat.AvgFirstTokenSec = performance.AvgFirstTokenSec
			stat.AvgTokensPerSec = performance.AvgTokensPerSec
			stat.TTFTSampleCount = performance.TTFTSampleCount
			stat.TPSSampleCount = performance.TPSSampleCount
		}
	}

	stats := make([]ProviderDailyStat, 0, len(statMap))
	for _, stat := range statMap {
		evaluatedRequests := stat.SuccessfulRequests + stat.FailedRequests
		if evaluatedRequests > 0 {
			stat.SuccessRate = float64(stat.SuccessfulRequests) / float64(evaluatedRequests)
		}
		if stat.DurationSampleCount > 0 {
			stat.AvgDurationSec = durationSums[providerStatMapKey(stat.ProviderID, stat.Provider)] / float64(stat.DurationSampleCount)
		}
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].TotalRequests == stats[j].TotalRequests {
			return stats[i].Provider < stats[j].Provider
		}
		return stats[i].TotalRequests > stats[j].TotalRequests
	})

	return stats, nil
}

func (ls *LogService) ModelStatsRangeV2(platform string, provider string, pricingModel string, startAt string, endAt string) ([]ModelUsageStat, error) {
	return ls.ModelStatsRangeV3(platform, provider, pricingModel, string(LogDataSourceModeProxy), startAt, endAt)
}

func (ls *LogService) ModelStatsRangeV3(platform string, provider string, pricingModel string, sourceMode string, startAt string, endAt string) ([]ModelUsageStat, error) {
	start, end, err := resolveAggregationRange(startAt, endAt)
	if err != nil {
		return nil, err
	}

	if !start.Before(end) {
		return []ModelUsageStat{}, nil
	}

	logs, err := ls.loadRequestLogsForAggregationV3(platform, provider, pricingModel, start, end, normalizeLogDataSourceMode(sourceMode))
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]*ModelUsageStat, 16)
	for _, logEntry := range logs {
		modelName := resolveLogPricingModel(logEntry)
		stat := grouped[modelName]
		if stat == nil {
			stat = &ModelUsageStat{Model: modelName}
			grouped[modelName] = stat
		}
		inputTokens := int64(logEntry.InputTokens)
		outputTokens := int64(logEntry.OutputTokens)
		cacheReadTokens := int64(logEntry.CacheReadTokens)

		stat.TotalRequests++
		stat.InputTokens += inputTokens
		stat.OutputTokens += outputTokens
		stat.CacheReadTokens += cacheReadTokens
		stat.TotalTokens += inputTokens + outputTokens + cacheReadTokens
		stat.CostTotal += logEntry.TotalCost
	}

	stats := make([]ModelUsageStat, 0, len(grouped))
	for _, stat := range grouped {
		stats = append(stats, *stat)
	}

	sort.Slice(stats, func(i, j int) bool {
		if stats[i].TotalTokens != stats[j].TotalTokens {
			return stats[i].TotalTokens > stats[j].TotalTokens
		}
		if stats[i].TotalRequests != stats[j].TotalRequests {
			return stats[i].TotalRequests > stats[j].TotalRequests
		}
		if stats[i].CostTotal != stats[j].CostTotal {
			return stats[i].CostTotal > stats[j].CostTotal
		}
		return stats[i].Model < stats[j].Model
	})

	return stats, nil
}

func parseCreatedAt(record xdb.Record) (time.Time, bool) {
	if t := record.GetTime("created_at"); t != nil {
		return t.In(time.Local), true
	}
	raw := strings.TrimSpace(record.GetString("created_at"))
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		timeLayout,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05-0700",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.In(time.Local), true
		}
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed.In(time.Local), true
		}
	}

	if normalized := strings.Replace(raw, " ", "T", 1); normalized != raw {
		if parsed, err := time.Parse(time.RFC3339, normalized); err == nil {
			return parsed.In(time.Local), true
		}
	}

	if len(raw) >= len("2006-01-02") {
		if parsed, err := time.ParseInLocation("2006-01-02", raw[:10], time.Local); err == nil {
			return parsed, false
		}
	}

	return time.Time{}, false
}

func parseTimeInput(value string) (time.Time, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return startOfDay(time.Now()), nil
	}

	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return parsed.In(time.Local), nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return parsed.In(time.Local), nil
	}

	localLayouts := []string{
		timeLayout,            // "2006-01-02 15:04:05" (前端本地时间，无时区)
		"2006-01-02T15:04:05", // ISO-like，无时区
	}
	for _, layout := range localLayouts {
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed, nil
		}
	}

	zoneLayouts := []string{
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 MST",
		"2006-01-02T15:04:05-0700",
	}
	for _, layout := range zoneLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.In(time.Local), nil
		}
		if parsed, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return parsed.In(time.Local), nil
		}
	}

	if len(raw) >= len("2006-01-02") {
		if parsed, err := time.ParseInLocation("2006-01-02", raw[:10], time.Local); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time format: %s", raw)
}

func dayFromTimestamp(value string) string {
	if len(value) >= len("2006-01-02") {
		if t, err := time.ParseInLocation(timeLayout, value, time.Local); err == nil {
			return t.Format("2006-01-02")
		}
		return value[:10]
	}
	return value
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfHour(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isNoSuchTableErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such table")
}

type HeatmapStat struct {
	Day                     string  `json:"day"`
	TotalRequests           int64   `json:"total_requests"`
	InputTokens             int64   `json:"input_tokens"`
	OutputTokens            int64   `json:"output_tokens"`
	CacheReadTokens         int64   `json:"cache_read_tokens"`
	ReasoningTokens         int64   `json:"reasoning_tokens"`
	TotalTokens             int64   `json:"total_tokens"`
	TotalCost               float64 `json:"total_cost"`
	PayloadBytes            int64   `json:"payload_bytes"`
	PayloadCapturedRequests int64   `json:"payload_captured_requests"`
}

func populateHeatmapTotalTokens(stat *HeatmapStat) {
	if stat == nil {
		return
	}
	stat.TotalTokens = stat.InputTokens + stat.OutputTokens + stat.CacheReadTokens
}

type LogStats struct {
	TotalRequests     int64            `json:"total_requests"`
	InputTokens       int64            `json:"input_tokens"`
	OutputTokens      int64            `json:"output_tokens"`
	ReasoningTokens   int64            `json:"reasoning_tokens"`
	CacheCreateTokens int64            `json:"cache_create_tokens"`
	CacheReadTokens   int64            `json:"cache_read_tokens"`
	CostTotal         float64          `json:"cost_total"`
	CostInput         float64          `json:"cost_input"`
	CostOutput        float64          `json:"cost_output"`
	CostCacheCreate   float64          `json:"cost_cache_create"`
	CostCacheRead     float64          `json:"cost_cache_read"`
	Series            []LogStatsSeries `json:"series"`
}

type LogSummary struct {
	TotalRequests       int64     `json:"total_requests"`
	SuccessfulRequests  int64     `json:"successful_requests"`
	FailedRequests      int64     `json:"failed_requests"`
	ExcludedRequests    int64     `json:"excluded_requests"`
	SuccessRate         float64   `json:"success_rate"`
	InputTokens         int64     `json:"input_tokens"`
	OutputTokens        int64     `json:"output_tokens"`
	CacheReadTokens     int64     `json:"cache_read_tokens"`
	TotalTokens         int64     `json:"total_tokens"`
	PeakTokens          int64     `json:"peak_tokens"`
	AvgTokensPerRequest float64   `json:"avg_tokens_per_request"`
	CostTotal           float64   `json:"cost_total"`
	CostInput           float64   `json:"cost_input"`
	CostCacheRead       float64   `json:"cost_cache_read"`
	SavedCostEstimate   float64   `json:"saved_cost_estimate"`
	ProjectedDailyCost  float64   `json:"projected_daily_cost"`
	PreviousCostTotal   float64   `json:"previous_cost_total"`
	ComparisonAvailable bool      `json:"comparison_available"`
	ActivityAvgQPS      float64   `json:"activity_avg_qps"`
	ActivityPeakQPS     float64   `json:"activity_peak_qps"`
	ActivityPoints      []float64 `json:"activity_points"`
}

type ProviderDailyStat struct {
	ProviderID           string  `json:"provider_id,omitempty"`
	Provider             string  `json:"provider"`
	TotalRequests        int64   `json:"total_requests"`
	SuccessfulRequests   int64   `json:"successful_requests"`
	FailedRequests       int64   `json:"failed_requests"`
	ExcludedRequests     int64   `json:"excluded_requests"`
	UnreadFailedRequests int64   `json:"unread_failed_requests"`
	SuccessRate          float64 `json:"success_rate"`
	InputTokens          int64   `json:"input_tokens"`
	OutputTokens         int64   `json:"output_tokens"`
	ReasoningTokens      int64   `json:"reasoning_tokens"`
	CacheCreateTokens    int64   `json:"cache_create_tokens"`
	CacheReadTokens      int64   `json:"cache_read_tokens"`
	CostTotal            float64 `json:"cost_total"`
	AvgDurationSec       float64 `json:"avg_duration_sec"`
	DurationSampleCount  int64   `json:"duration_sample_count"`
	AvgFirstTokenSec     float64 `json:"avg_first_token_sec"`
	AvgTokensPerSec      float64 `json:"avg_tokens_per_sec"`
	TTFTSampleCount      int64   `json:"ttft_sample_count"`
	TPSSampleCount       int64   `json:"tps_sample_count"`
}

type ProviderUnreadFailedCountResult struct {
	UnreadFailedRequests int64 `json:"unread_failed_requests"`
}

type ProviderUnreadFailedStat struct {
	ProviderID           string `json:"provider_id,omitempty"`
	Provider             string `json:"provider"`
	UnreadFailedRequests int64  `json:"unread_failed_requests"`
}

type ModelUsageStat struct {
	Model           string  `json:"model"`
	TotalRequests   int64   `json:"total_requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	CacheReadTokens int64   `json:"cache_read_tokens"`
	TotalTokens     int64   `json:"total_tokens"`
	CostTotal       float64 `json:"cost_total"`
}

type LogStatsSeries struct {
	Day               string  `json:"day"`
	TotalRequests     int64   `json:"total_requests"`
	InputTokens       int64   `json:"input_tokens"`
	OutputTokens      int64   `json:"output_tokens"`
	ReasoningTokens   int64   `json:"reasoning_tokens"`
	CacheCreateTokens int64   `json:"cache_create_tokens"`
	CacheReadTokens   int64   `json:"cache_read_tokens"`
	TotalCost         float64 `json:"total_cost"`
}

type ProviderPerformanceTrendPoint struct {
	BucketStart      string  `json:"bucket_start"`
	AvgFirstTokenSec float64 `json:"avg_first_token_sec"`
	AvgTokensPerSec  float64 `json:"avg_tokens_per_sec"`
	TTFTSampleCount  int64   `json:"ttft_sample_count"`
	TPSSampleCount   int64   `json:"tps_sample_count"`
}
