/**
 * @name: Pi 会话用量同步
 * @Descripttion: 解析 Pi 本地会话记录并写入请求日志统计；同时承载 Pi 与 Oh My Pi 共用的会话 JSONL 解析引擎。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_pi.go
 */

package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// piFamilyMaxSessionBytes 单个会话文件体积上限（与 cc-switch 一致，128MiB）
	piFamilyMaxSessionBytes = 128 << 20
	// piFamilyMaxLabelBytes provider / model 标签的最大字节数
	piFamilyMaxLabelBytes = 512
	// piFamilyMinLatencyMs / piFamilyMaxLatencyMs 可信的请求耗时范围，范围外视为无计时
	piFamilyMinLatencyMs = 100
	piFamilyMaxLatencyMs = 60 * 60 * 1000
	// piFamilyUnknownModel 无法确定模型时的占位
	piFamilyUnknownModel = "unknown"
	// piFamilySubagentDir 子代理产物目录，只有转录与审计日志，没有会话头，需跳过
	piFamilySubagentDir = "subagent-artifacts"
	// piFamilyMinUnixMillis / piFamilyMaxUnixMillis 时间戳合法范围（0001-01-01 ~ 9999-12-31）
	piFamilyMinUnixMillis = -62_167_219_200_000
	piFamilyMaxUnixMillis = 253_402_300_799_999
)

// errPiFamilyNoHeader 文件里没有可识别的会话头（空文件、非会话 JSONL 等），按跳过处理而不是失败
var errPiFamilyNoHeader = errors.New("会话文件缺少有效 session header")

// piFamilyRoot 会话根目录；Path 为空表示没有可扫描的目录
type piFamilyRoot struct {
	Path string
	// ProjectLayout 为 true 时会话文件位于 root/<项目目录>/*.jsonl，否则直接位于 root 下
	ProjectLayout bool
}

// piFamilyFlavor Pi 与 Oh My Pi 写同一种会话格式，差异全部收敛在这里
type piFamilyFlavor struct {
	label      string
	platform   string
	dataSource string
	// idPrefix / hashTag 请求 ID 哈希的前缀与域分隔符，沿用 cc-switch 的取值
	idPrefix string
	hashTag  string
	// modelUsageCarrier Oh My Pi 的 model_usage 条目也携带用量（自动思考评审、任务小组件等会话外调用）
	modelUsageCarrier bool
	// titleSlot Oh My Pi 在会话头之前会写一行定宽的 {"type":"title"} 标题槽
	titleSlot bool
	// subagentToolResults Pi 官方 subagent 示例扩展的 toolResult 会在 details.results 里带子代理用量
	subagentToolResults bool
	resolveRoot         func(home string, scope string) (piFamilyRoot, error)
}

var piFamilyPi = &piFamilyFlavor{
	label:               "Pi",
	platform:            sessionPlatformPi,
	dataSource:          requestLogDataSourcePiSession,
	idPrefix:            "pi_session",
	hashTag:             "pi-session",
	subagentToolResults: true,
	resolveRoot:         piResolveSessionRoot,
}

// piFamilyParserState 增量续读所需的会话上下文，存放在游标的 ParserState 中
type piFamilyParserState struct {
	SessionID string `json:"session_id"`
	// SessionTS 会话头时间戳（Unix 秒）；nil 表示会话头没有可用时间
	SessionTS *int64 `json:"session_ts,omitempty"`
}

// piFamilyContext 解析单个会话文件时的固定上下文
type piFamilyContext struct {
	flavor    *piFamilyFlavor
	scope     string
	sessionID string
	// sessionTS 会话头时间戳（Unix 秒），hasSessionTS 标明是否有效
	sessionTS    int64
	hasSessionTS bool
	// fileTS 文件修改时间（Unix 秒），作为时间戳的最后兜底
	fileTS int64
}

// syncPiSessionUsage Pi 会话用量同步入口；home 为用户主目录
func syncPiSessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	return piFamilySync(ls, scope, home, piFamilyPi)
}

