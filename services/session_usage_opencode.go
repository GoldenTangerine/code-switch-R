/**
 * @name: OpenCode 会话用量同步
 * @Descripttion: 只读解析 ~/.local/share/opencode/opencode.db 的 session/message 表，增量汇入 request_log
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 05:00:00
 * @LastEditTime: 2026-08-17 05:00:00
 * @FilePath: services/session_usage_opencode.go
 */

package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// opencodeDBFileName OpenCode 用量库文件名（位于 ~/.local/share/opencode/）
const opencodeDBFileName = "opencode.db"

// syncOpenCodeSessionUsage OpenCode 会话用量同步入口
// 与 JSONL 解析器不同，OpenCode 的用量存于自身 SQLite 库：
//   - 文件级门控：opencode.db 与 opencode.db-wal 的最大 mtime + 合并尺寸作为指纹，未变则整库跳过
//   - 会话级游标：cursor 键「db路径:sessionID」，waterline（MAX(time_updated)）存于 ParserState
//   - 消息级去重：source_record_id = opencode_session:{sessionID}:{messageID}
func syncOpenCodeSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "opencode"}

	dbPath := filepath.Join(root, opencodeDBFileName)
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 未安装或未使用 OpenCode：视为空源
			return result, nil
		}
		return result, err
	}

	// 文件级指纹：主库 + WAL 的最大 mtime 与合并尺寸
	fingerprintNS := fileModifiedNS(info)
	fingerprintSize := info.Size()
	if walInfo, walErr := os.Stat(dbPath + "-wal"); walErr == nil {
		if walNS := fileModifiedNS(walInfo); walNS > fingerprintNS {
			fingerprintNS = walNS
		}
		fingerprintSize += walInfo.Size()
	}

	fileCursor, unchanged, err := prepareSessionUsageDBCursor(scope, "opencode", dbPath, fingerprintNS, fingerprintSize)
	if err != nil {
		return result, err
	}
	if unchanged {
		result.Skipped++
		return result, nil
	}
	result.FilesScanned++

	db, err := openOpenCodeUsageDBReadOnly(dbPath)
	if err != nil {
		return result, err
	}
	defer db.Close()

	// 会话与 waterline（复用 cc-switch 验证过的聚合口径）
	sessionRows, err := db.Query(`
		SELECT s.id, MAX(s.time_updated, COALESCE(MAX(m.time_updated), s.time_updated)) AS sync_watermark
		FROM session s
		LEFT JOIN message m ON m.session_id = s.id
		GROUP BY s.id
	`)
	if err != nil {
		return result, fmt.Errorf("查询 OpenCode 会话失败: %w", err)
	}

	type openCodeSessionRow struct {
		ID        string
		Waterline string
	}
	sessions := make([]openCodeSessionRow, 0, 16)
	for sessionRows.Next() {
		var row openCodeSessionRow
		var watermark sql.NullString
		if err := sessionRows.Scan(&row.ID, &watermark); err != nil {
			_ = sessionRows.Close()
			return result, err
		}
		if watermark.Valid {
			row.Waterline = watermark.String
		}
		sessions = append(sessions, row)
	}
	if err := sessionRows.Err(); err != nil {
		_ = sessionRows.Close()
		return result, err
	}
	_ = sessionRows.Close()

	var sessionErrors []string
	for _, session := range sessions {
		if session.ID == "" {
			continue
		}
		imported, skipped, err := syncOpenCodeSessionMessages(ls, scope, dbPath, db, session.ID, session.Waterline)
		if err != nil {
			// 单会话失败不中断其他会话，聚合到最后统一上报
			sessionErrors = append(sessionErrors, fmt.Sprintf("session %s: %v", session.ID, err))
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	if len(sessionErrors) > 0 {
		result.Error = strings.Join(sessionErrors, "; ")
	}

	// 推进文件级指纹
	fileCursor.ModifiedNS = fingerprintNS
	fileCursor.FileSize = fingerprintSize
	if err := saveSessionFileCursor(fileCursor); err != nil {
		return result, err
	}
	return result, nil
}

// prepareSessionUsageDBCursor 数据库级游标（与 prepareSessionFileCursor 同构，指纹由调用方合成）
func prepareSessionUsageDBCursor(scope string, platform string, dbPath string, fingerprintNS int64, fingerprintSize int64) (sessionFileCursor, bool, error) {
	cursor, err := loadSessionFileCursor(scope, platform, dbPath)
	if err != nil {
		return sessionFileCursor{}, false, err
	}
	if cursor.ModifiedNS == fingerprintNS && cursor.FileSize == fingerprintSize {
		return cursor, true, nil
	}
	cursor.ModifiedNS = fingerprintNS
	cursor.FileSize = fingerprintSize
	cursor.ByteOffset = 0
	cursor.LineOffset = 0
	return cursor, false, nil
}

// openOpenCodeUsageDBReadOnly 以只读方式打开 OpenCode 用量库（独立连接，不影响主库连接池）
func openOpenCodeUsageDBReadOnly(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("打开 OpenCode 用量库失败: %w", err)
	}
	return db, nil
}

