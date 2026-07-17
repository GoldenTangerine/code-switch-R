/**
 * @name: 会话用量同步
 * @Descripttion: 增量解析 Claude、Codex 与 Gemini 本地会话并写入日志统计。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-17 17:01:12
 * @LastEditTime: 2026-07-17 17:01:12
 * @FilePath: services/session_usage.go
 */

package services

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/daodao97/xgo/xdb"
)

type SessionSyncSourceResult struct {
	Scope        string `json:"scope"`
	Platform     string `json:"platform"`
	Imported     int    `json:"imported"`
	Skipped      int    `json:"skipped"`
	FilesScanned int    `json:"files_scanned"`
	Error        string `json:"error,omitempty"`
}

type SessionSyncResult struct {
	Imported       int                       `json:"imported"`
	Skipped        int                       `json:"skipped"`
	FilesScanned   int                       `json:"files_scanned"`
	MatchesCreated int                       `json:"matches_created"`
	Sources        []SessionSyncSourceResult `json:"sources"`
	Errors         []string                  `json:"errors"`
}

type sessionFileCursor struct {
	SyncKey     string
	SourceScope string
	Platform    string
	FilePath    string
	ModifiedNS  int64
	FileSize    int64
	ByteOffset  int64
	LineOffset  int64
	ParserState string
}

type sessionUsageRecord struct {
	Platform          string
	Model             string
	ProviderID        string
	Provider          string
	DataSource        string
	SourceRecordID    string
	SessionID         string
	InputTokens       int
	OutputTokens      int
	CacheCreateTokens int
	CacheReadTokens   int
	ReasoningTokens   int
	CreatedAt         time.Time
}

type existingSessionUsageRecord struct {
	ID                int64
	Platform          string
	Model             string
	ProviderID        string
	Provider          string
	DataSource        string
	SessionID         string
	InputTokens       int
	OutputTokens      int
	CacheCreateTokens int
	CacheReadTokens   int
	ReasoningTokens   int
	CreatedAt         string
}

type sessionUsageRecordUpdate struct {
	ID  int64
	Log ReqeustLog
}

type claudeSessionUsageCandidate struct {
	Record        sessionUsageRecord
	HasStopReason bool
}

type codexCumulativeTokens struct {
	Input       uint64 `json:"input"`
	CachedInput uint64 `json:"cached_input"`
	Output      uint64 `json:"output"`
}

type codexParserState struct {
	ThreadID       string                 `json:"thread_id"`
	CurrentModel   string                 `json:"current_model"`
	PreviousTotal  *codexCumulativeTokens `json:"previous_total,omitempty"`
	EventIndex     int                    `json:"event_index"`
	ReplayActive   bool                   `json:"replay_active"`
	IdentityLoaded bool                   `json:"identity_loaded"`
}

var sessionUsageSyncMu sync.Mutex

func (ls *LogService) SyncLocalSessionUsage() (SessionSyncResult, error) {
	home, err := getUserHomeDir()
	if err != nil {
		return SessionSyncResult{}, err
	}
	return ls.syncSessionUsageRoots("native", home)
}

func (ls *LogService) SyncWSLSessionUsage() (SessionSyncResult, error) {
	if runtime.GOOS != "windows" {
		return SessionSyncResult{}, fmt.Errorf("WSL session scanning is only available on Windows")
	}

	detection := (&NetworkService{}).DetectWSL()
	if !detection.Detected {
		return SessionSyncResult{}, fmt.Errorf("WSL not detected")
	}

	sessionUsageSyncMu.Lock()
	defer sessionUsageSyncMu.Unlock()

	result := SessionSyncResult{Sources: []SessionSyncSourceResult{}, Errors: []string{}}
	for _, distro := range detection.Distros {
		home, err := resolveWSLHomePath(distro)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", distro, err))
			continue
		}
		scope := "wsl:" + strings.TrimSpace(distro)
		partial := ls.syncSessionUsageRootsLocked(scope, home)
		mergeSessionSyncResult(&result, partial)
	}
	matches, err := reconcileSessionUsageDedup()
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.MatchesCreated += matches
	}
	return result, nil
}