// piFamilySync Pi / Oh My Pi 共用的同步流程：发现文件、过滤共享根目录、逐文件增量导入
//   - 单个文件失败不会中断其余文件，错误汇总到 result.Error，保留已导入的计数
//   - 子代理 / 分叉会话（位于父会话同名目录下）始终计入；父会话里汇总了子会话用量的投影条目则跳过，避免重复
func piFamilySync(ls *LogService, scope string, home string, flavor *piFamilyFlavor) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: flavor.platform}
	root, err := flavor.resolveRoot(home, scope)
	if err != nil {
		return result, err
	}
	if root.Path == "" {
		return result, nil
	}
	files := piFamilySessionFiles(root)
	if len(files) == 0 {
		return result, nil
	}

	// Pi 与 Oh My Pi 可能共用同一个会话目录：以标题槽开头的文件归 Oh My Pi，其余归 Pi，避免两边重复计入
	if piFamilySharesRoot(home, scope) {
		owned := make([]string, 0, len(files))
		for _, path := range files {
			if piFamilyOpensWithTitleSlot(path) == flavor.titleSlot {
				owned = append(owned, path)
			}
		}
		files = owned
	}
	result.FilesScanned = len(files)

	imported := piFamilyImportedSet(files)
	var failures []string
	for _, path := range files {
		importedCount, skipped, err := piFamilySyncFile(ls, scope, path, flavor, imported)
		result.Imported += importedCount
		result.Skipped += skipped
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", path, err))
		}
	}
	if len(failures) > 0 {
		result.Error = strings.Join(failures, "; ")
	}
	return result, nil
}

// piFamilySyncFile 增量导入单个会话文件，返回 (导入数, 跳过数, 错误)；无会话头的文件按跳过处理
func piFamilySyncFile(ls *LogService, scope string, path string, flavor *piFamilyFlavor, imported map[string]struct{}) (int, int, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, 0, fmt.Errorf("%s 会话路径不是普通 JSONL 文件", flavor.label)
	}
	if info.Size() > piFamilyMaxSessionBytes {
		return 0, 0, fmt.Errorf("%s 会话文件超过 %d 字节安全上限", flavor.label, piFamilyMaxSessionBytes)
	}

	cursor, unchanged, err := prepareSessionFileCursor(scope, flavor.platform, path)
	if err != nil || unchanged {
		return 0, 0, err
	}

	// 续读需要会话头里的 id 与时间；上下文丢失时只能回到文件开头重新解析
	var state piFamilyParserState
	if cursor.ParserState != "" {
		if json.Unmarshal([]byte(cursor.ParserState), &state) != nil {
			state = piFamilyParserState{}
		}
	}
	if state.SessionID == "" {
		cursor.ByteOffset = 0
		cursor.LineOffset = 0
	}

	ctx := &piFamilyContext{
		flavor:    flavor,
		scope:     scope,
		sessionID: state.SessionID,
		fileTS:    info.ModTime().Unix(),
	}
	if state.SessionTS != nil {
		ctx.sessionTS = *state.SessionTS
		ctx.hasSessionTS = true
	}

	records := make([]sessionUsageRecord, 0, 16)
	seen := make(map[string]struct{}, 16)
	appendRecord := func(record sessionUsageRecord) {
		if _, exists := seen[record.SourceRecordID]; exists {
			return
		}
		seen[record.SourceRecordID] = struct{}{}
		records = append(records, record)
	}

	next, err := readJSONLLines(cursor, func(line []byte, _ int64) error {
		entry := piFamilyDecodeObject(line)
		if entry == nil {
			return nil
		}
		if ctx.sessionID == "" {
			entryType := piFamilyString(entry["type"])
			// Oh My Pi 的标题槽没有用量，直接跳过，继续找会话头
			if flavor.titleSlot && entryType == "title" {
				return nil
			}
			if entryType != "session" {
				return errPiFamilyNoHeader
			}
			id := piFamilyString(entry["id"])
			if !piFamilyValidTreeID(id) {
				return errPiFamilyNoHeader
			}
			ctx.sessionID = id
			if millis, ok := piFamilyTimestampMillis(entry["timestamp"]); ok {
				ctx.sessionTS = millis / 1000
				ctx.hasSessionTS = true
			}
			return nil
		}

		if subagent, ok := piFamilyParseSubagentRecords(ctx, entry); ok {
			for _, record := range subagent {
				appendRecord(record)
			}
			return nil
		}
		record, ok := piFamilyParseUsageRecord(ctx, entry)
		if !ok {
			return nil
		}
		if piFamilyDuplicatesImported(scope, entry, imported) {
			return nil
		}
		appendRecord(record)
		return nil
	})
	if errors.Is(err, errPiFamilyNoHeader) {
		return 0, 1, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if ctx.sessionID == "" {
		// 文件里还没有会话头（刚创建或只有标题槽），不保存游标，下一轮继续看
		return 0, 0, nil
	}

	records, duplicated, err := piFamilyDropCrossSessionDuplicates(records)
	if err != nil {
		return 0, 0, err
	}
	importedCount, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	skipped += duplicated

	newState := piFamilyParserState{SessionID: ctx.sessionID}
	if ctx.hasSessionTS {
		sessionTS := ctx.sessionTS
		newState.SessionTS = &sessionTS
	}
	encoded, err := json.Marshal(newState)
	if err != nil {
		return 0, 0, err
	}
	next.ParserState = string(encoded)
	if err := saveSessionFileCursor(next); err != nil {
		return 0, 0, err
	}
	return importedCount, skipped, nil
}

