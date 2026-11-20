/**
 * @name: 会话用量同步
 * @Descripttion: 增量解析 Claude、Codex、Gemini、OpenCode、Grok 及其他 CLI 本地会话并写入日志统计。
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
	"net/url"
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
	// BackfillLineOffset 升级前已导入到的行号；重新解析时该行及之前的记录只更新、不新建
	BackfillLineOffset int64
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
	// DurationSec 会话推算/原生记录的请求总耗时（秒），0 表示无计时
	DurationSec float64
	// FirstTokenSec 会话推算/原生记录的首字耗时（秒），0 表示无计时
	FirstTokenSec float64
	// UpdateOnly 只允许更新已存在的行（历史回填场景），库中不存在时跳过
	UpdateOnly bool
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
	DurationSec       float64
	FirstTokenSec     float64
}

type sessionUsageRecordUpdate struct {
	ID  int64
	Log ReqeustLog
}

type claudeSessionUsageCandidate struct {
	Record        sessionUsageRecord
	HasStopReason bool
	LineNumber    int64
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
	// Timer 请求耗时估算状态，跨增量同步保留
	Timer codexRequestTimer `json:"timer"`
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
		{platform: "opencode", root: filepath.Join(home, ".local", "share", "opencode"), sync: syncOpenCodeSessionUsage},
		{platform: "grokbuild", root: filepath.Join(home, ".grok"), sync: syncGrokSessionUsage},
		// 以下来源的入参为用户主目录，由各解析器自行定位数据目录（含多根目录与环境变量覆盖）
		{platform: sessionPlatformAntigravity, root: home, sync: syncAntigravitySessionUsage},
		{platform: sessionPlatformKimi, root: home, sync: syncKimiSessionUsage},
		{platform: sessionPlatformCopilot, root: home, sync: syncCopilotSessionUsage},
		{platform: sessionPlatformWorkBuddy, root: home, sync: syncWorkBuddySessionUsage},
		{platform: sessionPlatformDeepSeekHarness, root: home, sync: syncDeepSeekHarnessSessionUsage},
		{platform: sessionPlatformMiniMaxCode, root: home, sync: syncMiniMaxCodeSessionUsage},
		{platform: sessionPlatformOhMyPi, root: home, sync: syncOhMyPiSessionUsage},
		{platform: sessionPlatformPi, root: home, sync: syncPiSessionUsage},
	}

	for _, job := range jobs {
		sourceResult, err := job.sync(ls, scope, job.root)
		if err != nil {
			// 保留已成功文件的计数，只追加错误信息
			sourceResult.Scope = scope
			sourceResult.Platform = job.platform
			sourceResult.Error = err.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("%s/%s: %v", scope, job.platform, err))
		}
		result.Sources = append(result.Sources, sourceResult)
		result.Imported += sourceResult.Imported
		result.Skipped += sourceResult.Skipped
		result.FilesScanned += sourceResult.FilesScanned
	}
	return result
}

// sessionFileErrors 收集单个会话文件的同步错误，不中断同一来源的其他文件
type sessionFileErrors []string

func (errs *sessionFileErrors) add(path string, err error) {
	*errs = append(*errs, fmt.Sprintf("%s: %v", filepath.Base(path), err))
}

func (errs sessionFileErrors) err() error {
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "; "))
}

// sessionSQLiteReadOnlyURI 生成只读打开外部 SQLite 的 file: URI（路径按 URI 规则转义，兼容 Windows 盘符）
func sessionSQLiteReadOnlyURI(dbPath string) string {
	path := filepath.ToSlash(dbPath)
	if len(path) > 1 && path[1] == ':' {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String() + "?mode=ro"
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
	var fileErrors sessionFileErrors
	for _, path := range files {
		imported, skipped, err := syncClaudeSessionFile(ls, scope, path)
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
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
	var fileErrors sessionFileErrors
	for _, path := range files {
		imported, skipped, err := syncCodexSessionFile(ls, scope, path)
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
}

func syncGeminiSessionUsage(ls *LogService, scope string, root string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: "gemini"}
	files := collectSessionFiles(filepath.Join(root, "tmp"), func(path string) bool {
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "session-") || filepath.Base(filepath.Dir(path)) != "chats" {
			return false
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".jsonl":
			return true
		case ".json":
			// 新版 CLI 会写同名 .jsonl，两者并存时只读 .jsonl
			_, err := os.Stat(strings.TrimSuffix(path, filepath.Ext(path)) + ".jsonl")
			return err != nil
		default:
			return false
		}
	})
	result.FilesScanned = len(files)
	var fileErrors sessionFileErrors
	for _, path := range files {
		imported, skipped, err := syncGeminiSessionFile(ls, scope, path)
		if err != nil {
			// 单个文件失败不影响同一来源的其他文件，错误汇总后返回
			fileErrors.add(path, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
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
	// 估算耗时需要回复的起点行，增量读取时它常在游标之前，先回看一小段
	timing := newClaudeSessionTimingTracker()
	timing.SeedBefore(path, cursor.ByteOffset)
	next, err := readJSONLLines(cursor, func(line []byte, lineNumber int64) error {
		var value map[string]any
		if json.Unmarshal(line, &value) != nil {
			return nil
		}
		timing.Observe(value)
		if sessionStringValue(value["type"]) != "assistant" {
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
			LineNumber:    lineNumber,
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
		candidate := candidates[messageID]
		candidate.Record.DurationSec, candidate.Record.FirstTokenSec = timing.Estimate(messageID, candidate.HasStopReason)
		candidate.Record.UpdateOnly = candidate.LineNumber <= cursor.BackfillLineOffset
		records = append(records, candidate.Record)
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
		state.Timer.ObserveLine(value)
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
			tokenCountMs, _ := sessionTimestampMs(sessionStringValue(value["timestamp"]))
			// 与 cc-switch 一致用 last_token_usage 核对 token_usage_record；缺失时接受任意记录
			var lastUsage *codexCumulativeTokens
			if last := mapValue(info["last_token_usage"]); len(last) > 0 {
				parsed := parseCodexCumulativeTokens(last)
				lastUsage = &parsed
			}
			durationSec, firstTokenSec := state.Timer.Finish(tokenCountMs, lastUsage)
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
				DurationSec:     durationSec,
				FirstTokenSec:   firstTokenSec,
				UpdateOnly:      lineNumber <= cursor.BackfillLineOffset,
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
	root, err := parseGeminiSessionDocument(data)
	if err != nil {
		return 0, 0, err
	}
	isJSONL := strings.EqualFold(filepath.Ext(path), ".jsonl")
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
		// 新版 .jsonl 会通过 $set 检查点重排消息，位置序号不稳定，与 cc-switch 一致只用消息 id；
		// 旧版 .json 保留原键，避免已入库记录被重复导入
		if isJSONL && messageID != "" {
			stableKey = sessionID + ":" + messageID
		}
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

// parseGeminiSessionDocument 把 Gemini CLI 会话还原为 {sessionId, ..., messages: [...]}：
// 旧版 .json 整个文件即该对象；新版 .jsonl 按 CLI 的回放规则处理——带 sessionId 的行是元数据，
// 带 id 的行是消息（同 id 原位覆盖），{"$set": {...}} 合并元数据（含 messages 时整体替换），
// {"$rewindTo": id} 删除该条及之后的消息（找不到 id 时清空）
func parseGeminiSessionDocument(data []byte) (map[string]any, error) {
	var whole map[string]any
	if err := json.Unmarshal(data, &whole); err == nil && whole != nil {
		if _, ok := whole["messages"]; !ok {
			whole["messages"] = []any{}
		}
		return whole, nil
	}

	metadata := map[string]any{}
	messages := make([]any, 0, 32)
	indexByID := make(map[string]int, 32)
	upsert := func(raw any) {
		id := sessionStringValue(mapValue(raw)["id"])
		if id == "" {
			return
		}
		if position, ok := indexByID[id]; ok {
			messages[position] = raw
			return
		}
		indexByID[id] = len(messages)
		messages = append(messages, raw)
	}
	reset := func(list []any) {
		messages = messages[:0]
		indexByID = make(map[string]int, len(list))
		for _, item := range list {
			upsert(item)
		}
	}

	seen := false
	for _, line := range strings.Split(string(data), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(line)), &record) != nil || record == nil {
			continue
		}
		seen = true
		if rewindID, ok := record["$rewindTo"].(string); ok {
			length := 0
			if position, found := indexByID[rewindID]; found {
				length = position
			}
			messages = messages[:length]
			for id, position := range indexByID {
				if position >= length {
					delete(indexByID, id)
				}
			}
			continue
		}
		if _, ok := record["id"].(string); ok {
			upsert(record)
			continue
		}
		if set, ok := record["$set"].(map[string]any); ok {
			if list, ok := set["messages"].([]any); ok {
				reset(list)
				delete(set, "messages")
			}
			for key, value := range set {
				metadata[key] = value
			}
			continue
		}
		if _, ok := record["sessionId"].(string); ok {
			if list, ok := record["messages"].([]any); ok {
				for _, item := range list {
					upsert(item)
				}
				delete(record, "messages")
			}
			for key, value := range record {
				metadata[key] = value
			}
		}
	}
	if !seen {
		return nil, fmt.Errorf("invalid gemini session document")
	}
	metadata["messages"] = messages
	return metadata, nil
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
		// 文件被截断或改写：旧行号已无意义，历史回填标记一并清除
		cursor.ByteOffset = 0
		cursor.LineOffset = 0
		cursor.ParserState = ""
		cursor.BackfillLineOffset = 0
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
		SELECT modified_ns, file_size, byte_offset, line_offset, parser_state, backfill_line_offset
		FROM session_log_sync WHERE sync_key = ?
	`, syncKey).Scan(&cursor.ModifiedNS, &cursor.FileSize, &cursor.ByteOffset, &cursor.LineOffset, &cursor.ParserState, &cursor.BackfillLineOffset)
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
		SELECT modified_ns, file_size, byte_offset, line_offset, parser_state, backfill_line_offset
		FROM session_log_sync
		WHERE source_scope = ? AND platform = 'codex' AND file_path <> ?
		  AND (substr(file_path, -length(?)) = ? OR substr(file_path, -length(?)) = ?)
		ORDER BY line_offset DESC, byte_offset DESC, modified_ns DESC
		LIMIT 1
	`, cursor.SourceScope, cursor.FilePath, slashSuffix, slashSuffix, backslashSuffix, backslashSuffix).
		Scan(&cursor.ModifiedNS, &cursor.FileSize, &cursor.ByteOffset, &cursor.LineOffset, &cursor.ParserState, &cursor.BackfillLineOffset)
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
			byte_offset, line_offset, parser_state, backfill_line_offset, last_synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(sync_key) DO UPDATE SET
			modified_ns = excluded.modified_ns,
			file_size = excluded.file_size,
			byte_offset = excluded.byte_offset,
			line_offset = excluded.line_offset,
			parser_state = excluded.parser_state,
			backfill_line_offset = excluded.backfill_line_offset,
			last_synced_at = CURRENT_TIMESTAMP
	`, cursor.SyncKey, cursor.SourceScope, cursor.Platform, cursor.FilePath, cursor.ModifiedNS, cursor.FileSize, cursor.ByteOffset, cursor.LineOffset, cursor.ParserState, cursor.BackfillLineOffset)
}

func readJSONLLines(cursor sessionFileCursor, consume func([]byte, int64) error) (sessionFileCursor, error) {
	file, err := os.Open(cursor.FilePath)
	if err != nil {
		return cursor, err
	}
	defer file.Close()
	// 读取前记录文件状态：读取期间追加的内容会让下次同步看到 mtime/size 变化而继续读
	info, err := file.Stat()
	if err != nil {
		return cursor, err
	}
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
	receipts, err := existingSessionUsageReceipts(records, existing)
	if err != nil {
		return 0, 0, err
	}
	logs := make([]ReqeustLog, 0, len(records))
	updates := make([]sessionUsageRecordUpdate, 0, len(records))
	timingUpdates := make([]sessionUsageRecordUpdate, 0, len(records))
	pricingSnapshot := ls.resolvePricingSnapshot()
	skipped := 0
	for _, record := range records {
		stored, found := existing[record.SourceRecordID]
		if found && sessionUsageRecordUnchanged(stored, record) {
			skipped++
			continue
		}
		// 已登记回执但明细不在：用户删除过，不再复活；历史回填记录也只更新不新建
		if !found && (record.UpdateOnly || receipts[record.SourceRecordID]) {
			skipped++
			continue
		}
		durationSec, firstTokenSec := mergeSessionTiming(stored, found, record)
		// 仅补写计时：不重新计价、不重置去重匹配，避免历史成本随当前价格漂移
		if found && sessionUsageRecordSameExceptTiming(stored, record) {
			timingUpdates = append(timingUpdates, sessionUsageRecordUpdate{
				ID:  stored.ID,
				Log: ReqeustLog{DurationSec: durationSec, FirstTokenSec: firstTokenSec},
			})
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
			DurationSec:       durationSec,
			FirstTokenSec:     firstTokenSec,
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
		if err := insertSessionUsageReceipts(logs[start:end]); err != nil {
			return 0, 0, err
		}
	}
	for _, update := range updates {
		if err := updateSessionUsageRecord(update.ID, update.Log); err != nil {
			return 0, 0, err
		}
	}
	// 仅补计时的更新按批放进同一事务，避免首次回填历史时逐行占用写队列
	for start := 0; start < len(timingUpdates); start += 200 {
		end := min(start+200, len(timingUpdates))
		tasks := make([]WriteTask, 0, end-start)
		for _, update := range timingUpdates[start:end] {
			tasks = append(tasks, WriteTask{
				SQL:  "UPDATE request_log SET duration_sec = ?, first_token_sec = ? WHERE id = ?",
				Args: []interface{}{update.Log.DurationSec, update.Log.FirstTokenSec, update.ID},
			})
		}
		if err := GlobalDBQueue.ExecTxGroup(tasks); err != nil {
			return 0, 0, err
		}
	}
	return len(logs) + len(updates) + len(timingUpdates), skipped, nil
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
			cache_read_tokens, reasoning_tokens, created_at,
			COALESCE(duration_sec, 0), COALESCE(first_token_sec, 0)
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
				&value.DurationSec, &value.FirstTokenSec,
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
	if !sessionUsageRecordSameExceptTiming(stored, record) {
		return false
	}
	durationSec, firstTokenSec := mergeSessionTiming(stored, true, record)
	return !sessionTimingChanged(stored.DurationSec, durationSec) && !sessionTimingChanged(stored.FirstTokenSec, firstTokenSec)
}

// sessionUsageRecordSameExceptTiming 除计时外的身份、Token 与创建时间是否一致
func sessionUsageRecordSameExceptTiming(stored existingSessionUsageRecord, record sessionUsageRecord) bool {
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
	// created_at 以 UTC 秒精度、无时区字符串存储，按同一格式比较，避免毫秒与时区解析造成误判
	return sessionCreatedAtKey(stored.CreatedAt) == record.CreatedAt.UTC().Format(timeLayout)
}

// sessionCreatedAtKey 把库中读出的 created_at 统一为 UTC 秒精度字符串（兼容带 T/Z 的写法）
func sessionCreatedAtKey(value string) string {
	raw := strings.TrimSpace(value)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC().Format(timeLayout)
		}
	}
	if len(raw) >= len(timeLayout) {
		return strings.Replace(raw[:len(timeLayout)], "T", " ", 1)
	}
	return raw
}

// existingSessionUsageReceipts 查询库中不存在明细的记录是否已登记回执
func existingSessionUsageReceipts(records []sessionUsageRecord, existing map[string]existingSessionUsageRecord) (map[string]bool, error) {
	missing := make([]any, 0, len(records))
	for _, record := range records {
		if _, found := existing[record.SourceRecordID]; !found {
			missing = append(missing, record.SourceRecordID)
		}
	}
	receipts := make(map[string]bool, len(missing))
	if len(missing) == 0 {
		return receipts, nil
	}
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}
	for start := 0; start < len(missing); start += 200 {
		end := min(start+200, len(missing))
		placeholders := strings.TrimSuffix(strings.Repeat("?,", end-start), ",")
		rows, err := db.Query("SELECT source_record_id FROM session_usage_receipt WHERE source_record_id IN ("+placeholders+")", missing[start:end]...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			receipts[id] = true
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return receipts, nil
}

// insertSessionUsageReceipts 为新导入的会话行登记永久回执
func insertSessionUsageReceipts(logs []ReqeustLog) error {
	if len(logs) == 0 {
		return nil
	}
	args := make([]any, 0, len(logs))
	for _, logEntry := range logs {
		args = append(args, logEntry.SourceRecordID)
	}
	query := "INSERT OR IGNORE INTO session_usage_receipt (source_record_id) VALUES " + strings.Join(repeatString("(?)", len(logs)), ",")
	return GlobalDBQueue.Exec(query, args...)
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
			matched_pricing_model = ?, data_source = ?, session_id = ?, dedup_core = ?, created_at = ?,
			duration_sec = ?, first_token_sec = ?
		WHERE id = ?
	`,
		logEntry.Platform, logEntry.Model, logEntry.RequestedModel, logEntry.ResponseModel,
		logEntry.ProviderID, logEntry.Provider, logEntry.HttpCode, logEntry.InputTokens, logEntry.OutputTokens,
		logEntry.CacheCreateTokens, logEntry.CacheReadTokens, logEntry.ReasoningTokens, boolToInt(logEntry.IsStream),
		logEntry.TotalCost, logEntry.GroupMultiplier, logEntry.PriceSource, logEntry.InputCost, logEntry.OutputCost,
		logEntry.ReasoningCost, logEntry.CacheCreateCost, logEntry.CacheReadCost, boolToInt(logEntry.HasPricing),
		logEntry.MatchedPricingModel, logEntry.DataSource, logEntry.SessionID, logEntry.DedupCore, logEntry.CreatedAt,
		logEntry.DurationSec, logEntry.FirstTokenSec,
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
		data_source, source_record_id, session_id, dedup_core, created_at,
		duration_sec, first_token_sec`
	const valuesPerRow = 30
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
			logEntry.DurationSec, logEntry.FirstTokenSec,
		)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return GlobalDBQueueLogs.ExecBatchCtx(ctx, query, args...)
}

// sessionTimingChanged 判断新计时是否需要覆盖已存值：新值为 0（本次未取得计时）时保留旧值
func sessionTimingChanged(stored float64, next float64) bool {
	if next <= 0 {
		return false
	}
	diff := stored - next
	if diff < 0 {
		diff = -diff
	}
	return diff > 0.0005
}

// mergeSessionTiming 计算写入的计时：本次解析缺失计时时沿用已存值，避免增量读取丢失估算结果
func mergeSessionTiming(stored existingSessionUsageRecord, found bool, record sessionUsageRecord) (float64, float64) {
	durationSec := max(record.DurationSec, 0)
	firstTokenSec := max(record.FirstTokenSec, 0)
	if !found {
		return durationSec, firstTokenSec
	}
	// 与 cc-switch 一致：新计时缺耗时，或缺首字而旧值有首字时，整体保留旧值，避免耗时与首字来自不同口径
	if durationSec <= 0 || (firstTokenSec <= 0 && stored.FirstTokenSec > 0) {
		return stored.DurationSec, stored.FirstTokenSec
	}
	return durationSec, firstTokenSec
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
