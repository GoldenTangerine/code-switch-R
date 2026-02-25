package services

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	"github.com/daodao97/xgo/xdb"
)

const (
	requestLogStatsHourlyTable = "request_log_stats_hourly"
	requestLogStatsDailyTable  = "request_log_stats_daily"

	requestLogStatsMigrationKey = "request_log_stats_v1_backfill"
)

type requestLogStatsAgg struct {
	BucketStart        string
	Platform           string
	ProviderID         string
	Provider           string
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64
	InputTokens        int64
	OutputTokens       int64
	ReasoningTokens    int64
	CacheCreateTokens  int64
	CacheReadTokens    int64
	TotalCost          float64
}

func ensureRequestLogStatsStorageWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	if err := ensureRequestLogStatsTablesWithDB(db); err != nil {
		return err
	}
	if err := ensureRequestLogStatsTriggersWithDB(db); err != nil {
		return err
	}

	// 仅首次升级时做一次 backfill，避免用户清空统计后重启又被“自动复活”
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}
	applied, err := isSchemaMigrationApplied(db, requestLogStatsMigrationKey)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if err := backfillRequestLogStatsWithDB(db); err != nil {
		return err
	}
	if err := markSchemaMigrationApplied(db, requestLogStatsMigrationKey, time.Now().Format(timeLayout)); err != nil {
		return err
	}

	return nil
}

func ensureRequestLogStatsTablesWithDB(db *sql.DB) error {
	createHourly := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		bucket_start TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		total_requests INTEGER NOT NULL DEFAULT 0,
		successful_requests INTEGER NOT NULL DEFAULT 0,
		failed_requests INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		cache_create_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		total_cost REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_start, platform, provider_id)
	) WITHOUT ROWID`, requestLogStatsHourlyTable)

	if _, err := db.Exec(createHourly); err != nil {
		return err
	}
	if err := ensureRequestLogStatsColumnWithDB(db, requestLogStatsHourlyTable, "provider_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogStatsIdentityKeyWithDB(db, requestLogStatsHourlyTable); err != nil {
		return err
	}

	createDaily := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		bucket_start TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		total_requests INTEGER NOT NULL DEFAULT 0,
		successful_requests INTEGER NOT NULL DEFAULT 0,
		failed_requests INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		cache_create_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		total_cost REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_start, platform, provider_id)
	) WITHOUT ROWID`, requestLogStatsDailyTable)

	if _, err := db.Exec(createDaily); err != nil {
		return err
	}
	if err := ensureRequestLogStatsColumnWithDB(db, requestLogStatsDailyTable, "provider_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureRequestLogStatsIdentityKeyWithDB(db, requestLogStatsDailyTable); err != nil {
		return err
	}

	return nil
}

func ensureRequestLogStatsColumnWithDB(db *sql.DB, table string, column string, definition string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = '%s'", table, column)
	var count int
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		alter := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)
		if _, err := db.Exec(alter); err != nil {
			return err
		}
	}
	return nil
}