// piFamilyDropCrossSessionDuplicates 分叉出来的会话会把父会话的前缀条目原样抄进新文件，同一次请求只应记一次：
// 来源 ID 已存在且属于另一个会话的记录视为重复，先到先得；返回 (保留的记录, 丢弃数)
func piFamilyDropCrossSessionDuplicates(records []sessionUsageRecord) ([]sessionUsageRecord, int, error) {
	if len(records) == 0 {
		return records, 0, nil
	}
	existing, err := existingSessionUsageRecords(records)
	if err != nil {
		return nil, 0, err
	}
	kept := make([]sessionUsageRecord, 0, len(records))
	dropped := 0
	for _, record := range records {
		if stored, found := existing[record.SourceRecordID]; found && stored.SessionID != record.SessionID {
			dropped++
			continue
		}
		kept = append(kept, record)
	}
	return kept, dropped, nil
}

// piFamilyParseUsageRecord 把一条会话条目转换为用量记录；没有用量或 token 全为 0 时返回 false
//   - message/assistant 与 model_usage 条目带 provider/model/耗时；toolResult、compaction、branch_summary 没有模型归属，记为 unknown
//   - InputTokens 为不含缓存的新增输入；cacheRead / cacheWrite 分别对应缓存读 / 缓存创建
func piFamilyParseUsageRecord(ctx *piFamilyContext, entry map[string]any) (sessionUsageRecord, bool) {
	flavor := ctx.flavor
	var (
		kind       string
		usageValue any
		carrier    map[string]any
		hasUsage   bool
	)
	switch piFamilyString(entry["type"]) {
	case "message":
		message := mapValue(entry["message"])
		if message == nil {
			return sessionUsageRecord{}, false
		}
		switch piFamilyString(message["role"]) {
		case "assistant":
			kind = "assistant"
		case "toolResult":
			kind = "tool_result"
		default:
			return sessionUsageRecord{}, false
		}
		usageValue, hasUsage = message["usage"]
		carrier = message
	case "compaction":
		kind = "compaction"
		usageValue, hasUsage = entry["usage"]
	case "branch_summary":
		kind = "branch_summary"
		usageValue, hasUsage = entry["usage"]
	case "model_usage":
		if !flavor.modelUsageCarrier {
			return sessionUsageRecord{}, false
		}
		kind = "model_usage"
		usageValue, hasUsage = entry["usage"]
		carrier = entry
	default:
		return sessionUsageRecord{}, false
	}
	if !hasUsage {
		return sessionUsageRecord{}, false
	}
	attributed := kind == "assistant" || kind == "model_usage"
	usage := mapValue(usageValue)

	input := piFamilyTokenCount(usage["input"])
	output := piFamilyTokenCount(usage["output"])
	cacheRead := piFamilyTokenCount(usage["cacheRead"])
	cacheWrite := piFamilyTokenCount(usage["cacheWrite"])
	if input == 0 && output == 0 && cacheRead == 0 && cacheWrite == 0 {
		return sessionUsageRecord{}, false
	}

	model := piFamilyUnknownModel
	if attributed && carrier != nil {
		model = piFamilyBoundedLabel(carrier["model"], piFamilyUnknownModel)
		if actual := piFamilyString(carrier["responseModel"]); actual != "" {
			model = piFamilyTruncateLabel(actual)
		}
	}

	// 时间戳优先取条目自身，其次是 message 内的时间，再依次回落到会话头和文件修改时间
	createdSec := ctx.fileTS
	eventMillis, hasEvent := piFamilyTimestampMillis(entry["timestamp"])
	if !hasEvent && carrier != nil {
		eventMillis, hasEvent = piFamilyTimestampMillis(carrier["timestamp"])
	}
	switch {
	case hasEvent:
		createdSec = eventMillis / 1000
	case ctx.hasSessionTS:
		createdSec = ctx.sessionTS
	}

	// 耗时：优先用调用自带的实测 duration（毫秒，含首字等待）；没有时用 message 起始时间戳与条目落盘时间之差估算
	latencyMs := int64(0)
	firstTokenMs := int64(0)
	if attributed && carrier != nil {
		if duration, ok := piFamilyFloat(carrier["duration"]); ok && duration > 0 {
			latencyMs = piFamilyPlausibleLatency(0, int64(piFamilyRound(duration)))
		}
		if latencyMs == 0 {
			start, okStart := piFamilyTimestampMillis(carrier["timestamp"])
			end, okEnd := piFamilyTimestampMillis(entry["timestamp"])
			if okStart && okEnd {
				latencyMs = piFamilyPlausibleLatency(start, end)
			}
		}
		// Oh My Pi 额外记录了 ttft（首字耗时，毫秒）；cc-switch 未使用，这里仅在其落在 (0, 耗时] 内时采用
		if ttft, ok := piFamilyFloat(carrier["ttft"]); ok && latencyMs > 0 {
			first := int64(piFamilyRound(ttft))
			if first > 0 && first <= latencyMs {
				firstTokenMs = first
			}
		}
	}

	record := newSessionUsageRecordForSource(flavor.dataSource)
	record.Model = model
	record.SessionID = ctx.sessionID
	record.InputTokens = input
	record.OutputTokens = output
	record.CacheReadTokens = cacheRead
	record.CacheCreateTokens = cacheWrite
	record.CreatedAt = time.Unix(createdSec, 0).UTC()
	record.DurationSec = float64(latencyMs) / 1000.0
	record.FirstTokenSec = float64(firstTokenMs) / 1000.0
	record.SourceRecordID = stableSessionRecordID(ctx.scope, flavor.platform, piFamilyRequestID(flavor, entry, kind, usageValue, carrier))
	return record, true
}