func (ls *LogService) syncSessionUsageRoots(scope string, home string) (SessionSyncResult, error) {
	sessionUsageSyncMu.Lock()
	defer sessionUsageSyncMu.Unlock()

	result := ls.syncSessionUsageRootsLocked(scope, home)
	matches, err := reconcileSessionUsageDedup()
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	} else {
		result.MatchesCreated += matches
	}
	return result, nil
}

func (ls *LogService) syncSessionUsageRootsLocked(scope string, home string) SessionSyncResult {
	result := SessionSyncResult{Sources: []SessionSyncSourceResult{}, Errors: []string{}}
	jobs := []struct {
		platform string
		root     string
		sync     func(*LogService, string, string) (SessionSyncSourceResult, error)
	}{
		{platform: "claude", root: filepath.Join(home, ".claude"), sync: syncClaudeSessionUsage},
		{platform: "codex", root: filepath.Join(home, ".codex"), sync: syncCodexSessionUsage},
		{platform: "gemini", root: filepath.Join(home, ".gemini"), sync: syncGeminiSessionUsage},
	}

	for _, job := range jobs {
		sourceResult, err := job.sync(ls, scope, job.root)
		if err != nil {
			sourceResult = SessionSyncSourceResult{Scope: scope, Platform: job.platform, Error: err.Error()}
			result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", scope, job.platform, err))
		}
		result.Sources = append(result.Sources, sourceResult)
		result.Imported += sourceResult.Imported
		result.Skipped += sourceResult.Skipped
		result.FilesScanned += sourceResult.FilesScanned
	}
	return result
}

func mergeSessionSyncResult(target *SessionSyncResult, source SessionSyncResult) {
	if target == nil {
		return
	}
	target.Imported += source.Imported
	target.Skipped += source.Skipped
	target.FilesScanned += source.FilesScanned
	target.MatchesCreated += source.MatchesCreated
	target.Sources = append(target.Sources, source.Sources...)
	target.Errors = append(target.Errors, source.Errors...)
}