func ensureRequestLogStatsIdentityKeyWithDB(db *sql.DB, table string) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	table = strings.TrimSpace(table)
	if table != requestLogStatsHourlyTable && table != requestLogStatsDailyTable {
		return fmt.Errorf("invalid request log stats table: %s", table)
	}

	var createSQL sql.NullString
	if err := db.QueryRow(`
		SELECT sql
		FROM sqlite_master
		WHERE type = 'table' AND name = ?
		LIMIT 1
	`, table).Scan(&createSQL); err != nil {
		return err
	}
	definition := strings.ToLower(strings.TrimSpace(createSQL.String))
	if definition == "" {
		return nil
	}
	normalizedDefinition := strings.NewReplacer(
		" ", "",
		"\n", "",
		"\r", "",
		"\t", "",
	).Replace(definition)
	if strings.Contains(normalizedDefinition, "primarykey(bucket_start,platform,provider_id)") {
		return nil
	}

	tmpTable := table + "_v2"
	normalizedProviderID := "CASE WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN TRIM(provider_id) WHEN TRIM(COALESCE(provider, '')) <> '' THEN TRIM(provider) ELSE '__unknown__' END"

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, tmpTable)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`CREATE TABLE %s (
		bucket_start TEXT NOT NULL,
		platform TEXT NOT NULL DEFAULT '',
		provider_id TEXT NOT NULL DEFAULT '',
		provider TEXT NOT NULL DEFAULT '',
		total_requests INTEGER NOT NULL DEFAULT 0,
		successful_requests INTEGER NOT NULL DEFAULT 0,
		failed_requests INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		cache_create_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0,
		total_cost REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_start, platform, provider_id)
	) WITHOUT ROWID`, tmpTable)); err != nil {
		return err
	}

	insertSQL := fmt.Sprintf(`
		INSERT INTO %s (
			bucket_start, platform, provider_id, provider,
			total_requests, successful_requests, failed_requests,
			input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
			total_cost
		)
		SELECT
			bucket_start,
			platform,
			%s AS provider_id,
			MAX(
				CASE
					WHEN TRIM(COALESCE(provider, '')) <> '' THEN provider
					WHEN TRIM(COALESCE(provider_id, '')) <> '' THEN provider_id
					ELSE 'unknown'
				END
			) AS provider,
			SUM(total_requests),
			SUM(successful_requests),
			SUM(failed_requests),
			SUM(input_tokens),
			SUM(output_tokens),
			SUM(reasoning_tokens),
			SUM(cache_create_tokens),
			SUM(cache_read_tokens),
			SUM(total_cost)
		FROM %s
		GROUP BY bucket_start, platform, %s
	`, tmpTable, normalizedProviderID, table, normalizedProviderID)
	if _, err := tx.Exec(insertSQL); err != nil {
		return err
	}

	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE %s`, table)); err != nil {
		return err
	}
	if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, tmpTable, table)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func ensureRequestLogStatsTriggersWithDB(db *sql.DB) error {
	if err := cleanupRequestLogStatsTriggersWithDB(db); err != nil {
		return err
	}

	// 说明：
	// - request_log.created_at 使用 SQLite CURRENT_TIMESTAMP（UTC），这里用 localtime 转成本地日历桶
	// - 统计表的 bucket_start 统一存储本地时间字符串（timeLayout），便于按本地日期范围直接筛选
	createHourlyTrigger := fmt.Sprintf(`
CREATE TRIGGER IF NOT EXISTS request_log_stats_hourly_ai
AFTER INSERT ON request_log
BEGIN
  INSERT INTO %s (
    bucket_start, platform, provider_id, provider,
    total_requests, successful_requests, failed_requests,
    input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
    total_cost
	  ) VALUES (
	    strftime('%%Y-%%m-%%d %%H:00:00', datetime(NEW.created_at, 'localtime')),
	    COALESCE(NEW.platform, ''),
	    COALESCE(NULLIF(TRIM(NEW.provider_id), ''), COALESCE(NEW.provider, ''), '__unknown__'),
	    COALESCE(NEW.provider, ''),
	    1,
	    CASE WHEN COALESCE(NEW.http_code, 0) >= 200 AND COALESCE(NEW.http_code, 0) < 300 THEN 1 ELSE 0 END,
	    CASE WHEN COALESCE(NEW.http_code, 0) >= 200 AND COALESCE(NEW.http_code, 0) < 300 THEN 0 ELSE 1 END,
    COALESCE(NEW.input_tokens, 0),
    COALESCE(NEW.output_tokens, 0),
    COALESCE(NEW.reasoning_tokens, 0),
    COALESCE(NEW.cache_create_tokens, 0),
	    COALESCE(NEW.cache_read_tokens, 0),
	    COALESCE(NEW.total_cost, 0)
	  )
	  ON CONFLICT(bucket_start, platform, provider_id) DO UPDATE SET
	    provider = CASE WHEN excluded.provider <> '' THEN excluded.provider ELSE provider END,
	    total_requests = total_requests + excluded.total_requests,
	    successful_requests = successful_requests + excluded.successful_requests,
	    failed_requests = failed_requests + excluded.failed_requests,
	    input_tokens = input_tokens + excluded.input_tokens,
	    output_tokens = output_tokens + excluded.output_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    total_cost = total_cost + excluded.total_cost;
