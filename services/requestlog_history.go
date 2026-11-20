/*
@name: 请求日志历史汇总
@Descripttion: 保存已清理明细的小时用量并恢复旧版保留的统计。
@version: 1.0.0
@Author: sm
@Date: 2026-11-20 14:16:00
@LastEditTime: 2026-11-20 14:16:00
@FilePath: services/requestlog_history.go
*/
package services

import (
	"database/sql"
	"errors"
	"strings"

	modelpricing "codeswitch/resources/model-pricing"
	"github.com/daodao97/xgo/xdb"
)

const requestLogHistoryTable = "request_log_history"
const requestLogHistoryMigrationKey = "request_log_history_v1"
const requestLogHistoryResetKey = "request_log_history_reset_id"

// 会话回执的归档状态：0 表示普通回执，1 表示已经计入历史，2 表示允许重新导入明细。
const requestLogUnarchivedWhereClause = `(COALESCE(request_log.source_record_id, '') = '' OR NOT EXISTS (
	SELECT 1 FROM session_usage_receipt r WHERE r.source_record_id = request_log.source_record_id AND r.archive_state <> 0
))`

func ensureRequestLogHistoryWithDB(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS request_log_history (
		bucket_start TEXT NOT NULL,
		platform TEXT NOT NULL,
		provider_id TEXT NOT NULL,
		provider TEXT NOT NULL,
		pricing_model TEXT NOT NULL,
		data_source TEXT NOT NULL,
		include_in_all INTEGER NOT NULL,
		total_requests INTEGER NOT NULL,
		successful_requests INTEGER NOT NULL,
		failed_requests INTEGER NOT NULL,
		excluded_requests INTEGER NOT NULL,
		input_tokens INTEGER NOT NULL,
		output_tokens INTEGER NOT NULL,
		reasoning_tokens INTEGER NOT NULL,
		cache_create_tokens INTEGER NOT NULL,
		cache_read_tokens INTEGER NOT NULL,
		peak_tokens INTEGER NOT NULL DEFAULT 0,
		total_cost REAL NOT NULL,
		input_cost REAL NOT NULL DEFAULT 0,
		output_cost REAL NOT NULL DEFAULT 0,
		cache_create_cost REAL NOT NULL DEFAULT 0,
		cache_read_cost REAL NOT NULL DEFAULT 0,
		PRIMARY KEY (bucket_start, platform, provider_id, provider, pricing_model, data_source, include_in_all)
	) WITHOUT ROWID`); err != nil {
		return err
	}
	if err := ensureRequestLogStatsColumnWithDB(db, "session_usage_receipt", "archive_state", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	applied, err := isSchemaMigrationApplied(db, requestLogHistoryMigrationKey)
	if err != nil || applied {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// 旧版小时汇总只保存代理用量。扣除仍有明细的用量，避免与实时查询重复。
	if _, err := tx.Exec(`
		WITH live AS (
			SELECT strftime('%Y-%m-%d %H:00:00', created_at, 'localtime') AS hour,
				COALESCE(platform, '') AS platform_key,
				COALESCE(NULLIF(TRIM(provider_id), ''), provider, '') AS provider_key,
				COUNT(*) AS total,
				SUM(CASE WHEN TRIM(COALESCE(request_outcome, '')) = 'excluded' THEN 1 ELSE 0 END) AS excluded,
				SUM(CASE WHEN TRIM(COALESCE(request_outcome, '')) = 'success'
					OR (TRIM(COALESCE(request_outcome, '')) NOT IN ('success', 'failure', 'excluded')
						AND http_code >= 200 AND http_code < 300) THEN 1 ELSE 0 END) AS successful,
				SUM(COALESCE(input_tokens, 0)) AS input_tokens,
				SUM(COALESCE(output_tokens, 0)) AS output_tokens,
				SUM(COALESCE(reasoning_tokens, 0)) AS reasoning_tokens,
				SUM(COALESCE(cache_create_tokens, 0)) AS cache_create_tokens,
				SUM(COALESCE(cache_read_tokens, 0)) AS cache_read_tokens,
				SUM(COALESCE(total_cost, 0)) AS total_cost
			FROM request_log WHERE COALESCE(NULLIF(TRIM(data_source), ''), 'proxy') = 'proxy'
			GROUP BY hour, platform_key, provider_key
		)
		INSERT INTO request_log_history (
			bucket_start, platform, provider_id, provider, pricing_model, data_source, include_in_all,
			total_requests, successful_requests, failed_requests, excluded_requests,
			input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens, total_cost
		)
		SELECT h.bucket_start, h.platform, h.provider_id, h.provider, '', 'proxy', 1,
			h.total_requests - COALESCE(l.total, 0),
			MAX(h.successful_requests - COALESCE(l.successful, 0), 0),
			MAX(h.failed_requests - COALESCE(l.total - l.successful - l.excluded, 0), 0),
			MAX(h.excluded_requests - COALESCE(l.excluded, 0), 0),
			MAX(h.input_tokens - COALESCE(l.input_tokens, 0), 0),
			MAX(h.output_tokens - COALESCE(l.output_tokens, 0), 0),
			MAX(h.reasoning_tokens - COALESCE(l.reasoning_tokens, 0), 0),
			MAX(h.cache_create_tokens - COALESCE(l.cache_create_tokens, 0), 0),
			MAX(h.cache_read_tokens - COALESCE(l.cache_read_tokens, 0), 0),
			MAX(h.total_cost - COALESCE(l.total_cost, 0), 0)
		FROM request_log_stats_hourly h
		LEFT JOIN live l ON l.hour = h.bucket_start AND l.platform_key = h.platform AND l.provider_key = h.provider_id
		WHERE h.total_requests > COALESCE(l.total, 0)
		ON CONFLICT DO NOTHING`); err != nil {
		return err
	}
	if _, err := tx.Exec("INSERT INTO schema_migrations (key, value) VALUES (?, ?)", requestLogHistoryMigrationKey, "1"); err != nil {
		return err
	}
	return tx.Commit()
}

type archivedLogUsage struct {
	total, successful, failed, excluded, peakTokens int64
}

type requestLogHistoryKey struct {
	hour, platform, providerID, provider, model, source string
	includeInAll                                        bool
}

// 归档和清除明细共用写事务，统计使用与日志页面相同的计费口径。
func archiveRequestLogs(tx *sql.Tx, pricing *modelpricing.Service) error {
	// 先取得写锁，防止读取待归档明细期间批量写入提交。
	if _, err := tx.Exec("UPDATE request_log_history SET total_requests = total_requests WHERE 0"); err != nil {
		return err
	}

	upsert, err := tx.Prepare(`INSERT INTO request_log_history (
		bucket_start, platform, provider_id, provider, pricing_model, data_source, include_in_all,
		total_requests, successful_requests, failed_requests, excluded_requests, input_tokens, output_tokens,
		reasoning_tokens, cache_create_tokens, cache_read_tokens, peak_tokens, total_cost, input_cost,
		output_cost, cache_create_cost, cache_read_cost
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT DO UPDATE SET
		total_requests = total_requests + excluded.total_requests,
		successful_requests = successful_requests + excluded.successful_requests,
		failed_requests = failed_requests + excluded.failed_requests,
		excluded_requests = excluded_requests + excluded.excluded_requests,
		input_tokens = input_tokens + excluded.input_tokens,
		output_tokens = output_tokens + excluded.output_tokens,
		reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
		cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
		cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
		peak_tokens = MAX(peak_tokens, excluded.peak_tokens),
		total_cost = total_cost + excluded.total_cost,
		input_cost = input_cost + excluded.input_cost,
		output_cost = output_cost + excluded.output_cost,
		cache_create_cost = cache_create_cost + excluded.cache_create_cost,
		cache_read_cost = cache_read_cost + excluded.cache_read_cost`)
	if err != nil {
		return err
	}
	defer upsert.Close()

	type bucket struct {
		usage archivedLogUsage
		stats LogStats
	}
	model := xdb.New("request_log").Tx(tx)
	const batchSize = 2000
	fields := append(append([]string{}, requestLogListSelectFields...),
		"("+requestLogSourceWhereClause(LogDataSourceModeAll, "request_log")+") AS include_in_all")
	var resetID int64
	if err := tx.QueryRow(`SELECT COALESCE(
		(SELECT CAST(value AS INTEGER) FROM schema_migrations WHERE key = ?), 0
	)`, requestLogHistoryResetKey).Scan(&resetID); err != nil {
		return err
	}
	lastID := resetID
	for {
		records, err := model.Selects(xdb.Field(fields...), xdb.WhereGt("id", lastID),
			xdb.OrderByAsc("id"), xdb.Limit(batchSize), xdb.WhereRaw(requestLogUnarchivedWhereClause))
		if err != nil && !errors.Is(err, xdb.ErrNotFound) {
			return err
		}
		buckets := make(map[requestLogHistoryKey]*bucket)
		for i, entry := range buildRequestLogList(records, pricing) {
			source := strings.TrimSpace(entry.DataSource)
			if source == "" {
				source = requestLogDataSourceProxy
			}
			createdAt, ok := parseRequestLogLocalTime(entry.CreatedAt)
			if !ok || (source != requestLogDataSourceProxy && !isSessionRequestLogSource(source)) {
				continue
			}
			key := requestLogHistoryKey{
				hour: startOfHour(createdAt).Format(timeLayout), platform: entry.Platform,
				providerID: entry.ProviderID, provider: entry.Provider,
				model:        resolveStoredPricingModel(records[i].GetString("matched_pricing_model"), entry.Model),
				source:       source,
				includeInAll: records[i].GetInt("include_in_all") != 0,
			}
			current := buckets[key]
			if current == nil {
				current = &bucket{}
				buckets[key] = current
			}
			usage := logUsage(entry)
			current.usage.total += usage.total
			current.usage.successful += usage.successful
			current.usage.failed += usage.failed
			current.usage.excluded += usage.excluded
			current.usage.peakTokens = max(current.usage.peakTokens, usage.peakTokens)
			addLogUsage(&current.stats, entry, usage.total)
		}
		for key, current := range buckets {
			usage, stats := current.usage, current.stats
			if _, err := upsert.Exec(key.hour, key.platform, key.providerID, key.provider, key.model, key.source, key.includeInAll,
				usage.total, usage.successful, usage.failed, usage.excluded, stats.InputTokens, stats.OutputTokens,
				stats.ReasoningTokens, stats.CacheCreateTokens, stats.CacheReadTokens, usage.peakTokens,
				stats.CostTotal, stats.CostInput, stats.CostOutput, stats.CostCacheCreate, stats.CostCacheRead); err != nil {
				return err
			}
		}
		if len(records) < batchSize {
			break
		}
		lastID = records[len(records)-1].GetInt64("id")
	}
	_, err = tx.Exec(`INSERT INTO session_usage_receipt (source_record_id, archive_state)
		SELECT source_record_id, 1 FROM request_log
		WHERE id > ? AND source_record_id <> '' AND `+requestLogSourceWhereClause(LogDataSourceModeSession, "request_log")+`
		ON CONFLICT(source_record_id) DO UPDATE SET archive_state = 1`, resetID)
	return err
}

func archivedRequestLogs(tx *sql.Tx, options []xdb.Option, provider string) ([]ReqeustLog, error) {
	records, err := selectRecordsByProviderRef(xdb.New(requestLogHistoryTable).Tx(tx), options, provider)
	if err != nil {
		return nil, err
	}
	logs := make([]ReqeustLog, 0, len(records))
	for _, record := range records {
		logs = append(logs, ReqeustLog{
			CreatedAt: record.GetString("bucket_start"), ProviderID: record.GetString("provider_id"),
			Provider: record.GetString("provider"), EffectivePricingModel: record.GetString("pricing_model"),
			InputTokens: record.GetInt("input_tokens"), OutputTokens: record.GetInt("output_tokens"),
			ReasoningTokens: record.GetInt("reasoning_tokens"), CacheCreateTokens: record.GetInt("cache_create_tokens"),
			CacheReadTokens: record.GetInt("cache_read_tokens"), TotalCost: record.GetFloat64("total_cost"),
			InputCost: record.GetFloat64("input_cost"), OutputCost: record.GetFloat64("output_cost"),
			CacheCreateCost: record.GetFloat64("cache_create_cost"), CacheReadCost: record.GetFloat64("cache_read_cost"),
			archivedUsage: &archivedLogUsage{
				total: record.GetInt64("total_requests"), successful: record.GetInt64("successful_requests"),
				failed: record.GetInt64("failed_requests"), excluded: record.GetInt64("excluded_requests"),
				peakTokens: record.GetInt64("peak_tokens"),
			},
		})
	}
	return logs, nil
}

func requestLogHistorySourceWhereClause(mode LogDataSourceMode) string {
	switch mode {
	case LogDataSourceModeAll:
		return "include_in_all = 1"
	case LogDataSourceModeSession:
		return "data_source IN (" + sessionDataSourceSQLList() + ")"
	default:
		return "data_source = 'proxy'"
	}
}

func logUsage(entry ReqeustLog) archivedLogUsage {
	if entry.archivedUsage != nil {
		return *entry.archivedUsage
	}
	usage := archivedLogUsage{total: 1, peakTokens: calculateLogTotalTokens(entry)}
	switch resolvedRequestLogOutcome(entry) {
	case requestOutcomeFailure:
		usage.failed = 1
	case requestOutcomeExcluded:
		usage.excluded = 1
	default:
		usage.successful = 1
	}
	return usage
}

func logRequestCount(entry ReqeustLog) int64 {
	if entry.archivedUsage != nil {
		return entry.archivedUsage.total
	}
	return 1
}