// piFamilyParseSubagentRecords 解析 Pi 官方 subagent 示例扩展的 toolResult：子代理以 --no-session 运行，
// 父会话 details.results 是其用量的唯一记录。details 里带 sessionFile 链接的属于 pi-subagents 投影，
// 子会话文件自己就是记录，这里不读取。返回 (记录, 是否属于该契约)
func piFamilyParseSubagentRecords(ctx *piFamilyContext, entry map[string]any) ([]sessionUsageRecord, bool) {
	if !ctx.flavor.subagentToolResults || piFamilyString(entry["type"]) != "message" {
		return nil, false
	}
	message := mapValue(entry["message"])
	if message == nil || piFamilyString(message["role"]) != "toolResult" || piFamilyString(message["toolName"]) != "subagent" {
		return nil, false
	}
	if _, hasUsage := message["usage"]; hasUsage {
		return nil, false
	}
	details := mapValue(message["details"])
	switch piFamilyString(details["mode"]) {
	case "single", "parallel", "chain":
	default:
		return nil, false
	}
	var links []string
	piFamilyCollectSessionFiles(details, &links)
	if len(links) > 0 {
		return nil, false
	}
	results, ok := details["results"].([]any)
	if !ok {
		return nil, false
	}

	parentID := piFamilyRequestID(ctx.flavor, entry, "subagent", nil, message)
	records := make([]sessionUsageRecord, 0, len(results))
	for resultIndex, rawResult := range results {
		result := mapValue(rawResult)
		if result == nil {
			continue
		}
		hasRequestUsage := false
		if messages, ok := result["messages"].([]any); ok {
			for messageIndex, rawChild := range messages {
				child := mapValue(rawChild)
				if child == nil || piFamilyString(child["role"]) != "assistant" || mapValue(child["usage"]) == nil {
					continue
				}
				hasRequestUsage = true
				// 父条目是整个任务结束后才写入的，不能当作单次请求的结束时间，所以子请求沿用自己的时间戳
				timestamp, hasTimestamp := child["timestamp"]
				if !hasTimestamp {
					timestamp = entry["timestamp"]
				}
				childEntry := map[string]any{
					"type":      "message",
					"id":        fmt.Sprintf("%s:result:%d:message:%d", parentID, resultIndex, messageIndex),
					"timestamp": timestamp,
					"message":   child,
				}
				if record, ok := piFamilyParseUsageRecord(ctx, childEntry); ok {
					records = append(records, record)
				}
			}
		}
		if hasRequestUsage {
			continue // result.usage 是这些请求之和，已逐条计入
		}
		// 部分保存下来的结果只剩汇总用量：作为一条聚合记录保留，无法还原单次请求与耗时
		usage := mapValue(result["usage"])
		if usage == nil {
			continue
		}
		usageCopy := make(map[string]any, len(usage)+1)
		for key, value := range usage {
			usageCopy[key] = value
		}
		usageCopy["cost"] = map[string]any{"total": usage["cost"]}
		aggregate := map[string]any{
			"type":      "message",
			"id":        fmt.Sprintf("%s:result:%d:aggregate", parentID, resultIndex),
			"timestamp": entry["timestamp"],
			"message": map[string]any{
				"role":         "assistant",
				"model":        result["model"],
				"stopReason":   result["stopReason"],
				"errorMessage": result["errorMessage"],
				"usage":        usageCopy,
			},
		}
		if record, ok := piFamilyParseUsageRecord(ctx, aggregate); ok {
			records = append(records, record)
		}
	}
	return records, true
}