END;`, requestLogStatsHourlyTable)

	if _, err := db.Exec(createHourlyTrigger); err != nil {
		return err
	}

	createDailyTrigger := fmt.Sprintf(`
CREATE TRIGGER IF NOT EXISTS request_log_stats_daily_ai
AFTER INSERT ON request_log
BEGIN
  INSERT INTO %s (
    bucket_start, platform, provider_id, provider,
    total_requests, successful_requests, failed_requests,
    input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
    total_cost
	  ) VALUES (
	    strftime('%%Y-%%m-%%d 00:00:00', datetime(NEW.created_at, 'localtime')),
	    COALESCE(NEW.platform, ''),
	    COALESCE(NULLIF(TRIM(NEW.provider_id), ''), COALESCE(NEW.provider, ''), '__unknown__'),
	    COALESCE(NEW.provider, ''),
	    1,
	    CASE WHEN COALESCE(NEW.http_code, 0) >= 200 AND COALESCE(NEW.http_code, 0) < 300 THEN 1 ELSE 0 END,
	    CASE WHEN COALESCE(NEW.http_code, 0) >= 200 AND COALESCE(NEW.http_code, 0) < 300 THEN 0 ELSE 1 END,
    COALESCE(NEW.input_tokens, 0),
    COALESCE(NEW.output_tokens, 0),
    COALESCE(NEW.reasoning_tokens, 0),
    COALESCE(NEW.cache_create_tokens, 0),
	    COALESCE(NEW.cache_read_tokens, 0),
	    COALESCE(NEW.total_cost, 0)
	  )
	  ON CONFLICT(bucket_start, platform, provider_id) DO UPDATE SET
	    provider = CASE WHEN excluded.provider <> '' THEN excluded.provider ELSE provider END,
	    total_requests = total_requests + excluded.total_requests,
	    successful_requests = successful_requests + excluded.successful_requests,
	    failed_requests = failed_requests + excluded.failed_requests,
	    input_tokens = input_tokens + excluded.input_tokens,
	    output_tokens = output_tokens + excluded.output_tokens,
    reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
    cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
    cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
    total_cost = total_cost + excluded.total_cost;
END;`, requestLogStatsDailyTable)

	if _, err := db.Exec(createDailyTrigger); err != nil {
		return err
	}

	return nil
}

func cleanupRequestLogStatsTriggersWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}

	// 历史版本可能创建了 DELETE/UPDATE 触发器，导致用户“清空请求明细”时误伤统计表。
	// 这里统一清理除当前插入触发器以外的 request_log_stats_* 触发器。
	rows, err := db.Query("SELECT name, sql FROM sqlite_master WHERE type = 'trigger' AND tbl_name = 'request_log'")
	if err != nil {
		return err
	}
	defer rows.Close()

	keep := map[string]bool{
		"request_log_stats_hourly_ai": true,
		"request_log_stats_daily_ai":  true,
	}

	for rows.Next() {
		var name string
		var sqlText sql.NullString
		if err := rows.Scan(&name, &sqlText); err != nil {
			return err
		}
		name = strings.TrimSpace(name)
		if name == "" || keep[name] {
			continue
		}

		sqlValue := sqlText.String
		shouldDrop := strings.HasPrefix(name, "request_log_stats_") ||
			strings.Contains(sqlValue, requestLogStatsHourlyTable) ||
			strings.Contains(sqlValue, requestLogStatsDailyTable)
		if !shouldDrop {
			continue
		}

		escaped := strings.ReplaceAll(name, `"`, `""`)
		if _, err := db.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s"`, escaped)); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return nil
}

func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	return err
}

