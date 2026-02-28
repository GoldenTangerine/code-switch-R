package services

import (
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const timeLayout = "2006-01-02 15:04:05"

type LogService struct {
	modelPricing *ModelPricingService
}

type requestLogSelecter interface {
	Selects(...xdb.Option) ([]xdb.Record, error)
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

	query := `SELECT COALESCE(SUM(total_cost), 0) FROM request_log WHERE created_at >= ?`
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

func NewLogService(modelPricing *ModelPricingService) *LogService {
	return &LogService{modelPricing: modelPricing}
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

func (ls *LogService) ListRequestLogs(platform string, provider string, limit int) ([]ReqeustLog, error) {
	return ls.ListRequestLogsV2(platform, provider, limit, "", "")
}

func (ls *LogService) ListRequestLogsV2(platform string, provider string, limit int, startAt string, endAt string) ([]ReqeustLog, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.OrderByDesc("created_at"),
		xdb.OrderByDesc("id"),
		xdb.Limit(limit),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
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
	records, err := selectRecordsByProviderRef(model, options, provider)
	if err != nil {
		return nil, err
	}
	pricingSnapshot := ls.resolvePricingSnapshot()
	logs := make([]ReqeustLog, 0, len(records))
	for _, record := range records {
		createdAtLocal, _ := parseCreatedAt(record)
		createdAtValue := record.GetString("created_at")
		if !createdAtLocal.IsZero() {
			createdAtValue = createdAtLocal.Format(timeLayout)
		}
		logEntry := ReqeustLog{
			ID:                        record.GetInt64("id"),
			Platform:                  record.GetString("platform"),
			Model:                     record.GetString("model"),
			RequestedModel:            record.GetString("requested_model"),
			ResponseModel:             record.GetString("response_model"),
			ProviderID:                record.GetString("provider_id"),
			Provider:                  record.GetString("provider"),
			PriceSource:               record.GetString("price_source"),
			HttpCode:                  record.GetInt("http_code"),
			InputTokens:               record.GetInt("input_tokens"),
			OutputTokens:              record.GetInt("output_tokens"),
			CacheCreateTokens:         record.GetInt("cache_create_tokens"),
			Ephemeral5mTokens:         record.GetInt("ephemeral_5m_tokens"),
			Ephemeral1hTokens:         record.GetInt("ephemeral_1h_tokens"),
			CacheReadTokens:           record.GetInt("cache_read_tokens"),
			ReasoningTokens:           record.GetInt("reasoning_tokens"),
			CreatedAt:                 createdAtValue,
			IsStream:                  record.GetBool("is_stream"),
			DurationSec:               record.GetFloat64("duration_sec"),
			InputCost:                 record.GetFloat64("input_cost"),
			OutputCost:                record.GetFloat64("output_cost"),
			ReasoningCost:             record.GetFloat64("reasoning_cost"),
			CacheCreateCost:           record.GetFloat64("cache_create_cost"),
			CacheReadCost:             record.GetFloat64("cache_read_cost"),
			Ephemeral5mCost:           record.GetFloat64("ephemeral_5m_cost"),
			Ephemeral1hCost:           record.GetFloat64("ephemeral_1h_cost"),
			TotalCost:                 record.GetFloat64("total_cost"),
			HasPricing:                record.GetBool("has_pricing"),
			MatchedPricingModel:       record.GetString("matched_pricing_model"),
			ProviderPricingAvailable:  record.GetBool("provider_pricing_available"),
			ProviderQuotaType:         record.GetInt("provider_quota_type"),
			ProviderInputUSDPerM:      record.GetFloat64("provider_input_usd_per_m"),
			ProviderOutputUSDPerM:     record.GetFloat64("provider_output_usd_per_m"),
			ProviderPerCallUnified:    record.GetFloat64("provider_per_call_unified"),
			ProviderPerCallInput:      record.GetFloat64("provider_per_call_input"),
			ProviderPerCallOutput:     record.GetFloat64("provider_per_call_output"),
			ProviderPerCallUnifiedSet: record.GetBool("provider_per_call_unified_set"),
			ProviderPerCallInputSet:   record.GetBool("provider_per_call_input_set"),
			ProviderPerCallOutputSet:  record.GetBool("provider_per_call_output_set"),
		}
		applyLogPricing(pricingSnapshot, &logEntry)
		logs = append(logs, logEntry)
	}
	return logs, nil
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
		return requestLogPriceSourceNone
	default:
		if totalCost > 0 {
			return requestLogPriceSourceBuiltin
		}
		return requestLogPriceSourceNone
	}
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

func applyLogPricing(pricing *modelpricing.Service, logEntry *ReqeustLog) {
	if logEntry == nil {
		return
	}

	logEntry.PriceSource = normalizeRequestLogPriceSource(logEntry.PriceSource, logEntry.TotalCost)

	if logEntry.TotalCost > 0 {
		logEntry.HasPricing = true
	}
	if logEntry.PriceSource == requestLogPriceSourceProviderAPI {
		if logEntry.ProviderPricingAvailable || hasStoredBreakdownCost(logEntry) {
			logEntry.HasPricing = true
			return
		}
		if pricing == nil || strings.TrimSpace(logEntry.Model) == "" {
			logEntry.HasPricing = true
			return
		}
	} else if pricing == nil || strings.TrimSpace(logEntry.Model) == "" {
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

	breakdown := pricing.CalculateCost(logEntry.Model, usage)
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
	logEntry.HasPricing = true
	if logEntry.PriceSource == requestLogPriceSourceNone {
		logEntry.PriceSource = requestLogPriceSourceBuiltin
	}
	if logEntry.TotalCost <= 0 && breakdown.TotalCost > 0 && logEntry.PriceSource != requestLogPriceSourceProviderAPI {
		logEntry.TotalCost = breakdown.TotalCost
		logEntry.PriceSource = requestLogPriceSourceBuiltin
	}
	if breakdown.FuzzyMatched &&
		strings.TrimSpace(breakdown.PricingModel) != "" &&
		logEntry.PriceSource != requestLogPriceSourceProviderAPI &&
		!strings.EqualFold(strings.TrimSpace(logEntry.Model), strings.TrimSpace(breakdown.PricingModel)) {
		logEntry.MatchedPricingModel = breakdown.PricingModel
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
		return stats, nil
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
			&stat.ReasoningTokens,
			&stat.TotalCost,
		); err != nil {
			return nil, err
		}
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
		reasoning := record.GetInt("reasoning_tokens")
		bucket.InputTokens += int64(input)
		bucket.OutputTokens += int64(output)
		bucket.ReasoningTokens += int64(reasoning)
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
	seriesEnd := seriesStart.Add(24 * time.Hour)
	return ls.StatsRangeV2(platform, "", seriesStart.Format(timeLayout), seriesEnd.Format(timeLayout))
}

func (ls *LogService) StatsRangeV2(platform string, provider string, startAt string, endAt string) (LogStats, error) {
	stats := LogStats{
		Series: make([]LogStatsSeries, 0),
	}

	start, err := parseTimeInput(startAt)
	if err != nil {
		return stats, err
	}

	end := time.Time{}
	if strings.TrimSpace(endAt) != "" {
		end, err = parseTimeInput(endAt)
		if err != nil {
			return stats, err
		}
	} else {
		end = start.Add(24 * time.Hour)
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

	startKey := start.Format(timeLayout)
	endKey := end.Format(timeLayout)
	tableName := requestLogStatsHourlyTable
	if useDayBuckets {
		tableName = requestLogStatsDailyTable
		startKey = startOfDay(start).Format(timeLayout)
		endKey = startOfDay(end).Format(timeLayout)
	}
	model := xdb.New(tableName)
	options := []xdb.Option{
		xdb.WhereGte("bucket_start", startKey),
		xdb.WhereLt("bucket_start", endKey),
		xdb.Field(
			"provider",
			"bucket_start",
			"total_requests",
			"input_tokens",
			"output_tokens",
			"reasoning_tokens",
			"cache_create_tokens",
			"cache_read_tokens",
			"total_cost",
		),
		xdb.OrderByAsc("bucket_start"),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	records, err := selectRecordsByProviderRef(model, options, provider)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return stats, nil
		}
		return stats, err
	}

	bucketByKey := make(map[string]*LogStatsSeries, bucketCount)
	for i := range seriesBuckets {
		if seriesBuckets[i] != nil {
			bucketByKey[seriesBuckets[i].Day] = seriesBuckets[i]
		}
	}

	for _, record := range records {
		key := strings.TrimSpace(record.GetString("bucket_start"))
		bucket := bucketByKey[key]
		if bucket == nil {
			continue
		}
		total := record.GetInt64("total_requests")
		input := record.GetInt64("input_tokens")
		output := record.GetInt64("output_tokens")
		reasoning := record.GetInt64("reasoning_tokens")
		cacheCreate := record.GetInt64("cache_create_tokens")
		cacheRead := record.GetInt64("cache_read_tokens")
		costTotal := record.GetFloat64("total_cost")

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

func (ls *LogService) ProviderDailyStats(platform string) ([]ProviderDailyStat, error) {
	start := startOfDay(time.Now())
	end := start.Add(24 * time.Hour)
	return ls.ProviderStatsRangeV2(platform, "", start.Format(timeLayout), end.Format(timeLayout))
}

func (ls *LogService) ProviderStatsRangeV2(platform string, provider string, startAt string, endAt string) ([]ProviderDailyStat, error) {
	start, err := parseTimeInput(startAt)
	if err != nil {
		return nil, err
	}

	end := time.Time{}
	if strings.TrimSpace(endAt) != "" {
		end, err = parseTimeInput(endAt)
		if err != nil {
			return nil, err
		}
	} else {
		end = start.Add(24 * time.Hour)
	}

	if !start.Before(end) {
		return []ProviderDailyStat{}, nil
	}

	duration := end.Sub(start)
	startKey := start.Format(timeLayout)
	endKey := end.Format(timeLayout)
	tableName := requestLogStatsHourlyTable
	if duration > 48*time.Hour {
		tableName = requestLogStatsDailyTable
		startKey = startOfDay(start).Format(timeLayout)
		endKey = startOfDay(end).Format(timeLayout)
	}
	model := xdb.New(tableName)
	options := []xdb.Option{
		xdb.WhereGte("bucket_start", startKey),
		xdb.WhereLt("bucket_start", endKey),
		xdb.Field(
			"provider_id",
			"provider",
			"total_requests",
			"successful_requests",
			"failed_requests",
			"input_tokens",
			"output_tokens",
			"reasoning_tokens",
			"cache_create_tokens",
			"cache_read_tokens",
			"total_cost",
		),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	records, err := selectRecordsByProviderRef(model, options, provider)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return []ProviderDailyStat{}, nil
		}
		return nil, err
	}

	statMap := map[string]*ProviderDailyStat{}
	for _, record := range records {
		providerID := strings.TrimSpace(record.GetString("provider_id"))
		providerName := strings.TrimSpace(record.GetString("provider"))
		if providerName == "" {
			providerName = "(unknown)"
		}
		statKey := providerID
		if statKey == "" {
			statKey = providerName
		}

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

		total := record.GetInt64("total_requests")
		success := record.GetInt64("successful_requests")
		fail := record.GetInt64("failed_requests")
		stat.TotalRequests += total
		stat.SuccessfulRequests += success
		stat.FailedRequests += fail
		stat.InputTokens += record.GetInt64("input_tokens")
		stat.OutputTokens += record.GetInt64("output_tokens")
		stat.ReasoningTokens += record.GetInt64("reasoning_tokens")
		stat.CacheCreateTokens += record.GetInt64("cache_create_tokens")
		stat.CacheReadTokens += record.GetInt64("cache_read_tokens")
		stat.CostTotal += record.GetFloat64("total_cost")
	}

	stats := make([]ProviderDailyStat, 0, len(statMap))
	for _, stat := range statMap {
		if stat.TotalRequests > 0 {
			stat.SuccessRate = float64(stat.SuccessfulRequests) / float64(stat.TotalRequests)
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

func (ls *LogService) ModelStatsRangeV2(platform string, provider string, startAt string, endAt string) ([]ModelUsageStat, error) {
	start, err := parseTimeInput(startAt)
	if err != nil {
		return nil, err
	}

	end := time.Time{}
	if strings.TrimSpace(endAt) != "" {
		end, err = parseTimeInput(endAt)
		if err != nil {
			return nil, err
		}
	} else {
		end = start.Add(24 * time.Hour)
	}

	if !start.Before(end) {
		return []ModelUsageStat{}, nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}

	startKey := start.UTC().Format(timeLayout)
	endKey := end.UTC().Format(timeLayout)
	platformKey := strings.TrimSpace(platform)

	queryModelStats := func(providerColumn string, providerValue string) ([]ModelUsageStat, error) {
		query := `
			SELECT
				CASE
					WHEN TRIM(COALESCE(model, '')) = '' THEN '—'
					ELSE TRIM(model)
				END AS model_name,
				COUNT(*) AS total_requests,
				COALESCE(SUM(CASE WHEN input_tokens > 0 THEN input_tokens ELSE 0 END), 0) AS input_tokens,
				COALESCE(SUM(CASE WHEN output_tokens > 0 THEN output_tokens ELSE 0 END), 0) AS output_tokens,
				COALESCE(SUM(CASE WHEN cache_read_tokens > 0 THEN cache_read_tokens ELSE 0 END), 0) AS cache_read_tokens,
				COALESCE(SUM(CASE WHEN input_tokens > 0 THEN input_tokens ELSE 0 END), 0)
				  + COALESCE(SUM(CASE WHEN output_tokens > 0 THEN output_tokens ELSE 0 END), 0)
				  + COALESCE(SUM(CASE WHEN cache_read_tokens > 0 THEN cache_read_tokens ELSE 0 END), 0) AS total_tokens,
				COALESCE(SUM(total_cost), 0) AS total_cost
			FROM request_log
			WHERE created_at >= ? AND created_at < ?
		`
		args := make([]interface{}, 0, 4)
		args = append(args, startKey, endKey)
		if platformKey != "" {
			query += " AND platform = ?"
			args = append(args, platformKey)
		}
		if providerColumn != "" {
			query += " AND " + providerColumn + " = ?"
			args = append(args, providerValue)
		}
		query += `
			GROUP BY model_name
			ORDER BY total_tokens DESC, total_requests DESC, total_cost DESC, model_name ASC
		`

		rows, err := db.Query(query, args...)
		if err != nil {
			if isNoSuchTableErr(err) {
				return []ModelUsageStat{}, nil
			}
			return nil, err
		}
		defer rows.Close()

		stats := make([]ModelUsageStat, 0, 16)
		for rows.Next() {
			stat := ModelUsageStat{}
			if err := rows.Scan(
				&stat.Model,
				&stat.TotalRequests,
				&stat.InputTokens,
				&stat.OutputTokens,
				&stat.CacheReadTokens,
				&stat.TotalTokens,
				&stat.CostTotal,
			); err != nil {
				return nil, err
			}
			stats = append(stats, stat)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return stats, nil
	}

	providerRef := strings.TrimSpace(provider)
	if providerRef == "" {
		return queryModelStats("", "")
	}

	stats, err := queryModelStats("provider_id", providerRef)
	if err != nil {
		return nil, err
	}
	if len(stats) > 0 {
		return stats, nil
	}
	return queryModelStats("provider", providerRef)
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
	Day             string  `json:"day"`
	TotalRequests   int64   `json:"total_requests"`
	InputTokens     int64   `json:"input_tokens"`
	OutputTokens    int64   `json:"output_tokens"`
	ReasoningTokens int64   `json:"reasoning_tokens"`
	TotalCost       float64 `json:"total_cost"`
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

type ProviderDailyStat struct {
	ProviderID         string  `json:"provider_id,omitempty"`
	Provider           string  `json:"provider"`
	TotalRequests      int64   `json:"total_requests"`
	SuccessfulRequests int64   `json:"successful_requests"`
	FailedRequests     int64   `json:"failed_requests"`
	SuccessRate        float64 `json:"success_rate"`
	InputTokens        int64   `json:"input_tokens"`
	OutputTokens       int64   `json:"output_tokens"`
	ReasoningTokens    int64   `json:"reasoning_tokens"`
	CacheCreateTokens  int64   `json:"cache_create_tokens"`
	CacheReadTokens    int64   `json:"cache_read_tokens"`
	CostTotal          float64 `json:"cost_total"`
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