func resolveWSLHomePath(distro string) (string, error) {
	command := hideWindowCmd("wsl", "-d", distro, "bash", "-lc", `printf '%s' "$HOME"`)
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("resolve home: %w", err)
	}
	linuxHome := strings.TrimSpace(string(output))
	if !strings.HasPrefix(linuxHome, "/") || strings.Contains(linuxHome, "..") {
		return "", fmt.Errorf("invalid WSL home path")
	}
	parts := strings.Split(strings.TrimPrefix(linuxHome, "/"), "/")
	pathParts := []string{`\\wsl.localhost\` + strings.TrimSpace(distro)}
	pathParts = append(pathParts, parts...)
	return filepath.Join(pathParts...), nil
}

func syncClaudeSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "claude"}
	files := collectSessionFiles(filepath.Join(root, "projects"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".jsonl")
	})
	result.FilesScanned = len(files)
	for _, path := range files {
		imported, skipped, err := syncClaudeSessionFile(ls, scope, path)
		if err != nil {
			return result, err
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, nil
}

func syncCodexSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "codex"}
	files := collectSessionFiles(filepath.Join(root, "sessions"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".jsonl")
	})
	files = append(files, collectSessionFiles(filepath.Join(root, "archived_sessions"), func(path string) bool {
		return strings.EqualFold(filepath.Ext(path), ".jsonl")
	})...)
	sort.Strings(files)
	result.FilesScanned = len(files)
	for _, path := range files {
		imported, skipped, err := syncCodexSessionFile(ls, scope, path)
		if err != nil {
			return result, err
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, nil
}

func syncGeminiSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "gemini"}
	files := collectSessionFiles(filepath.Join(root, "tmp"), func(path string) bool {
		name := filepath.Base(path)
		return strings.HasPrefix(name, "session-") && strings.EqualFold(filepath.Ext(name), ".json") && filepath.Base(filepath.Dir(path)) == "chats"
	})
	result.FilesScanned = len(files)
	for _, path := range files {
		imported, skipped, err := syncGeminiSessionFile(ls, scope, path)
		if err != nil {
			return result, err
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, nil
}

func collectSessionFiles(root string, accept func(string) bool) []string {
	if strings.TrimSpace(root) == "" {
		return nil
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}

	files := make([]string, 0, 32)
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() && accept(path) {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func syncClaudeSessionFile(ls *LogService, scope string, path string) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, "claude", path)
	if err != nil || unchanged {
		return 0, 0, err
	}

	candidates := make(map[string]claudeSessionUsageCandidate, 8)
	messageOrder := make([]string, 0, 8)
	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		var value map[string]any
		if json.Unmarshal(line, &value) != nil || sessionStringValue(value["type"]) != "assistant" {
			return nil
		}
		message := mapValue(value["message"])
		usage := mapValue(message["usage"])
		if len(usage) == 0 {
			return nil
		}
		input := intValue(usage["input_tokens"])
		output := intValue(usage["output_tokens"])
		cacheRead := intValue(usage["cache_read_input_tokens"])
		cacheCreate := intValue(usage["cache_creation_input_tokens"])
		if input+output+cacheRead+cacheCreate <= 0 {
			return nil
		}
		sessionID := firstString(value, "sessionId", "session_id")
		messageID := firstString(message, "id")
		if messageID == "" {
			return nil
		}
		candidate := claudeSessionUsageCandidate{
			HasStopReason: firstString(message, "stop_reason") != "",
			Record: sessionUsageRecord{
				Platform:          "claude",
				Model:             normalizeSessionModel(firstString(message, "model")),
				ProviderID:        requestLogProviderIDClaudeSession,
				Provider:          requestLogProviderClaudeSession,
				DataSource:        requestLogDataSourceClaudeSession,
				SourceRecordID:    stableSessionRecordID(scope, "claude", messageID),
				SessionID:         sessionID,
				InputTokens:       input,
				OutputTokens:      output,
				CacheCreateTokens: cacheCreate,
				CacheReadTokens:   cacheRead,
				CreatedAt:         parseSessionTimestamp(sessionStringValue(value["timestamp"])),
			},
		}
		existing, found := candidates[messageID]
		if !found {
			messageOrder = append(messageOrder, messageID)
		}
		if !found || (candidate.HasStopReason && !existing.HasStopReason) ||
			(candidate.HasStopReason == existing.HasStopReason && candidate.Record.OutputTokens > existing.Record.OutputTokens) {
			candidates[messageID] = candidate
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	records := make([]sessionUsageRecord, 0, len(messageOrder))
	for _, messageID := range messageOrder {
		records = append(records, candidates[messageID].Record)
	}
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

func syncCodexSessionFile(ls *LogService, scope string, path string) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, "codex", path)
	if err != nil || unchanged {
		return 0, 0, err
	}

	state := codexParserState{CurrentModel: "unknown"}
	if strings.TrimSpace(cursor.ParserState) != "" {
		_ = json.Unmarshal([]byte(cursor.ParserState), &state)
	}
	records := make([]sessionUsageRecord, 0, 8)
	next, err := readJSONLLines(cursor, func(line []byte, lineNumber int64) error {
		var value map[string]any
		if json.Unmarshal(line, &value) != nil {
			return nil
		}
		eventType := sessionStringValue(value["type"])
		payload := mapValue(value["payload"])
		switch eventType {
		case "session_meta":
			if !state.IdentityLoaded {
				state.ThreadID = firstString(payload, "id", "thread_id", "threadId", "session_id", "sessionId")
				parentSession := firstString(payload, "session_id", "sessionId")
				state.ReplayActive = firstString(payload, "forked_from_id") != "" || mapValue(payload["source"])["subagent"] != nil || (parentSession != "" && parentSession != state.ThreadID)
				state.IdentityLoaded = true
			}
		case "turn_context":
			model := firstString(payload, "model")
			if model == "" {
				model = firstString(mapValue(payload["info"]), "model")
			}
			if model != "" {
				state.CurrentModel = normalizeSessionModel(model)
			}
		case "event_msg":
			payloadType := sessionStringValue(payload["type"])
			if payloadType == "thread_settings_applied" {
				state.ReplayActive = false
				return nil
			}
			if payloadType != "token_count" {
				return nil
			}
			info := mapValue(payload["info"])
			if len(info) == 0 {
				return nil
			}
			if model := firstString(info, "model", "model_name"); model != "" {
				state.CurrentModel = normalizeSessionModel(model)
			}
			usageValue := mapValue(info["total_token_usage"])
			isTotal := len(usageValue) > 0
			if !isTotal {
				usageValue = mapValue(info["last_token_usage"])
			}
			if len(usageValue) == 0 {
				return nil
			}
			current := parseCodexCumulativeTokens(usageValue)
			delta := current
			if isTotal && state.PreviousTotal != nil {
				delta = codexCumulativeTokens{
					Input:       saturatingSub(current.Input, state.PreviousTotal.Input),
					CachedInput: saturatingSub(current.CachedInput, state.PreviousTotal.CachedInput),
					Output:      saturatingSub(current.Output, state.PreviousTotal.Output),
				}
			}
			if isTotal {
				state.PreviousTotal = &current
			}
			if delta.Input+delta.CachedInput+delta.Output == 0 {
				return nil
			}
			state.EventIndex++
			if state.ReplayActive {
				return nil
			}
			cached := delta.CachedInput
			if cached > delta.Input {
				cached = delta.Input
			}
			freshInput := delta.Input - cached
			stableKey := fmt.Sprintf("thread-v1:%s:%d", state.ThreadID, state.EventIndex)
			records = append(records, sessionUsageRecord{
				Platform:        "codex",
				Model:           state.CurrentModel,
				ProviderID:      requestLogProviderIDCodexSession,
				Provider:        requestLogProviderCodexSession,
				DataSource:      requestLogDataSourceCodexSession,
				SourceRecordID:  stableSessionRecordID(scope, "codex", stableKey),
				SessionID:       state.ThreadID,
				InputTokens:     int(freshInput),
				OutputTokens:    int(delta.Output),
				CacheReadTokens: int(cached),
				CreatedAt:       parseSessionTimestamp(sessionStringValue(value["timestamp"])),
			})
		default:
			if strings.HasPrefix(eventType, "inter_agent_communication") {
				state.ReplayActive = false
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	stateJSON, _ := json.Marshal(state)
	next.ParserState = string(stateJSON)
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

func syncGeminiSessionFile(ls *LogService, scope string, path string) (int, int, error) {
	cursor, unchanged, err := prepareSessionFileCursor(scope, "gemini", path)
	if err != nil || unchanged {
		return 0, 0, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return 0, 0, err
	}
	sessionID := firstString(root, "sessionId", "session_id")
	messages, _ := root["messages"].([]any)
	records := make([]sessionUsageRecord, 0, 8)
	messageIndex := int64(0)
	for _, raw := range messages {
		message := mapValue(raw)
		if sessionStringValue(message["type"]) != "gemini" {
			continue
		}
		messageIndex++
		tokens := mapValue(message["tokens"])
		inputTotal := intValue(tokens["input"])
		cacheRead := intValue(tokens["cached"])
		freshInput := max(inputTotal-cacheRead, 0)
		output := intValue(tokens["output"])
		reasoning := intValue(tokens["thoughts"])
		if freshInput+cacheRead+output+reasoning <= 0 {
			continue
		}
		messageID := firstString(message, "id")
		stableKey := strings.Join([]string{sessionID, messageID, strconv.FormatInt(messageIndex, 10)}, ":")
		records = append(records, sessionUsageRecord{
			Platform:        "gemini",
			Model:           normalizeSessionModel(firstString(message, "model")),
			ProviderID:      requestLogProviderIDGeminiSession,
			Provider:        requestLogProviderGeminiSession,
			DataSource:      requestLogDataSourceGeminiSession,
			SourceRecordID:  stableSessionRecordID(scope, "gemini", stableKey),
			SessionID:       sessionID,
			InputTokens:     freshInput,
			OutputTokens:    output,
			CacheReadTokens: cacheRead,
			ReasoningTokens: reasoning,
			CreatedAt:       parseSessionTimestamp(sessionStringValue(message["timestamp"])),
		})
	}
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	cursor.ModifiedNS = fileModifiedNS(mustFileInfo(path))
	cursor.FileSize = int64(len(data))
	cursor.ByteOffset = int64(len(data))
	cursor.LineOffset = messageIndex
	if err := saveSessionFileCursor(cursor); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

func prepareSessionFileCursor(scope string, platform string, path string) (sessionFileCursor, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return sessionFileCursor{}, false, err
	}
	cursor, err := loadSessionFileCursor(scope, platform, path)
	if err != nil {
		return sessionFileCursor{}, false, err
	}
	modifiedNS := fileModifiedNS(info)
	if cursor.ModifiedNS == modifiedNS && cursor.FileSize == info.Size() {
		return cursor, true, nil
	}
	if info.Size() < cursor.ByteOffset {
		cursor.ByteOffset = 0
		cursor.LineOffset = 0
		cursor.ParserState = ""
	}
	cursor.ModifiedNS = modifiedNS
	cursor.FileSize = info.Size()
	return cursor, false, nil
}

func loadSessionFileCursor(scope string, platform string, path string) (sessionFileCursor, error) {
	syncKey := buildSessionSyncKey(scope, platform, path)
	cursor := sessionFileCursor{SyncKey: syncKey, SourceScope: scope, Platform: platform, FilePath: path}
	db, err := xdb.DB("default")
	if err != nil {
		return cursor, err
	}
	err = db.QueryRow(`
		SELECT modified_ns, file_size, byte_offset, line_offset, parser_state
		FROM session_log_sync WHERE sync_key = ?
	`, syncKey).Scan(&cursor.ModifiedNS, &cursor.FileSize, &cursor.ByteOffset, &cursor.LineOffset, &cursor.ParserState)
	if errors.Is(err, sql.ErrNoRows) {
		return inheritArchivedCodexCursor(db, cursor)
	}
	return cursor, err
}

func inheritArchivedCodexCursor(db *sql.DB, cursor sessionFileCursor) (sessionFileCursor, error) {
	if cursor.Platform != "codex" || filepath.Base(filepath.Dir(cursor.FilePath)) != "archived_sessions" {
		return cursor, nil
	}
	fileName := filepath.Base(cursor.FilePath)
	slashSuffix := "/" + fileName
	backslashSuffix := "\\" + fileName
	err := db.QueryRow(`
		SELECT modified_ns, file_size, byte_offset, line_offset, parser_state
		FROM session_log_sync
		WHERE source_scope = ? AND platform = 'codex' AND file_path <> ?
		  AND (substr(file_path, -length(?)) = ? OR substr(file_path, -length(?)) = ?)
		ORDER BY line_offset DESC, byte_offset DESC, modified_ns DESC
		LIMIT 1
	`, cursor.SourceScope, cursor.FilePath, slashSuffix, slashSuffix, backslashSuffix, backslashSuffix).
		Scan(&cursor.ModifiedNS, &cursor.FileSize, &cursor.ByteOffset, &cursor.LineOffset, &cursor.ParserState)
	if errors.Is(err, sql.ErrNoRows) {
		return cursor, nil
	}
	return cursor, err
}

func saveSessionFileCursor(cursor sessionFileCursor) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("database write queue is not initialized")
	}
	return GlobalDBQueue.Exec(`
		INSERT INTO session_log_sync (
			sync_key, source_scope, platform, file_path, modified_ns, file_size,
			byte_offset, line_offset, parser_state, last_synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(sync_key) DO UPDATE SET
			modified_ns = excluded.modified_ns,
			file_size = excluded.file_size,
			byte_offset = excluded.byte_offset,
			line_offset = excluded.line_offset,
			parser_state = excluded.parser_state,
			last_synced_at = CURRENT_TIMESTAMP
	`, cursor.SyncKey, cursor.SourceScope, cursor.Platform, cursor.FilePath, cursor.ModifiedNS, cursor.FileSize, cursor.ByteOffset, cursor.LineOffset, cursor.ParserState)
}

func readJSONLLines(cursor sessionFileCursor, consume func([]byte, int64) error) (sessionFileCursor, error) {
	file, err := os.Open(cursor.FilePath)
	if err != nil {
		return cursor, err
	}
	defer file.Close()
	if _, err := file.Seek(cursor.ByteOffset, io.SeekStart); err != nil {
		return cursor, err
	}

	reader := bufio.NewReader(file)
	byteOffset := cursor.ByteOffset
	lineOffset := cursor.LineOffset
	for {
		lineStart := byteOffset
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := strings.TrimSpace(string(line))
			if trimmed != "" {
				if json.Valid([]byte(trimmed)) {
					lineOffset++
					if err := consume([]byte(trimmed), lineOffset); err != nil {
						return cursor, err
					}
					byteOffset += int64(len(line))
				} else if readErr == io.EOF {
					byteOffset = lineStart
				} else {
					lineOffset++
					byteOffset += int64(len(line))
				}
			} else {
				byteOffset += int64(len(line))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return cursor, readErr
		}
	}
	info, err := file.Stat()
	if err != nil {
		return cursor, err
	}
	cursor.ModifiedNS = fileModifiedNS(info)
	cursor.FileSize = info.Size()
	cursor.ByteOffset = byteOffset
	cursor.LineOffset = lineOffset
	return cursor, nil
}

func persistSessionUsageRecords(ls *LogService, records []sessionUsageRecord) (int, int, error) {
	if len(records) == 0 {
		return 0, 0, nil
	}
	if GlobalDBQueueLogs == nil {
		return 0, 0, fmt.Errorf("request log write queue is not initialized")
	}

	existing, err := existingSessionUsageRecords(records)
	if err != nil {
		return 0, 0, err
	}
	logs := make([]ReqeustLog, 0, len(records))
	updates := make([]sessionUsageRecordUpdate, 0, len(records))
	pricingSnapshot := ls.resolvePricingSnapshot()
	skipped := 0
	for _, record := range records {
		stored, found := existing[record.SourceRecordID]
		if found && sessionUsageRecordUnchanged(stored, record) {
			skipped++
			continue
		}
		createdAt := stored.CreatedAt
		if !record.CreatedAt.IsZero() {
			createdAt = record.CreatedAt.UTC().Format(timeLayout)
		} else if createdAt == "" {
			createdAt = time.Now().UTC().Format(timeLayout)
		}
		logEntry := ReqeustLog{
			Platform:          record.Platform,
			Model:             record.Model,
			RequestedModel:    record.Model,
			ResponseModel:     record.Model,
			ProviderID:        record.ProviderID,
			Provider:          record.Provider,
			HttpCode:          200,
			InputTokens:       record.InputTokens,
			OutputTokens:      record.OutputTokens,
			CacheCreateTokens: record.CacheCreateTokens,
			CacheReadTokens:   record.CacheReadTokens,
			ReasoningTokens:   record.ReasoningTokens,
			IsStream:          true,
			GroupMultiplier:   1,
			DataSource:        record.DataSource,
			SourceRecordID:    record.SourceRecordID,
			SessionID:         record.SessionID,
			CreatedAt:         createdAt,
			DedupCore:         buildRequestLogDedupCore(record.Platform, record.InputTokens, record.OutputTokens, record.CacheReadTokens),
		}
		applyLogPricing(pricingSnapshot, &logEntry)
		if found {
			updates = append(updates, sessionUsageRecordUpdate{ID: stored.ID, Log: logEntry})
		} else {
			logs = append(logs, logEntry)
		}
	}

	for start := 0; start < len(logs); start += 50 {
		end := min(start+50, len(logs))
		if err := insertSessionRequestLogChunk(logs[start:end]); err != nil {
			return 0, 0, err
		}
	}
	for _, update := range updates {
		if err := updateSessionUsageRecord(update.ID, update.Log); err != nil {
			return 0, 0, err
		}
	}
	return len(logs) + len(updates), skipped, nil
}

func existingSessionUsageRecords(records []sessionUsageRecord) (map[string]existingSessionUsageRecord, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}
	existing := make(map[string]existingSessionUsageRecord)
	for start := 0; start < len(records); start += 200 {
		end := min(start+200, len(records))
		placeholders := strings.TrimSuffix(strings.Repeat("?,", end-start), ",")
		args := make([]any, 0, end-start)
		for _, record := range records[start:end] {
			args = append(args, record.SourceRecordID)
		}
		rows, err := db.Query(`SELECT id, source_record_id, platform, model, provider_id, provider,
			data_source, session_id, input_tokens, output_tokens, cache_create_tokens,
			cache_read_tokens, reasoning_tokens, created_at
			FROM request_log WHERE source_record_id IN (`+placeholders+")", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var sourceRecordID string
			var value existingSessionUsageRecord
			if err := rows.Scan(
				&value.ID, &sourceRecordID, &value.Platform, &value.Model, &value.ProviderID,
				&value.Provider, &value.DataSource, &value.SessionID, &value.InputTokens,
				&value.OutputTokens, &value.CacheCreateTokens, &value.CacheReadTokens,
				&value.ReasoningTokens, &value.CreatedAt,
			); err != nil {
				rows.Close()
				return nil, err
			}
			existing[sourceRecordID] = value
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return existing, nil
}

func sessionUsageRecordUnchanged(stored existingSessionUsageRecord, record sessionUsageRecord) bool {
	if stored.Platform != record.Platform || stored.Model != record.Model ||
		stored.ProviderID != record.ProviderID || stored.Provider != record.Provider ||
		stored.DataSource != record.DataSource || stored.SessionID != record.SessionID ||
		stored.InputTokens != record.InputTokens || stored.OutputTokens != record.OutputTokens ||
		stored.CacheCreateTokens != record.CacheCreateTokens || stored.CacheReadTokens != record.CacheReadTokens ||
		stored.ReasoningTokens != record.ReasoningTokens {
		return false
	}
	if record.CreatedAt.IsZero() {
		return true
	}
	storedCreatedAt, err := parseTimeInput(stored.CreatedAt)
	return err == nil && storedCreatedAt.UTC().Equal(record.CreatedAt.UTC())
}

func updateSessionUsageRecord(id int64, logEntry ReqeustLog) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("database write queue is not initialized")
	}
	if err := GlobalDBQueue.Exec(`
		UPDATE session_usage_dedup_state
		SET last_session_log_id = MIN(last_session_log_id, MAX(? - 1, 0)), updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, id); err != nil {
		return err
	}
	if err := GlobalDBQueue.Exec("DELETE FROM session_usage_dedup WHERE session_log_id = ?", id); err != nil {
		return err
	}
	return GlobalDBQueue.Exec(`
		UPDATE request_log SET
			platform = ?, model = ?, requested_model = ?, response_model = ?,
			provider_id = ?, provider = ?, http_code = ?, input_tokens = ?, output_tokens = ?,
			cache_create_tokens = ?, cache_read_tokens = ?, reasoning_tokens = ?, is_stream = ?,
			total_cost = ?, group_multiplier = ?, price_source = ?, input_cost = ?, output_cost = ?,
			reasoning_cost = ?, cache_create_cost = ?, cache_read_cost = ?, has_pricing = ?,
			matched_pricing_model = ?, data_source = ?, session_id = ?, dedup_core = ?, created_at = ?
		WHERE id = ?
	`,
		logEntry.Platform, logEntry.Model, logEntry.RequestedModel, logEntry.ResponseModel,
		logEntry.ProviderID, logEntry.Provider, logEntry.HttpCode, logEntry.InputTokens, logEntry.OutputTokens,
		logEntry.CacheCreateTokens, logEntry.CacheReadTokens, logEntry.ReasoningTokens, boolToInt(logEntry.IsStream),
		logEntry.TotalCost, logEntry.GroupMultiplier, logEntry.PriceSource, logEntry.InputCost, logEntry.OutputCost,
		logEntry.ReasoningCost, logEntry.CacheCreateCost, logEntry.CacheReadCost, boolToInt(logEntry.HasPricing),
		logEntry.MatchedPricingModel, logEntry.DataSource, logEntry.SessionID, logEntry.DedupCore, logEntry.CreatedAt,
		id,
	)
}

func insertSessionRequestLogChunk(logs []ReqeustLog) error {
	if len(logs) == 0 {
		return nil
	}
	const columns = `platform, model, requested_model, response_model, provider_id, provider, http_code,
		input_tokens, output_tokens, cache_create_tokens, cache_read_tokens, reasoning_tokens,
		is_stream, total_cost, group_multiplier, price_source, input_cost, output_cost, reasoning_cost,
		cache_create_cost, cache_read_cost, has_pricing, matched_pricing_model,
		data_source, source_record_id, session_id, dedup_core, created_at`
	const valuesPerRow = 28
	valueGroup := "(" + strings.TrimSuffix(strings.Repeat("?,", valuesPerRow), ",") + ")"
	query := "INSERT OR IGNORE INTO request_log (" + columns + ") VALUES " + strings.Join(repeatString(valueGroup, len(logs)), ",")
	args := make([]any, 0, len(logs)*valuesPerRow)
	for _, logEntry := range logs {
		args = append(args,
			logEntry.Platform, logEntry.Model, logEntry.RequestedModel, logEntry.ResponseModel,
			logEntry.ProviderID, logEntry.Provider, logEntry.HttpCode,
			logEntry.InputTokens, logEntry.OutputTokens, logEntry.CacheCreateTokens, logEntry.CacheReadTokens, logEntry.ReasoningTokens,
			boolToInt(logEntry.IsStream), logEntry.TotalCost, logEntry.GroupMultiplier, logEntry.PriceSource,
			logEntry.InputCost, logEntry.OutputCost, logEntry.ReasoningCost, logEntry.CacheCreateCost, logEntry.CacheReadCost,
			boolToInt(logEntry.HasPricing), logEntry.MatchedPricingModel,
			logEntry.DataSource, logEntry.SourceRecordID, logEntry.SessionID, logEntry.DedupCore, logEntry.CreatedAt,
		)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return GlobalDBQueueLogs.ExecBatchCtx(ctx, query, args...)
}

func repeatString(value string, count int) []string {
	result := make([]string, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func stableSessionRecordID(scope string, platform string, key string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{scope, platform, key}, "\x00")))
	return platform + "_session:" + hex.EncodeToString(sum[:16])
}

func buildSessionSyncKey(scope string, platform string, path string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{scope, platform, filepath.Clean(path)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeSessionModel(value string) string {
	model := strings.ToLower(strings.TrimSpace(value))
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = model[slash+1:]
	}
	if len(model) > 11 {
		suffix := model[len(model)-11:]
		if suffix[0] == '-' && suffix[5] == '-' && suffix[8] == '-' &&
			isASCIIDigits(suffix[1:5]) && isASCIIDigits(suffix[6:8]) && isASCIIDigits(suffix[9:11]) {
			model = model[:len(model)-11]
		}
	}
	if len(model) > 9 {
		suffix := model[len(model)-9:]
		if suffix[0] == '-' && isASCIIDigits(suffix[1:]) {
			model = model[:len(model)-9]
		}
	}
	return model
}

func isASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func parseSessionTimestamp(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func parseCodexCumulativeTokens(value map[string]any) codexCumulativeTokens {
	return codexCumulativeTokens{
		Input:       uint64(max(intValue(value["input_tokens"]), 0)),
		CachedInput: uint64(max(firstInt(value, "cached_input_tokens", "cache_read_input_tokens"), 0)),
		Output:      uint64(max(intValue(value["output_tokens"]), 0)),
	}
}

func saturatingSub(current uint64, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func sessionStringValue(value any) string {
	result, _ := value.(string)
	return strings.TrimSpace(result)
}

func intValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func firstString(value map[string]any, keys ...string) string {
	for _, key := range keys {
		if result := sessionStringValue(value[key]); result != "" {
			return result
		}
	}
	return ""
}

func firstInt(value map[string]any, keys ...string) int {
	for _, key := range keys {
		if result := intValue(value[key]); result != 0 {
			return result
		}
	}
	return 0
}

func fileModifiedNS(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.ModTime().UnixNano()
}

func mustFileInfo(path string) os.FileInfo {
	info, _ := os.Stat(path)
	return info
}