// piFamilyDuplicatesImported pi-subagents 的投影条目汇总了所链接子会话的用量；
// 链接的会话全部已在本轮导入时，投影本身要跳过
func piFamilyDuplicatesImported(scope string, entry map[string]any, imported map[string]struct{}) bool {
	if piFamilyString(entry["type"]) != "message" {
		return false
	}
	message := mapValue(entry["message"])
	if message == nil || piFamilyString(message["role"]) != "toolResult" {
		return false
	}
	if _, hasUsage := message["usage"]; !hasUsage {
		return false
	}
	var links []string
	piFamilyCollectSessionFiles(message["details"], &links)
	if len(links) == 0 {
		return false
	}
	for _, link := range links {
		if !piFamilyLinkImported(scope, link, imported) {
			return false
		}
	}
	return true
}

// piFamilyLinkImported 判断投影里的 sessionFile 是否对应本轮已导入的文件；
// WSL 等跨系统场景链接路径无法直接解析，退而按末尾三段路径匹配
func piFamilyLinkImported(scope string, link string, imported map[string]struct{}) bool {
	if canonical, err := filepath.EvalSymlinks(link); err == nil {
		if _, ok := imported[canonical]; ok {
			return true
		}
	}
	if scope == "native" {
		return false
	}
	parts := strings.Split(strings.Trim(strings.ReplaceAll(link, "\\", "/"), "/"), "/")
	if len(parts) > 3 {
		parts = parts[len(parts)-3:]
	}
	suffix := "/" + strings.Join(parts, "/")
	for path := range imported {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
			return true
		}
	}
	return false
}

// piFamilyCollectSessionFiles 递归收集 details 中所有 sessionFile 字符串字段
func piFamilyCollectSessionFiles(value any, links *[]string) {
	switch typed := value.(type) {
	case map[string]any:
		for key, field := range typed {
			if text, ok := field.(string); ok && key == "sessionFile" {
				*links = append(*links, text)
				continue
			}
			piFamilyCollectSessionFiles(field, links)
		}
	case []any:
		for _, item := range typed {
			piFamilyCollectSessionFiles(item, links)
		}
	}
}

