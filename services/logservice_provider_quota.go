package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// CostSinceByProvider 查询指定供应商从 start 时间起的累计费用。
// providerID 优先命中新身份；providerName 兜底兼容历史无 provider_id 的旧日志。
func (ls *LogService) CostSinceByProvider(start string, platform string, providerID string, providerName string) (float64, error) {
	startTime, err := parseTimeInput(start)
	if err != nil {
		return 0, err
	}

	return ls.costByProvider(platform, providerID, providerName, &startTime)
}

// CostByProvider 查询指定供应商的累计总费用。
// 优先使用 request_log_stats_daily 聚合表，兼容老库时回退 request_log 明细表。
func (ls *LogService) CostByProvider(platform string, providerID string, providerName string) (float64, error) {
	return ls.costByProvider(platform, providerID, providerName, nil)
}

func (ls *LogService) costByProvider(platform string, providerID string, providerName string, startTime *time.Time) (float64, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return 0, err
	}

	providerID = strings.TrimSpace(providerID)
	providerName = strings.TrimSpace(providerName)
	platform = strings.TrimSpace(platform)
	if providerID == "" && providerName == "" {
		return 0, nil
	}

	if startTime == nil {
		total, ok, err := queryProviderQuotaCostFromDailyStats(db, platform, providerID, providerName)
		if err != nil {
			return 0, err
		}
		if ok {
			return total, nil
		}
	}

	return queryProviderQuotaCostFromRequestLog(db, platform, providerID, providerName, startTime)
}

func queryProviderQuotaCostFromDailyStats(db *sql.DB, platform string, providerID string, providerName string) (float64, bool, error) {
	total, err := queryProviderQuotaCost(
		db,
		requestLogStatsDailyTable,
		platform,
		providerID,
		providerName,
		nil,
		true,
	)
	if err != nil {
		if isMissingSQLiteObjectErr(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return total, true, nil
}

func queryProviderQuotaCostFromRequestLog(db *sql.DB, platform string, providerID string, providerName string, startTime *time.Time) (float64, error) {
	total, err := queryProviderQuotaCost(
		db,
		"request_log",
		platform,
		providerID,
		providerName,
		startTime,
		false,
	)
	if err != nil {
		if isMissingSQLiteObjectErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}

func queryProviderQuotaCost(
	db *sql.DB,
	table string,
	platform string,
	providerID string,
	providerName string,
	startTime *time.Time,
	useStatsTable bool,
) (float64, error) {
	query, args := buildProviderQuotaCostQuery(table, platform, providerID, providerName, startTime, useStatsTable)
	if query == "" {
		return 0, nil
	}

	total := 0.0
	if err := db.QueryRow(query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func buildProviderQuotaCostQuery(
	table string,
	platform string,
	providerID string,
	providerName string,
	startTime *time.Time,
	useStatsTable bool,
) (string, []any) {
	table = strings.TrimSpace(table)
	if table == "" {
		return "", nil
	}

	whereClauses := []string{"1=1"}
	args := make([]any, 0, 5)
	if !useStatsTable && table == "request_log" {
		whereClauses = append(whereClauses, requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log"))
	}

	if startTime != nil {
		whereClauses = append(whereClauses, "created_at >= ?")
		args = append(args, startTime.UTC().Format(timeLayout))
	}

	if platform != "" {
		whereClauses = append(whereClauses, "platform = ?")
		args = append(args, platform)
	}

	providerClause, providerArgs := buildProviderQuotaIdentityWhereClause(providerID, providerName, useStatsTable)
	if providerClause == "" {
		return "", nil
	}
	whereClauses = append(whereClauses, providerClause)
	args = append(args, providerArgs...)

	return fmt.Sprintf(
		"SELECT COALESCE(SUM(total_cost), 0) FROM %s WHERE %s",
		table,
		strings.Join(whereClauses, " AND "),
	), args
}

func buildProviderQuotaIdentityWhereClause(providerID string, providerName string, useStatsTable bool) (string, []any) {
	providerID = strings.TrimSpace(providerID)
	providerName = strings.TrimSpace(providerName)

	switch {
	case providerID != "" && providerName != "":
		if useStatsTable {
			return strings.Join([]string{
				"(",
				"provider_id = ?",
				"OR provider_id = ?",
				"OR ((provider_id = '' OR provider_id IS NULL) AND provider = ?)",
				")",
			}, " "), []any{providerID, providerName, providerName}
		}
		return strings.Join([]string{
			"(",
			"provider_id = ?",
			"OR ((provider_id = '' OR provider_id IS NULL) AND provider = ?)",
			")",
		}, " "), []any{providerID, providerName}
	case providerID != "":
		return "provider_id = ?", []any{providerID}
	case providerName != "":
		if useStatsTable {
			return strings.Join([]string{
				"(",
				"provider_id = ?",
				"OR ((provider_id = '' OR provider_id IS NULL) AND provider = ?)",
				")",
			}, " "), []any{providerName, providerName}
		}
		return "provider = ?", []any{providerName}
	default:
		return "", nil
	}
}

func isMissingSQLiteObjectErr(err error) bool {
	if err == nil {
		return false
	}
	return isNoSuchTableErr(err) || strings.Contains(err.Error(), "no such column")
}

// ResolveFiveHourQuotaStatusByProvider 解析指定供应商的 5 小时额度周期状态。
// 运行时优先读取 provider 级持久化周期状态，保持与全局 tray 的 5 小时额度口径一致。
func (ls *LogService) ResolveFiveHourQuotaStatusByProvider(platform string, providerID string, providerName string) (FiveHourQuotaStatus, error) {
	return ls.resolveFiveHourQuotaStatusByProviderAt(platform, providerID, providerName, time.Now())
}

func (ls *LogService) resolveFiveHourQuotaStatusByProviderAt(platform string, providerID string, providerName string, now time.Time) (FiveHourQuotaStatus, error) {
	status := FiveHourQuotaStatus{}

	ref := providerRefFromStringID(providerID, providerName)
	if ref == "" {
		return status, nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return status, err
	}

	state, err := queryFiveHourQuotaCycleStateByProvider(db, platform, ref)
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
