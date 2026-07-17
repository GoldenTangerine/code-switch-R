/**
 * @name: 会话用量去重
 * @Descripttion: 增量建立代理日志与会话日志的一对一持久化匹配关系。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 17:01:12
 * @LastEditTime: 2026-07-17 17:01:12
 * @FilePath: services/session_usage_dedup.go
 */

package services

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/daodao97/xgo/xdb"
)

type requestLogDedupCandidate struct {
	ID                int64
	DataSource        string
	DedupCore         string
	Model             string
	CacheCreateTokens int
	CreatedAt         string
}

type sessionUsageDedupPair struct {
	SessionLogID int64
	ProxyLogID   int64
}

func reconcileSessionUsageDedup() (int, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return 0, err
	}
	if GlobalDBQueue == nil {
		return 0, fmt.Errorf("database write queue is not initialized")
	}

	lastProxyID, lastSessionID, err := loadSessionUsageDedupWatermark(db)
	if err != nil {
		return 0, err
	}
	newSessions, maxSessionID, err := loadNewDedupCandidates(db, lastSessionID, true)
	if err != nil {
		return 0, err
	}
	newProxies, maxProxyID, err := loadNewDedupCandidates(db, lastProxyID, false)
	if err != nil {
		return 0, err
	}

	usedSessions := make(map[int64]struct{}, len(newSessions))
	usedProxies := make(map[int64]struct{}, len(newProxies))
	pairs := make([]sessionUsageDedupPair, 0, min(len(newSessions)+len(newProxies), 256))

	for _, sessionLog := range newSessions {
		proxy, found, err := findDedupMatch(db, sessionLog, false, usedProxies)
		if err != nil {
			return 0, err
		}
		if !found {
			continue
		}
		usedSessions[sessionLog.ID] = struct{}{}
		usedProxies[proxy.ID] = struct{}{}
		pairs = append(pairs, sessionUsageDedupPair{SessionLogID: sessionLog.ID, ProxyLogID: proxy.ID})
	}

	for _, proxyLog := range newProxies {
		if _, used := usedProxies[proxyLog.ID]; used {
			continue
		}
		sessionLog, found, err := findDedupMatch(db, proxyLog, true, usedSessions)
		if err != nil {
			return 0, err
		}
		if !found {
			continue
		}
		usedSessions[sessionLog.ID] = struct{}{}
		usedProxies[proxyLog.ID] = struct{}{}
		pairs = append(pairs, sessionUsageDedupPair{SessionLogID: sessionLog.ID, ProxyLogID: proxyLog.ID})
	}

	created, err := persistSessionUsageDedupPairs(db, pairs)
	if err != nil {
		return 0, err
	}
	if maxProxyID < lastProxyID {
		maxProxyID = lastProxyID
	}
	if maxSessionID < lastSessionID {
		maxSessionID = lastSessionID
	}
	if err := GlobalDBQueue.Exec(`
		UPDATE session_usage_dedup_state
		SET last_proxy_log_id = ?, last_session_log_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, maxProxyID, maxSessionID); err != nil {
		return created, err
	}
	return created, nil
}

func loadSessionUsageDedupWatermark(db *sql.DB) (int64, int64, error) {
	var proxyID int64
	var sessionID int64
	err := db.QueryRow(`
		SELECT last_proxy_log_id, last_session_log_id
		FROM session_usage_dedup_state WHERE id = 1
	`).Scan(&proxyID, &sessionID)
	return proxyID, sessionID, err
}

func loadNewDedupCandidates(db *sql.DB, afterID int64, sessions bool) ([]requestLogDedupCandidate, int64, error) {
	sourceCondition := requestLogSourceWhereClause(LogDataSourceModeProxy, "request_log")
	if sessions {
		sourceCondition = requestLogSourceWhereClause(LogDataSourceModeSession, "request_log")
	}
	rows, err := db.Query(`
		SELECT id,
			COALESCE(data_source, 'proxy'),
			COALESCE(dedup_core, ''),
			COALESCE(model, ''),
			COALESCE(cache_create_tokens, 0),
			COALESCE(created_at, '')
		FROM request_log
		WHERE id > ? AND `+sourceCondition+`
		ORDER BY id ASC
	`, afterID)
	if err != nil {
		return nil, afterID, err
	}
	defer rows.Close()

	items := make([]requestLogDedupCandidate, 0, 128)
	maxID := afterID
	for rows.Next() {
		var item requestLogDedupCandidate
		if err := rows.Scan(&item.ID, &item.DataSource, &item.DedupCore, &item.Model, &item.CacheCreateTokens, &item.CreatedAt); err != nil {
			return nil, maxID, err
		}
		if item.ID > maxID {
			maxID = item.ID
		}
		items = append(items, item)
	}
	return items, maxID, rows.Err()
}

func findDedupMatch(
	db *sql.DB,
	input requestLogDedupCandidate,
	wantSession bool,
	reserved map[int64]struct{},
) (requestLogDedupCandidate, bool, error) {
	if strings.TrimSpace(input.DedupCore) == "" || strings.TrimSpace(input.CreatedAt) == "" {
		return requestLogDedupCandidate{}, false, nil
	}
	sourceCondition := requestLogSourceWhereClause(LogDataSourceModeProxy, "candidate")
	joinColumn := "proxy_log_id"
	if wantSession {
		sourceCondition = requestLogSourceWhereClause(LogDataSourceModeSession, "candidate")
		joinColumn = "session_log_id"
	}
	statusCondition := ""
	if !wantSession {
		statusCondition = " AND candidate.http_code >= 200 AND candidate.http_code < 300"
	}
	rows, err := db.Query(`
		SELECT candidate.id,
			COALESCE(candidate.data_source, 'proxy'),
			COALESCE(candidate.dedup_core, ''),
			COALESCE(candidate.model, ''),
			COALESCE(candidate.cache_create_tokens, 0),
			COALESCE(candidate.created_at, '')
		FROM request_log candidate
		LEFT JOIN session_usage_dedup matched ON matched.`+joinColumn+` = candidate.id
		WHERE matched.`+joinColumn+` IS NULL
		  AND `+sourceCondition+`
		  AND candidate.dedup_core = ?
		  AND candidate.created_at >= datetime(?, '-`+fmt.Sprint(sessionUsageDedupWindowSeconds)+` seconds')
		  AND candidate.created_at <= datetime(?, '+`+fmt.Sprint(sessionUsageDedupWindowSeconds)+` seconds')
		  `+statusCondition+`
		ORDER BY ABS(strftime('%s', candidate.created_at) - strftime('%s', ?)), candidate.id
		LIMIT 64
	`, input.DedupCore, input.CreatedAt, input.CreatedAt, input.CreatedAt)
	if err != nil {
		return requestLogDedupCandidate{}, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var candidate requestLogDedupCandidate
		if err := rows.Scan(&candidate.ID, &candidate.DataSource, &candidate.DedupCore, &candidate.Model, &candidate.CacheCreateTokens, &candidate.CreatedAt); err != nil {
			return requestLogDedupCandidate{}, false, err
		}
		if _, used := reserved[candidate.ID]; used {
			continue
		}
		sessionLog := input
		proxyLog := candidate
		if wantSession {
			sessionLog = candidate
			proxyLog = input
		}
		if !dedupModelsCompatible(sessionLog.Model, proxyLog.Model) {
			continue
		}
		if sessionLog.DataSource == requestLogDataSourceClaudeSession && sessionLog.CacheCreateTokens != proxyLog.CacheCreateTokens {
			continue
		}
		return candidate, true, nil
	}
	return requestLogDedupCandidate{}, false, rows.Err()
}

func dedupModelsCompatible(left string, right string) bool {
	left = normalizeSessionModel(left)
	right = normalizeSessionModel(right)
	return left == right || left == "" || right == "" || left == "unknown" || right == "unknown"
}

func persistSessionUsageDedupPairs(db *sql.DB, pairs []sessionUsageDedupPair) (int, error) {
	if len(pairs) == 0 {
		return 0, nil
	}
	var before int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_usage_dedup").Scan(&before); err != nil {
		return 0, err
	}
	for start := 0; start < len(pairs); start += 200 {
		end := min(start+200, len(pairs))
		groups := make([]string, 0, end-start)
		args := make([]any, 0, (end-start)*2)
		for _, pair := range pairs[start:end] {
			groups = append(groups, "(?, ?)")
			args = append(args, pair.SessionLogID, pair.ProxyLogID)
		}
		if err := GlobalDBQueue.Exec(
			"INSERT OR IGNORE INTO session_usage_dedup (session_log_id, proxy_log_id) VALUES "+strings.Join(groups, ","),
			args...,
		); err != nil {
			return 0, err
		}
	}
	var after int
	if err := db.QueryRow("SELECT COUNT(*) FROM session_usage_dedup").Scan(&after); err != nil {
		return 0, err
	}
	return max(after-before, 0), nil
}

func resetSessionUsageSyncState() error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("database write queue is not initialized")
	}
	if err := GlobalDBQueue.Exec("DELETE FROM session_log_sync"); err != nil {
		return err
	}
	if err := GlobalDBQueue.Exec("DELETE FROM session_usage_dedup"); err != nil {
		return err
	}
	return GlobalDBQueue.Exec(`
		UPDATE session_usage_dedup_state
		SET last_proxy_log_id = 0, last_session_log_id = 0, updated_at = ?
		WHERE id = 1
	`, time.Now().UTC().Format(timeLayout))
}
