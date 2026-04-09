package services

import (
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// CostSinceByProvider 查询指定供应商从 start 时间起的累计费用
// providerID 和 providerName 使用 OR 匹配，兼容重命名场景
func (ls *LogService) CostSinceByProvider(start string, platform string, providerID string, providerName string) (float64, error) {
	startTime, err := parseTimeInput(start)
	if err != nil {
		return 0, err
	}

	db, err := xdb.DB("default")
	if err != nil {
		return 0, err
	}

	ref := providerRefFromStringID(providerID, providerName)
	if ref == "" {
		return 0, nil
	}

	query := `SELECT COALESCE(SUM(total_cost), 0) FROM request_log WHERE created_at >= ?`
	args := []interface{}{startTime.UTC().Format(timeLayout)}

	if strings.TrimSpace(platform) != "" {
		query += ` AND platform = ?`
		args = append(args, platform)
	}

	query += ` AND (TRIM(COALESCE(provider_id,'')) = ? OR TRIM(COALESCE(provider,'')) = ?)`
	args = append(args, ref, ref)

	total := 0.0
	if err := db.QueryRow(query, args...).Scan(&total); err != nil {
		if isNoSuchTableErr(err) {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
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