// syncOpenCodeSessionMessages 同步单个会话的消息用量（waterline 变化才解析）
func syncOpenCodeSessionMessages(ls *LogService, scope string, dbPath string, db *sql.DB, sessionID string, waterline string) (int, int, error) {
	cursorKey := dbPath + ":" + sessionID
	cursor, err := loadSessionFileCursor(scope, "opencode", cursorKey)
	if err != nil {
		return 0, 0, err
	}
	if cursor.ParserState != "" && cursor.ParserState == waterline {
		return 0, 1, nil
	}

	messageRows, err := db.Query(`
		SELECT id, data FROM message
		WHERE session_id = ?
		ORDER BY time_created
	`, sessionID)
	if err != nil {
		return 0, 0, err
	}

	records := make([]sessionUsageRecord, 0, 16)
	for messageRows.Next() {
		var messageID string
		var data string
		if err := messageRows.Scan(&messageID, &data); err != nil {
			_ = messageRows.Close()
			return 0, 0, err
		}
		record, ok := buildOpenCodeUsageRecord(sessionID, messageID, data)
		if !ok {
			continue
		}
		records = append(records, record)
	}
	if err := messageRows.Err(); err != nil {
		_ = messageRows.Close()
		return 0, 0, err
	}
	_ = messageRows.Close()

	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}

	cursor.ParserState = waterline
	cursor.ByteOffset = 0
	cursor.LineOffset = 0
	if err := saveSessionFileCursor(cursor); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

// buildOpenCodeUsageRecord 从 message.data JSON 提取用量（仅 assistant 消息）
// 字段：modelID / tokens.input / tokens.output / tokens.reasoning / tokens.cache.read / tokens.cache.write / time.created
func buildOpenCodeUsageRecord(sessionID string, messageID string, data string) (sessionUsageRecord, bool) {
	parsed := gjson.Parse(data)
	if parsed.Get("role").String() != "assistant" {
		return sessionUsageRecord{}, false
	}

	record := sessionUsageRecord{
		Platform:       "opencode",
		Model:          parsed.Get("modelID").String(),
		ProviderID:     requestLogProviderIDOpenCodeSession,
		Provider:       requestLogProviderOpenCodeSession,
		DataSource:     requestLogDataSourceOpenCodeSession,
		SourceRecordID: "opencode_session:" + sessionID + ":" + messageID,
		SessionID:      sessionID,
	}

	tokens := parsed.Get("tokens")
	record.InputTokens = int(tokens.Get("input").Int())
	record.OutputTokens = int(tokens.Get("output").Int())
	record.ReasoningTokens = int(tokens.Get("reasoning").Int())
	record.CacheReadTokens = int(tokens.Get("cache.read").Int())
	record.CacheCreateTokens = int(tokens.Get("cache.write").Int())

	if created := parsed.Get("time.created").Int(); created > 0 {
		record.CreatedAt = time.UnixMilli(created)
	}

	if record.InputTokens == 0 && record.OutputTokens == 0 && record.ReasoningTokens == 0 {
		return sessionUsageRecord{}, false
	}
	return record, true
}
