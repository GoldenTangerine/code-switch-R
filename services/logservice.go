package services

import (
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
	if provider != "" {
		options = append(options, xdb.WhereEq("provider", provider))
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
	records, err := model.Selects(options...)
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
			ID:                record.GetInt64("id"),
			Platform:          record.GetString("platform"),
			Model:             record.GetString("model"),
			RequestedModel:    record.GetString("requested_model"),
			ResponseModel:     record.GetString("response_model"),
			Provider:          record.GetString("provider"),
			PriceSource:       record.GetString("price_source"),
			HttpCode:          record.GetInt("http_code"),
			InputTokens:       record.GetInt("input_tokens"),
			OutputTokens:      record.GetInt("output_tokens"),
			CacheCreateTokens: record.GetInt("cache_create_tokens"),
			CacheReadTokens:   record.GetInt("cache_read_tokens"),
			ReasoningTokens:   record.GetInt("reasoning_tokens"),
			CreatedAt:         createdAtValue,
			IsStream:          record.GetBool("is_stream"),
			DurationSec:       record.GetFloat64("duration_sec"),
			TotalCost:         record.GetFloat64("total_cost"),
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

func applyLogPricing(pricing *modelpricing.Service, logEntry *ReqeustLog) {
	if logEntry == nil {
		return
	}

	logEntry.PriceSource = normalizeRequestLogPriceSource(logEntry.PriceSource, logEntry.TotalCost)

	if logEntry.TotalCost > 0 {
		logEntry.HasPricing = true
	}
	if logEntry.PriceSource == requestLogPriceSourceProviderAPI {
		logEntry.HasPricing = true
		return
	}

	if pricing == nil || strings.TrimSpace(logEntry.Model) == "" {
		return
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:       logEntry.InputTokens,
		OutputTokens:      logEntry.OutputTokens,
		ReasoningTokens:   logEntry.ReasoningTokens,
		CacheCreateTokens: logEntry.CacheCreateTokens,
		CacheReadTokens:   logEntry.CacheReadTokens,
	}

	breakdown := pricing.CalculateCost(logEntry.Model, usage)
	logEntry.InputCost = breakdown.InputCost
	logEntry.OutputCost = breakdown.OutputCost
	logEntry.ReasoningCost = breakdown.ReasoningCost
	logEntry.CacheCreateCost = breakdown.CacheCreateCost
	logEntry.CacheReadCost = breakdown.CacheReadCost
	logEntry.Ephemeral5mCost = breakdown.Ephemeral5mCost
	logEntry.Ephemeral1hCost = breakdown.Ephemeral1hCost
	if breakdown.HasPricing {
		logEntry.HasPricing = true
		if logEntry.PriceSource == requestLogPriceSourceNone {
			logEntry.PriceSource = requestLogPriceSourceBuiltin
		}
	}
	if logEntry.TotalCost <= 0 && breakdown.TotalCost > 0 {
		logEntry.TotalCost = breakdown.TotalCost
		logEntry.PriceSource = requestLogPriceSourceBuiltin
	}
	if breakdown.FuzzyMatched &&
		strings.TrimSpace(breakdown.PricingModel) != "" &&
		!strings.EqualFold(strings.TrimSpace(logEntry.Model), strings.TrimSpace(breakdown.PricingModel)) {
		logEntry.MatchedPricingModel = breakdown.PricingModel
	}
}

func (ls *LogService) ListProviders(platform string) ([]string, error) {
	model := xdb.New("request_log")
	options := []xdb.Option{
		xdb.Field("DISTINCT provider as provider"),
		xdb.WhereNotEq("provider", ""),
		xdb.OrderByAsc("provider"),
	}
	if platform != "" {
		options = append(options, xdb.WhereEq("platform", platform))
	}
	records, err := model.Selects(options...)
	if err != nil {
		return nil, err
	}
	providers := make([]string, 0, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.GetString("provider"))
		if name != "" {
			providers = append(providers, name)
		}
	}
	return providers, nil
}

func (ls *LogService) HeatmapStats(days int) ([]HeatmapStat, error) {
	if days <= 0 {
		days = 30
	}
	totalHours := days * 24
	if totalHours <= 0 {
		totalHours = 24
	}
	rangeStart := startOfHour(time.Now())
	if totalHours > 1 {
		rangeStart = rangeStart.Add(-time.Duration(totalHours-1) * time.Hour)
	}
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
			bucket = &HeatmapStat{Day: hourStart.Format("01-02 15")}
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
	if provider != "" {
		options = append(options, xdb.WhereEq("provider", provider))
	}
	records, err := model.Selects(options...)
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
	if provider != "" {
		options = append(options, xdb.WhereEq("provider", provider))
	}
	records, err := model.Selects(options...)
	if err != nil {
		if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
			return []ProviderDailyStat{}, nil
		}
		return nil, err
	}

	statMap := map[string]*ProviderDailyStat{}
	for _, record := range records {
		providerName := strings.TrimSpace(record.GetString("provider"))
		if providerName == "" {
			providerName = "(unknown)"
		}

		stat := statMap[providerName]
		if stat == nil {
			stat = &ProviderDailyStat{Provider: providerName}
			statMap[providerName] = stat
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