func isSchemaMigrationApplied(db *sql.DB, key string) (bool, error) {
	var value string
	err := db.QueryRow("SELECT value FROM schema_migrations WHERE key = ? LIMIT 1", key).Scan(&value)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func markSchemaMigrationApplied(db *sql.DB, key string, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO schema_migrations (key, value) VALUES (?, ?)", key, value)
	return err
}

func backfillRequestLogStatsWithDB(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	pricing, pricingErr := modelpricing.DefaultService()
	if pricingErr != nil {
		pricing = nil
	}

	model := xdb.New("request_log")

	const batchSize = 5000
	lastID := int64(0)

	hourlyUpsert := fmt.Sprintf(`
	INSERT INTO %s (
	  bucket_start, platform, provider_id, provider,
	  total_requests, successful_requests, failed_requests,
	  input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
	  total_cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(bucket_start, platform, provider_id) DO UPDATE SET
	  provider = CASE WHEN excluded.provider <> '' THEN excluded.provider ELSE provider END,
	  total_requests = total_requests + excluded.total_requests,
	  successful_requests = successful_requests + excluded.successful_requests,
	  failed_requests = failed_requests + excluded.failed_requests,
	  input_tokens = input_tokens + excluded.input_tokens,
	  output_tokens = output_tokens + excluded.output_tokens,
  reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
  cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
  cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
  total_cost = total_cost + excluded.total_cost
`, requestLogStatsHourlyTable)

	dailyUpsert := fmt.Sprintf(`
	INSERT INTO %s (
	  bucket_start, platform, provider_id, provider,
	  total_requests, successful_requests, failed_requests,
	  input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
	  total_cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(bucket_start, platform, provider_id) DO UPDATE SET
	  provider = CASE WHEN excluded.provider <> '' THEN excluded.provider ELSE provider END,
	  total_requests = total_requests + excluded.total_requests,
	  successful_requests = successful_requests + excluded.successful_requests,
	  failed_requests = failed_requests + excluded.failed_requests,
	  input_tokens = input_tokens + excluded.input_tokens,
	  output_tokens = output_tokens + excluded.output_tokens,
  reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
  cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
  cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
  total_cost = total_cost + excluded.total_cost
`, requestLogStatsDailyTable)

	for {
		records, err := model.Selects(
			xdb.WhereGt("id", lastID),
			xdb.OrderByAsc("id"),
			xdb.Limit(batchSize),
			xdb.Field(
				"id",
				"platform",
				"provider_id",
				"provider",
				"model",
				"http_code",
				"input_tokens",
				"output_tokens",
				"reasoning_tokens",
				"cache_create_tokens",
				"cache_read_tokens",
				"total_cost",
				"created_at",
			),
		)
		if err != nil {
			if errors.Is(err, xdb.ErrNotFound) || isNoSuchTableErr(err) {
				return nil
			}
			return err
		}
		if len(records) == 0 {
			return nil
		}

		hourly := map[string]*requestLogStatsAgg{}
		daily := map[string]*requestLogStatsAgg{}

		for _, record := range records {
			id := record.GetInt64("id")
			if id > lastID {
				lastID = id
			}

			platform := strings.TrimSpace(record.GetString("platform"))
			providerID := strings.TrimSpace(record.GetString("provider_id"))
			provider := strings.TrimSpace(record.GetString("provider"))
			if providerID == "" {
				providerID = provider
			}
			if providerID == "" {
				providerID = "__unknown__"
			}

			createdAtLocal, _ := parseCreatedAt(record)
			if createdAtLocal.IsZero() {
				continue
			}
			hourBucket := startOfHour(createdAtLocal).Format(timeLayout)
			dayBucket := startOfDay(createdAtLocal).Format(timeLayout)

			input := record.GetInt("input_tokens")
			output := record.GetInt("output_tokens")
			reasoning := record.GetInt("reasoning_tokens")
			cacheCreate := record.GetInt("cache_create_tokens")
			cacheRead := record.GetInt("cache_read_tokens")
			totalCost := record.GetFloat64("total_cost")
			totalCost = estimateBackfillTotalCost(
				pricing,
				record.GetString("model"),
				input,
				output,
				reasoning,
				cacheCreate,
				cacheRead,
				totalCost,
			)

			httpCode := record.GetInt("http_code")
			success := int64(0)
			fail := int64(1)
			if httpCode >= 200 && httpCode < 300 {
				success = 1
				fail = 0
			}

			hourKey := fmt.Sprintf("%s|%s|%s", hourBucket, platform, providerID)
			if agg := hourly[hourKey]; agg != nil {
				agg.TotalRequests++
				agg.SuccessfulRequests += success
				agg.FailedRequests += fail
				agg.InputTokens += int64(input)
				agg.OutputTokens += int64(output)
				agg.ReasoningTokens += int64(reasoning)
				agg.CacheCreateTokens += int64(cacheCreate)
				agg.CacheReadTokens += int64(cacheRead)
				agg.TotalCost += totalCost
			} else {
				hourly[hourKey] = &requestLogStatsAgg{
					BucketStart:        hourBucket,
					Platform:           platform,
					ProviderID:         providerID,
					Provider:           provider,
					TotalRequests:      1,
					SuccessfulRequests: success,
					FailedRequests:     fail,
					InputTokens:        int64(input),
					OutputTokens:       int64(output),
					ReasoningTokens:    int64(reasoning),
					CacheCreateTokens:  int64(cacheCreate),
					CacheReadTokens:    int64(cacheRead),
					TotalCost:          totalCost,
				}
			}

			dayKey := fmt.Sprintf("%s|%s|%s", dayBucket, platform, providerID)
			if agg := daily[dayKey]; agg != nil {
				agg.TotalRequests++
				agg.SuccessfulRequests += success
				agg.FailedRequests += fail
				agg.InputTokens += int64(input)
				agg.OutputTokens += int64(output)
				agg.ReasoningTokens += int64(reasoning)
				agg.CacheCreateTokens += int64(cacheCreate)
				agg.CacheReadTokens += int64(cacheRead)
				agg.TotalCost += totalCost
			} else {
				daily[dayKey] = &requestLogStatsAgg{
					BucketStart:        dayBucket,
					Platform:           platform,
					ProviderID:         providerID,
					Provider:           provider,
					TotalRequests:      1,
					SuccessfulRequests: success,
					FailedRequests:     fail,
					InputTokens:        int64(input),
					OutputTokens:       int64(output),
					ReasoningTokens:    int64(reasoning),
					CacheCreateTokens:  int64(cacheCreate),
					CacheReadTokens:    int64(cacheRead),
					TotalCost:          totalCost,
				}
			}
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		for _, agg := range hourly {
			if _, err := tx.Exec(
				hourlyUpsert,
				agg.BucketStart,
				agg.Platform,
				agg.ProviderID,
				agg.Provider,
				agg.TotalRequests,
				agg.SuccessfulRequests,
				agg.FailedRequests,
				agg.InputTokens,
				agg.OutputTokens,
				agg.ReasoningTokens,
				agg.CacheCreateTokens,
				agg.CacheReadTokens,
				agg.TotalCost,
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		for _, agg := range daily {
			if _, err := tx.Exec(
				dailyUpsert,
				agg.BucketStart,
				agg.Platform,
				agg.ProviderID,
				agg.Provider,
				agg.TotalRequests,
				agg.SuccessfulRequests,
				agg.FailedRequests,
				agg.InputTokens,
				agg.OutputTokens,
				agg.ReasoningTokens,
				agg.CacheCreateTokens,
				agg.CacheReadTokens,
				agg.TotalCost,
			); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
}

func estimateBackfillTotalCost(
	pricing *modelpricing.Service,
	model string,
	inputTokens int,
	outputTokens int,
	reasoningTokens int,
	cacheCreateTokens int,
	cacheReadTokens int,
	storedTotalCost float64,
) float64 {
	if storedTotalCost > 0 || pricing == nil {
		return storedTotalCost
	}

	if strings.TrimSpace(model) == "" {
		return storedTotalCost
	}

	if inputTokens <= 0 && outputTokens <= 0 && reasoningTokens <= 0 && cacheCreateTokens <= 0 && cacheReadTokens <= 0 {
		return storedTotalCost
	}

	usage := modelpricing.UsageSnapshot{
		InputTokens:       inputTokens,
		OutputTokens:      outputTokens,
		ReasoningTokens:   reasoningTokens,
		CacheCreateTokens: cacheCreateTokens,
		CacheReadTokens:   cacheReadTokens,
	}

	cost := pricing.CalculateCost(model, usage)
	if cost.TotalCost > 0 {
		return cost.TotalCost
	}
	return storedTotalCost
}