// piFamilyRequestID 生成稳定的单次请求标识，沿用 cc-switch 的 request-v3 口径：
// 有条目 id 时对 (kind, id, timestamp) 取哈希；没有 id 时退化为包含消息元数据与用量的语义哈希
func piFamilyRequestID(flavor *piFamilyFlavor, entry map[string]any, kind string, usage any, message map[string]any) string {
	semanticHasher := sha256.New()
	piFamilyHashField(semanticHasher, []byte(flavor.hashTag+"-semantic-v1"))
	piFamilyHashField(semanticHasher, []byte(kind))

	if timestamp, ok := entry["timestamp"]; ok {
		piFamilyHashField(semanticHasher, []byte("entry_timestamp"))
		piFamilyHashJSON(semanticHasher, timestamp)
	}
	if message != nil {
		if timestamp, ok := message["timestamp"]; ok {
			piFamilyHashField(semanticHasher, []byte("message_timestamp"))
			piFamilyHashJSON(semanticHasher, timestamp)
		}
		for _, key := range []string{"provider", "model", "responseModel", "responseId", "api", "toolCallId", "toolName", "stopReason", "errorMessage"} {
			if value, ok := message[key]; ok {
				piFamilyHashField(semanticHasher, []byte(key))
				piFamilyHashJSON(semanticHasher, value)
			}
		}
		if content, ok := message["content"]; ok {
			piFamilyHashField(semanticHasher, []byte("content"))
			piFamilyHashJSON(semanticHasher, content)
		}
	} else if summary, ok := entry["summary"]; ok {
		piFamilyHashField(semanticHasher, []byte("summary"))
		piFamilyHashJSON(semanticHasher, summary)
	}
	piFamilyHashField(semanticHasher, []byte("usage"))
	piFamilyHashJSON(semanticHasher, usage)
	semanticID := flavor.idPrefix + "_semantic:" + hex.EncodeToString(semanticHasher.Sum(nil))

	entryID := piFamilyString(entry["id"])
	if entryID == "" {
		return semanticID
	}
	requestHasher := sha256.New()
	piFamilyHashField(requestHasher, []byte(flavor.hashTag+"-request-v3"))
	piFamilyHashField(requestHasher, []byte(kind))
	piFamilyHashField(requestHasher, []byte(entryID))
	if timestamp, ok := entry["timestamp"]; ok {
		piFamilyHashJSON(requestHasher, timestamp)
	}
	return flavor.idPrefix + ":" + hex.EncodeToString(requestHasher.Sum(nil))
}

// piFamilyHashField 长度前缀 + 内容，避免字段拼接产生歧义
func piFamilyHashField(hasher hash.Hash, value []byte) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	hasher.Write(length[:])
	hasher.Write(value)
}

// piFamilyHashJSON 对 JSON 值做确定性哈希（对象键按字典序）
func piFamilyHashJSON(hasher hash.Hash, value any) {
	switch typed := value.(type) {
	case nil:
		piFamilyHashField(hasher, []byte("null"))
	case bool:
		piFamilyHashField(hasher, []byte("bool"))
		if typed {
			piFamilyHashField(hasher, []byte("true"))
		} else {
			piFamilyHashField(hasher, []byte("false"))
		}
	case json.Number:
		piFamilyHashField(hasher, []byte("number"))
		piFamilyHashField(hasher, []byte(typed.String()))
	case string:
		piFamilyHashField(hasher, []byte("string"))
		piFamilyHashField(hasher, []byte(typed))
	case []any:
		piFamilyHashField(hasher, []byte("array"))
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(typed)))
		piFamilyHashField(hasher, length[:])
		for _, item := range typed {
			piFamilyHashJSON(hasher, item)
		}
	case map[string]any:
		piFamilyHashField(hasher, []byte("object"))
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(typed)))
		piFamilyHashField(hasher, length[:])
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			piFamilyHashField(hasher, []byte(key))
			piFamilyHashJSON(hasher, typed[key])
		}
	default:
		piFamilyHashField(hasher, []byte("null"))
	}
}

// piFamilyDecodeObject 解码一行 JSON 对象；数字保留为 json.Number，避免大整数精度丢失
func piFamilyDecodeObject(line []byte) map[string]any {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var value map[string]any
	if decoder.Decode(&value) != nil {
		return nil
	}
	return value
}

// piFamilyString 取去除首尾空白后的非空字符串，其余类型返回空串
func piFamilyString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

// piFamilyInt64 按 serde_json::as_i64 的口径取整数：浮点形式（如 5.0）不算整数
func piFamilyInt64(value any) (int64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Int64()
	return parsed, err == nil
}

// piFamilyUint 按 as_u64 口径取非负整数，不合法返回 0；超出 uint64 的按 0 处理
func piFamilyUint(value any) uint64 {
	number, ok := value.(json.Number)
	if !ok {
		return 0
	}
	parsed, err := number.Int64()
	if err != nil || parsed < 0 {
		return 0
	}
	return uint64(parsed)
}

// piFamilyFloat 取有限浮点数
func piFamilyFloat(value any) (float64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	parsed, err := number.Float64()
	if err != nil || parsed != parsed || parsed > 1e300 || parsed < -1e300 {
		return 0, false
	}
	return parsed, true
}

// piFamilyTokenCount 读取 token 数量，负数或非整数按 0 处理，封顶 uint32 上限
func piFamilyTokenCount(value any) int {
	count := piFamilyUint(value)
	if count > 1<<32-1 {
		count = 1<<32 - 1
	}
	return int(count)
}

