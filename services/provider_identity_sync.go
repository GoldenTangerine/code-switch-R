package services

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/daodao97/xgo/xdb"
)

type providerIdentitySyncExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

func normalizeProviderIdentityRenameInput(kind, providerID, oldName, newName string) (string, string, string, string) {
	kind = strings.TrimSpace(kind)
	providerID = strings.TrimSpace(providerID)
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if newName == "" {
		newName = oldName
	}
	if oldName == "" {
		oldName = newName
	}
	return kind, providerID, oldName, newName
}

// syncProviderIdentityRename 在 provider 改名后同步关联存储，避免黑名单/统计/日志断档。
// providerID 统一使用字符串，兼容 int64（claude/codex/custom）和 string（gemini）。
func syncProviderIdentityRename(kind, providerID, oldName, newName string) error {
	kind, providerID, oldName, newName = normalizeProviderIdentityRenameInput(kind, providerID, oldName, newName)
	if kind == "" || newName == "" {
		return nil
	}

	db, err := xdb.DB("default")
	if err != nil {
		return err
	}
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

	if err := syncProviderIdentityRenameTx(tx, kind, providerID, oldName, newName); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func syncProviderIdentityRenameTx(tx *sql.Tx, kind, providerID, oldName, newName string) error {
	if tx == nil {
		return fmt.Errorf("nil tx")
	}
	return syncProviderIdentityRenameWithExec(tx, kind, providerID, oldName, newName)
}

func syncProviderIdentityRenameWithExec(exec providerIdentitySyncExecer, kind, providerID, oldName, newName string) error {
	if exec == nil {
		return nil
	}
	kind, providerID, oldName, newName = normalizeProviderIdentityRenameInput(kind, providerID, oldName, newName)
	if kind == "" || newName == "" {
		return nil
	}

	if err := syncBlacklistIdentityWithExec(exec, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}
	if err := syncRequestLogIdentityWithExec(exec, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}
	if err := syncHealthCheckIdentityWithExec(exec, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}
	if err := syncRequestLogStatsIdentityWithExec(exec, requestLogStatsHourlyTable, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}
	if err := syncRequestLogStatsIdentityWithExec(exec, requestLogStatsDailyTable, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}
	if err := syncRequestLogProviderQuotaCycleIdentityWithExec(exec, kind, providerID, oldName, newName); err != nil && !isNoSuchTableErr(err) {
		return err
	}

	return nil
}

func syncBlacklistIdentityWithExec(exec providerIdentitySyncExecer, kind, providerID, oldName, newName string) error {
	if exec == nil {
		return nil
	}
	if providerID != "" {
		if _, err := exec.Exec(`
			UPDATE provider_blacklist
			SET provider_name = ?, provider_id = ?
			WHERE platform = ? AND provider_id = ?
		`, newName, providerID, kind, providerID); err != nil {
			return err
		}
		legacyName := oldName
		if legacyName == "" {
			legacyName = newName
		}
		if legacyName != "" {
			if _, err := exec.Exec(`
				UPDATE provider_blacklist
				SET provider_id = ?, provider_name = ?
				WHERE platform = ?
				  AND provider_name = ?
				  AND (provider_id = '' OR provider_id IS NULL)
			`, providerID, newName, kind, legacyName); err != nil {
				return err
			}
		}
		return nil
	}

	if oldName != "" && oldName != newName {
		if _, err := exec.Exec(`
			UPDATE provider_blacklist
			SET provider_name = ?
			WHERE platform = ? AND provider_name = ?
		`, newName, kind, oldName); err != nil {
			return err
		}
	}
	return nil
}

func syncRequestLogIdentityWithExec(exec providerIdentitySyncExecer, kind, providerID, oldName, newName string) error {
	if exec == nil {
		return nil
	}
	if providerID != "" {
		if _, err := exec.Exec(`
			UPDATE request_log
			SET provider_id = ?, provider = ?
			WHERE platform = ? AND provider_id = ?
		`, providerID, newName, kind, providerID); err != nil {
			return err
		}
		legacyName := oldName
		if legacyName == "" {
			legacyName = newName
		}
		if legacyName != "" {
			if _, err := exec.Exec(`
				UPDATE request_log
				SET provider_id = ?, provider = ?
				WHERE platform = ?
				  AND provider = ?
				  AND (provider_id = '' OR provider_id IS NULL)
			`, providerID, newName, kind, legacyName); err != nil {
				return err
			}
		}
		return nil
	}
	if oldName != "" && oldName != newName {
		_, err := exec.Exec(`
			UPDATE request_log
			SET provider = ?
			WHERE platform = ? AND provider = ?
		`, newName, kind, oldName)
		return err
	}
	return nil
}

func syncHealthCheckIdentityWithExec(exec providerIdentitySyncExecer, kind, providerID, oldName, newName string) error {
	if exec == nil {
		return nil
	}
	if providerID != "" {
		numericID, parseErr := strconv.ParseInt(providerID, 10, 64)
		if parseErr == nil {
			if _, err := exec.Exec(`
				UPDATE health_check_history
				SET provider_name = ?
				WHERE platform = ? AND provider_id = ?
			`, newName, kind, numericID); err != nil {
				return err
			}
			legacyName := oldName
			if legacyName == "" {
				legacyName = newName
			}
			if legacyName != "" {
				if _, err := exec.Exec(`
					UPDATE health_check_history
					SET provider_name = ?
					WHERE platform = ?
					  AND provider_name = ?
					  AND provider_id = 0
				`, newName, kind, legacyName); err != nil {
					return err
				}
			}
			return nil
		}
	}

	if oldName != "" && oldName != newName {
		if _, err := exec.Exec(`
			UPDATE health_check_history
			SET provider_name = ?
			WHERE platform = ? AND provider_name = ?
		`, newName, kind, oldName); err != nil {
			return err
		}
	}
	return nil
}

func syncRequestLogStatsIdentityWithExec(exec providerIdentitySyncExecer, table, kind, providerID, oldName, newName string) error {
	if exec == nil || strings.TrimSpace(table) == "" {
		return nil
	}
	if table != requestLogStatsHourlyTable && table != requestLogStatsDailyTable {
		return fmt.Errorf("invalid stats table: %s", table)
	}
	if newName == "" {
		return nil
	}

	if providerID != "" {
		legacyName := oldName
		if legacyName == "" {
			legacyName = newName
		}
		if legacyName != "" {
			upsertSQL := fmt.Sprintf(`
				INSERT INTO %s (
					bucket_start, platform, provider_id, provider,
					total_requests, successful_requests, failed_requests,
					input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
					total_cost
				)
				SELECT
					bucket_start, platform, ?, ?,
					total_requests, successful_requests, failed_requests,
					input_tokens, output_tokens, reasoning_tokens, cache_create_tokens, cache_read_tokens,
					total_cost
				FROM %s
				WHERE platform = ? AND provider = ? AND (provider_id = '' OR provider_id IS NULL)
				ON CONFLICT(bucket_start, platform, provider_id) DO UPDATE SET
					provider = excluded.provider,
					total_requests = total_requests + excluded.total_requests,
					successful_requests = successful_requests + excluded.successful_requests,
					failed_requests = failed_requests + excluded.failed_requests,
					input_tokens = input_tokens + excluded.input_tokens,
					output_tokens = output_tokens + excluded.output_tokens,
					reasoning_tokens = reasoning_tokens + excluded.reasoning_tokens,
					cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
					cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
					total_cost = total_cost + excluded.total_cost
			`, table, table)
			if _, err := exec.Exec(upsertSQL, providerID, newName, kind, legacyName); err != nil {
				return err
			}

			deleteSQL := fmt.Sprintf(`
				DELETE FROM %s
				WHERE platform = ? AND provider = ? AND (provider_id = '' OR provider_id IS NULL)
			`, table)
			if _, err := exec.Exec(deleteSQL, kind, legacyName); err != nil {
				return err
			}
		}

		updateSQL := fmt.Sprintf(`
			UPDATE %s
			SET provider = ?
			WHERE platform = ? AND provider_id = ?
		`, table)
		if _, err := exec.Exec(updateSQL, newName, kind, providerID); err != nil {
			return err
		}
		return nil
	}

	if oldName != "" && oldName != newName {
		updateSQL := fmt.Sprintf(`
			UPDATE %s
			SET provider = ?
			WHERE platform = ? AND provider = ?
		`, table)
		if _, err := exec.Exec(updateSQL, newName, kind, oldName); err != nil {
			return err
		}
	}
	return nil
}

func syncRequestLogProviderQuotaCycleIdentityWithExec(exec providerIdentitySyncExecer, kind, providerID, oldName, newName string) error {
	if exec == nil {
		return nil
	}

	kind = strings.TrimSpace(kind)
	providerID = strings.TrimSpace(providerID)
	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if kind == "" || newName == "" {
		return nil
	}

	targetRef := providerRefFromStringID(providerID, newName)
	legacyRef := strings.TrimSpace(oldName)
	if legacyRef == "" {
		legacyRef = newName
	}

	state, err := buildFiveHourQuotaCycleStateFromHistoryByProvider(exec, kind, targetRef)
	if err != nil {
		return err
	}
	if state.WindowStart.IsZero() || state.NextReset.IsZero() {
		if err := deleteFiveHourQuotaCycleStateByProviderWithExec(exec, kind, targetRef); err != nil {
			return err
		}
	} else if err := upsertFiveHourQuotaCycleStateByProviderWithExec(exec, state); err != nil {
		return err
	}

	if legacyRef != "" && legacyRef != targetRef {
		if err := deleteFiveHourQuotaCycleStateByProviderWithExec(exec, kind, legacyRef); err != nil {
			return err
		}
	}

	if providerID != "" && newName != "" && newName != targetRef && newName != legacyRef {
		if err := deleteFiveHourQuotaCycleStateByProviderWithExec(exec, kind, newName); err != nil {
			return err
		}
	}

	return nil
}