// piFamilyTimestampMillis 解析毫秒时间戳：整数绝对值不超过 1e11 视为秒，字符串按 RFC3339 解析
func piFamilyTimestampMillis(value any) (int64, bool) {
	var millis int64
	if number, ok := piFamilyInt64(value); ok {
		millis = number
		if number >= -100_000_000_000 && number <= 100_000_000_000 {
			millis = number * 1000
		}
	} else if text, ok := value.(string); ok {
		parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(text))
		if err != nil {
			return 0, false
		}
		millis = parsed.UnixMilli()
	} else {
		return 0, false
	}
	if millis < piFamilyMinUnixMillis || millis > piFamilyMaxUnixMillis {
		return 0, false
	}
	return millis, true
}

// piFamilyPlausibleLatency 两个毫秒时间点之差落在可信范围内才返回，否则返回 0（无计时）
func piFamilyPlausibleLatency(startMs int64, endMs int64) int64 {
	latency := endMs - startMs
	if latency < piFamilyMinLatencyMs || latency > piFamilyMaxLatencyMs {
		return 0
	}
	return latency
}

// piFamilyRound 四舍五入，0.5 向远离零的方向取整
func piFamilyRound(value float64) float64 {
	if value < 0 {
		return -float64(int64(-value + 0.5))
	}
	return float64(int64(value + 0.5))
}

func piFamilyBoundedLabel(value any, fallback string) string {
	if text := piFamilyString(value); text != "" {
		return piFamilyTruncateLabel(text)
	}
	return fallback
}

// piFamilyTruncateLabel 把标签截到 512 字节以内，且不切断多字节字符
func piFamilyTruncateLabel(value string) string {
	if len(value) <= piFamilyMaxLabelBytes {
		return value
	}
	end := piFamilyMaxLabelBytes
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}

// piFamilyValidTreeID 会话 id 只允许字母数字及 - _ .，首尾必须是字母数字，长度不超过 256
func piFamilyValidTreeID(id string) bool {
	if id == "" || len(id) > 256 {
		return false
	}
	isAlnum := func(char byte) bool {
		return (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
	}
	if !isAlnum(id[0]) || !isAlnum(id[len(id)-1]) {
		return false
	}
	for index := 0; index < len(id); index++ {
		char := id[index]
		if !isAlnum(char) && char != '-' && char != '_' && char != '.' {
			return false
		}
	}
	return true
}

// piFamilySessionFiles 按会话根目录的布局发现 .jsonl 文件，并追加嵌套的子代理 / 分叉会话
func piFamilySessionFiles(root piFamilyRoot) []string {
	entries, err := os.ReadDir(root.Path)
	if err != nil {
		return nil
	}
	top := make([]string, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(root.Path, entry.Name())
		if root.ProjectLayout {
			if !entry.IsDir() {
				continue
			}
			projectEntries, err := os.ReadDir(path)
			if err != nil {
				continue
			}
			for _, projectEntry := range projectEntries {
				if projectEntry.Type().IsRegular() && strings.HasSuffix(projectEntry.Name(), ".jsonl") {
					top = append(top, filepath.Join(path, projectEntry.Name()))
				}
			}
			continue
		}
		if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".jsonl") {
			top = append(top, path)
		}
	}

	files := append([]string(nil), top...)
	for _, session := range top {
		piFamilyCollectChildSessions(strings.TrimSuffix(session, ".jsonl"), &files)
	}
	sort.Strings(files)
	// 去重（理论上不会重复，防御符号链接等异常布局）
	unique := files[:0]
	for index, path := range files {
		if index == 0 || path != files[index-1] {
			unique = append(unique, path)
		}
	}
	return unique
}

// piFamilyCollectChildSessions 子代理与分叉会话嵌套在父会话去掉扩展名的目录下，孙会话同理；
// subagent-artifacts 目录没有会话头，跳过。DirEntry 不跟随符号链接，因此不会成环
func piFamilyCollectChildSessions(dir string, files *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		switch {
		case entry.IsDir():
			if entry.Name() != piFamilySubagentDir {
				piFamilyCollectChildSessions(path, files)
			}
		case entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".jsonl"):
			*files = append(*files, path)
		}
	}
}

// piFamilyImportedSet 本轮文件的规范化路径集合，用于识别投影与子会话的对应关系
func piFamilyImportedSet(files []string) map[string]struct{} {
	imported := make(map[string]struct{}, len(files))
	for _, path := range files {
		if canonical, err := filepath.EvalSymlinks(path); err == nil {
			imported[canonical] = struct{}{}
		} else {
			imported[path] = struct{}{}
		}
	}
	return imported
}

// piFamilyOpensWithTitleSlot 文件首行是否为 Oh My Pi 的标题槽（只探测前 4KiB）
func piFamilyOpensWithTitleSlot(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	buffer := make([]byte, 4096)
	count, _ := file.Read(buffer)
	line := buffer[:count]
	if newline := bytes.IndexByte(line, '\n'); newline >= 0 {
		line = line[:newline]
	}
	entry := piFamilyDecodeObject(bytes.TrimSpace(line))
	return entry != nil && piFamilyString(entry["type"]) == "title"
}

// piFamilySharesRoot Pi 与 Oh My Pi 当前是否解析到同一个会话根目录
func piFamilySharesRoot(home string, scope string) bool {
	piRoot, piErr := piResolveSessionRoot(home, scope)
	ompRoot, ompErr := ompResolveSessionRoot(home, scope)
	if piErr != nil || ompErr != nil || piRoot.Path == "" || ompRoot.Path == "" {
		return false
	}
	canonical := func(path string) string {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return resolved
		}
		return filepath.Clean(path)
	}
	return canonical(piRoot.Path) == canonical(ompRoot.Path)
}

// piExpandHome 展开以 ~ 开头的路径；非绝对路径返回 ok=false（需要项目上下文才能解析）
func piExpandHome(value string, home string) (string, bool) {
	value = strings.TrimSpace(value)
	switch {
	case value == "~":
		value = home
	case strings.HasPrefix(value, "~/") || strings.HasPrefix(value, "~\\"):
		value = filepath.Join(home, value[2:])
	}
	if !filepath.IsAbs(value) {
		return value, false
	}
	return filepath.Clean(value), true
}

// piResolveSessionRoot Pi 的会话根目录，优先级：
//  1. PI_CODING_AGENT_SESSION_DIR（仅本机）
//  2. <agent 目录>/settings.json 的 sessionDir（WSL 环境只认 ~ 开头的值）
//  3. <agent 目录>/sessions（按项目分目录）
//
// agent 目录：本机取 PI_CODING_AGENT_DIR，默认 ~/.pi/agent。目录不存在不算错误；相对路径无法全局枚举，返回错误
func piResolveSessionRoot(home string, scope string) (piFamilyRoot, error) {
	native := scope == "native"
	if native {
		if raw := os.Getenv("PI_CODING_AGENT_SESSION_DIR"); raw != "" {
			return piConfiguredSessionRoot(raw, home, "环境变量 PI_CODING_AGENT_SESSION_DIR")
		}
	}

	agentDir := filepath.Join(home, ".pi", "agent")
	if native {
		if raw := os.Getenv("PI_CODING_AGENT_DIR"); raw != "" {
			resolved, ok := piExpandHome(raw, home)
			if !ok {
				return piFamilyRoot{}, fmt.Errorf("PI_CODING_AGENT_DIR 必须是绝对路径: %s", raw)
			}
			agentDir = resolved
		}
	}

	if content, err := os.ReadFile(filepath.Join(agentDir, "settings.json")); err == nil {
		var settings struct {
			SessionDir string `json:"sessionDir"`
		}
		if json.Unmarshal(content, &settings) == nil && strings.TrimSpace(settings.SessionDir) != "" {
			value := strings.TrimSpace(settings.SessionDir)
			if native || strings.HasPrefix(value, "~") {
				return piConfiguredSessionRoot(value, home, "settings.json 的 sessionDir")
			}
		}
	}
	return piFamilyRoot{Path: filepath.Join(agentDir, "sessions"), ProjectLayout: true}, nil
}

// piConfiguredSessionRoot 显式配置的会话目录：会话文件直接位于该目录下；目录不存在时视为空
func piConfiguredSessionRoot(value string, home string, source string) (piFamilyRoot, error) {
	path, ok := piExpandHome(value, home)
	if !ok {
		return piFamilyRoot{}, fmt.Errorf("Pi 会话目录（%s）为相对路径 %q，无法全局枚举", source, value)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return piFamilyRoot{}, nil
	}
	return piFamilyRoot{Path: path}, nil
}

// piFamilyUsesXDG Oh My Pi 的 XDG 数据目录规则仅适用于 Linux / macOS
func piFamilyUsesXDG() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}
