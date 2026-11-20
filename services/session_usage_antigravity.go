/**
 * @name: Antigravity 会话用量同步
 * @Descripttion: 解析 Antigravity 本地会话记录与原生会话库，导入用量并补齐真实耗时与首字时间。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_antigravity.go
 */

package services

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// antigravityParserStateVersion 游标状态版本；解析规则变化时递增即可让历史会话重新导入
const antigravityParserStateVersion = "v1"

// antigravityTranscriptRelPath 会话目录下的完整会话记录（JSONL，同一 step 可能被重写多次）
var antigravityTranscriptRelPath = filepath.Join(".system_generated", "logs", "transcript_full.jsonl")

// antigravityTranscript 单个会话记录文件
type antigravityTranscript struct {
	SessionID string
	Path      string
	// Root Antigravity 数据根目录（brain 与 conversations 的共同父目录）
	Root string
}

// antigravityGeneration 原生会话库中某个 step 对应的生成信息
type antigravityGeneration struct {
	Model         string
	LatencySec    float64
	FirstTokenSec float64
}

// syncAntigravitySessionUsage Antigravity 会话用量同步入口；home 为用户主目录
// CLI 与 IDE 分别写入 ~/.gemini/antigravity-cli 与 ~/.gemini/antigravity，两处都扫描，同一会话只取先出现的根目录
func syncAntigravitySessionUsage(ls *LogService, scope string, home string) (SessionSyncSourceResult, error) {
	result := SessionSyncSourceResult{Scope: scope, Platform: sessionPlatformAntigravity}
	roots := []string{
		filepath.Join(home, ".gemini", "antigravity-cli"),
		filepath.Join(home, ".gemini", "antigravity"),
	}
	transcripts := collectAntigravityTranscripts(roots)
	result.FilesScanned = len(transcripts)
	var fileErrors sessionFileErrors
	for _, transcript := range transcripts {
		imported, skipped, err := syncAntigravityTranscript(ls, scope, transcript)
		if err != nil {
			// 单个会话失败不影响其他会话
			fileErrors.add(transcript.SessionID, err)
			continue
		}
		result.Imported += imported
		result.Skipped += skipped
	}
	return result, fileErrors.err()
}

// collectAntigravityTranscripts 收集各根目录 brain/<会话ID>/ 下的会话记录，跳过符号链接与越界路径
func collectAntigravityTranscripts(roots []string) []antigravityTranscript {
	seen := make(map[string]struct{}, 16)
	transcripts := make([]antigravityTranscript, 0, 16)
	for _, root := range roots {
		brain := filepath.Join(root, "brain")
		entries, err := os.ReadDir(brain)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			sessionID := entry.Name()
			if !isAntigravitySessionID(sessionID) {
				continue
			}
			if _, exists := seen[sessionID]; exists {
				continue
			}
			path := filepath.Join(brain, sessionID, antigravityTranscriptRelPath)
			info, err := os.Lstat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			seen[sessionID] = struct{}{}
			transcripts = append(transcripts, antigravityTranscript{SessionID: sessionID, Path: path, Root: root})
		}
	}
	sort.Slice(transcripts, func(i, j int) bool { return transcripts[i].Path < transcripts[j].Path })
	return transcripts
}

// isAntigravitySessionID 会话目录名只允许字母数字与连字符
func isAntigravitySessionID(id string) bool {
	if id == "" {
		return false
	}
	for _, char := range id {
		isAlnum := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')
		if !isAlnum && char != '-' {
			return false
		}
	}
	return true
}

// antigravityNativeDBPath 与会话记录同根的原生会话库 conversations/<会话ID>.db
func antigravityNativeDBPath(transcript antigravityTranscript) string {
	return filepath.Join(transcript.Root, "conversations", transcript.SessionID+".db")
}

// antigravityNativeFingerprint 原生库（含 WAL）的修改时间与大小指纹；原生库晚于会话记录写入时也能触发重新导入
func antigravityNativeFingerprint(transcript antigravityTranscript) string {
	path := antigravityNativeDBPath(transcript)
	signature := func(target string) string {
		info, err := os.Stat(target)
		if err != nil {
			return "0:0"
		}
		return fmt.Sprintf("%d:%d", fileModifiedNS(info), info.Size())
	}
	if _, err := os.Stat(path); err != nil {
		return "none"
	}
	return signature(path) + ":" + signature(path+"-wal")
}

// syncAntigravityTranscript 同步单个会话：会话记录或原生库任一变化即整份重新解析（step 会被原地重写，无法按偏移增量读取）
func syncAntigravityTranscript(ls *LogService, scope string, transcript antigravityTranscript) (int, int, error) {
	info, err := os.Stat(transcript.Path)
	if err != nil {
		return 0, 0, err
	}
	cursor, err := loadSessionFileCursor(scope, sessionPlatformAntigravity, transcript.Path)
	if err != nil {
		return 0, 0, err
	}
	nativeState := antigravityParserStateVersion + ":" + antigravityNativeFingerprint(transcript)
	modifiedNS := fileModifiedNS(info)
	if cursor.ModifiedNS == modifiedNS && cursor.FileSize == info.Size() && cursor.ParserState == nativeState {
		return 0, 0, nil
	}

	steps, err := readAntigravitySteps(transcript.Path)
	if err != nil {
		return 0, 0, err
	}
	records, nativeOK := buildAntigravityRecords(scope, transcript, steps)
	imported, skipped, err := persistSessionUsageRecords(ls, records)
	if err != nil {
		return 0, 0, err
	}
	cursor.ModifiedNS = modifiedNS
	cursor.FileSize = info.Size()
	cursor.ByteOffset = info.Size()
	cursor.LineOffset = int64(len(steps))
	// 原生库暂时读不到（被占用等）时不记录指纹，下次同步再补计时
	cursor.ParserState = ""
	if nativeOK {
		cursor.ParserState = nativeState
	}
	if err := saveSessionFileCursor(cursor); err != nil {
		return 0, 0, err
	}
	return imported, skipped, nil
}

// readAntigravitySteps 读取会话记录，同一 step_index 以最后一行为准，按 step 顺序返回
func readAntigravitySteps(path string) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byStep := make(map[int64]map[string]any, 64)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 256*1024), 64*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var value map[string]any
		if json.Unmarshal([]byte(line), &value) != nil {
			continue
		}
		stepValue, ok := value["step_index"].(float64)
		if !ok || stepValue < 0 {
			continue
		}
		byStep[int64(stepValue)] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	indexes := make([]int64, 0, len(byStep))
	for index := range byStep {
		indexes = append(indexes, index)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
	steps := make([]map[string]any, 0, len(indexes))
	for _, index := range indexes {
		steps = append(steps, byStep[index])
	}
	return steps, nil
}

// antigravityPendingRecord 构建中的记录：先按会话记录解析，再按需用原生库补模型与计时
type antigravityPendingRecord struct {
	Record  sessionUsageRecord
	Step    int64
	Display string
}

// buildAntigravityRecords 把已完成的 PLANNER_RESPONSE 转为用量记录；第二个返回值表示原生库是否读取成功
func buildAntigravityRecords(scope string, transcript antigravityTranscript, steps []map[string]any) ([]sessionUsageRecord, bool) {
	// 逐条响应可能没有 model 字段：按 step 顺序继承最近一次 `Model Selection` 切换
	currentModel := ""
	pending := make([]antigravityPendingRecord, 0, len(steps))
	for _, step := range steps {
		index := int64(step["step_index"].(float64))
		switch sessionStringValue(step["type"]) {
		case "USER_INPUT":
			if model := antigravityModelFromSettingsChange(sessionStringValue(step["content"])); model != "" {
				currentModel = model
			}
			continue
		case "PLANNER_RESPONSE":
		default:
			continue
		}
		if sessionStringValue(step["status"]) != "DONE" {
			continue
		}
		input := max(intValue(step["input_tokens"]), 0)
		output := max(intValue(step["output_tokens"]), 0)
		cacheRead := max(intValue(step["cache_read_tokens"]), 0)
		cacheWrite := max(intValue(step["cache_write_tokens"]), 0)
		if input+output+cacheRead+cacheWrite <= 0 {
			continue
		}
		display := antigravityKnownModel(step)
		if display == "" {
			display = currentModel
		}
		record := newSessionUsageRecordForSource(requestLogDataSourceAntigravitySession)
		record.SourceRecordID = stableSessionRecordID(scope, sessionPlatformAntigravity, fmt.Sprintf("%s:%d", transcript.SessionID, index))
		record.SessionID = transcript.SessionID
		// Antigravity 导出的 input_tokens 已不含缓存，output_tokens 已包含思考，不再重复加减
		record.InputTokens = input
		record.OutputTokens = output
		record.CacheReadTokens = cacheRead
		record.CacheCreateTokens = cacheWrite
		record.CreatedAt = parseSessionTimestamp(sessionStringValue(step["created_at"]))
		pending = append(pending, antigravityPendingRecord{Record: record, Step: index, Display: display})
	}
	if len(pending) == 0 {
		return nil, true
	}

	// 已有计时且模型已确定的步骤无需再打开原生库（与 cc-switch 的 steps_with_timing 一致）
	existing := map[string]existingSessionUsageRecord{}
	probe := make([]sessionUsageRecord, 0, len(pending))
	for _, item := range pending {
		probe = append(probe, item.Record)
	}
	if loaded, err := existingSessionUsageRecords(probe); err == nil {
		existing = loaded
	}
	wanted := make([]int64, 0, len(pending))
	for _, item := range pending {
		stored, found := existing[item.Record.SourceRecordID]
		needsModel := antigravityNeedsNativeModel(item.Display)
		if !found || stored.DurationSec <= 0 || (needsModel && antigravityIsUnresolvedModel(stored.Model)) {
			wanted = append(wanted, item.Step)
		}
	}

	generations := map[int64]antigravityGeneration{}
	nativeOK := true
	if len(wanted) > 0 {
		loaded, err := readAntigravityGenerations(antigravityNativeDBPath(transcript), wanted)
		if err != nil {
			nativeOK = false
		} else {
			generations = loaded
		}
	}

	records := make([]sessionUsageRecord, 0, len(pending))
	for _, item := range pending {
		generation := generations[item.Step]
		display := item.Display
		if display == "" {
			display = generation.Model
		}
		actual := display
		resolved := true
		switch {
		case antigravityIsOrchestrator(display):
			if generation.Model != "" {
				actual = generation.Model
			} else {
				resolved = false
			}
		case antigravityIsOrchestratorWorker(display):
			actual = "gemini-4-hydrogen"
			if generation.Model != "" && !antigravityIsUnresolvedWorkerTarget(generation.Model) {
				actual = generation.Model
			} else {
				resolved = false
			}
		}
		model := antigravityModelID(actual)
		if model == "" {
			model = "unknown"
			resolved = false
		}
		// 原生库这次没给出具体模型时，保留此前已解析出的具体模型，避免降级与重新计价
		if stored, found := existing[item.Record.SourceRecordID]; found && !resolved && !antigravityIsUnresolvedModel(stored.Model) {
			model = stored.Model
		}
		record := item.Record
		record.Model = model
		record.DurationSec = generation.LatencySec
		record.FirstTokenSec = generation.FirstTokenSec
		records = append(records, record)
	}
	return records, nativeOK
}

// antigravityNeedsNativeModel 显示名缺失或只代表编排模式/工作角色时，需要原生库给出实际模型
func antigravityNeedsNativeModel(display string) bool {
	return display == "" || antigravityIsOrchestrator(display) || antigravityIsOrchestratorWorker(display)
}

// antigravityIsUnresolvedModel 已入库的模型是否仍不具体（未知、编排模式名或工作角色别名）
func antigravityIsUnresolvedModel(model string) bool {
	return antigravityIsUnresolvedWorkerTarget(model)
}

// antigravityKnownModel 读取 step 自带的模型字段（unknown 视为缺失）
func antigravityKnownModel(step map[string]any) string {
	model := sessionStringValue(step["model"])
	if model == "" || strings.EqualFold(model, "unknown") {
		return ""
	}
	return model
}

// antigravityModelFromSettingsChange 从 "changed setting `Model Selection` from X to Y" 中取出 Y
func antigravityModelFromSettingsChange(content string) string {
	start := strings.Index(content, "Model Selection")
	if start < 0 {
		return ""
	}
	rest := content[start:]
	to := strings.Index(rest, " to ")
	if to < 0 {
		return ""
	}
	tail := rest[to+4:]
	// 模型名本身带点（Gemini 3.8），只把「句号 + 空格」视为句末
	end := len(tail)
	for _, marker := range []string{". ", "\n", "`"} {
		if position := strings.Index(tail, marker); position >= 0 {
			end = position
			break
		}
	}
	model := strings.TrimSpace(strings.Trim(strings.TrimSpace(tail[:end]), "."))
	if model == "" || strings.EqualFold(model, "none") {
		return ""
	}
	return model
}

// antigravityModelID 显示名转模型 ID：「Gemini 3.8 Flash (High)」→「gemini-3.8-flash-high」；已是 ID 的原样返回
func antigravityModelID(display string) string {
	display = strings.TrimSpace(display)
	if !strings.Contains(display, " ") {
		return display
	}
	value := strings.ToLower(display)
	value = strings.ReplaceAll(value, " (", "-")
	value = strings.NewReplacer("(", "", ")", "").Replace(value)
	return strings.Join(strings.Fields(value), "-")
}

// antigravityOrchestratorMode 编排模式只代表多模型模式，不代表单次响应实际使用的模型
func antigravityOrchestratorMode(model string) string {
	switch strings.ToLower(antigravityModelID(model)) {
	case "orchestrator-carbon", "gemini-pro-orchestrator":
		return "carbon"
	case "orchestrator-argon", "gemini-pro-orchestrator-argon", "gemini-pro-orchestrator-g1-pro":
		return "argon"
	case "orchestrator-neon", "gemini-pro-orchestrator-neon":
		return "neon"
	default:
		return ""
	}
}

func antigravityIsOrchestrator(model string) bool {
	return antigravityOrchestratorMode(model) != ""
}

// antigravityIsOrchestratorWorker 编排模式下工作模型的占位别名
func antigravityIsOrchestratorWorker(model string) bool {
	return strings.EqualFold(antigravityModelID(model), "gemini-flash-worker-agy")
}

// antigravityIsUnresolvedWorkerTarget 模式名、角色别名等无法确定具体模型的取值
func antigravityIsUnresolvedWorkerTarget(model string) bool {
	id := antigravityModelID(model)
	return id == "" || strings.EqualFold(id, "unknown") || antigravityIsOrchestrator(id) ||
		antigravityIsOrchestratorWorker(id) || id == "gemini-flash"
}

// readAntigravityGenerations 只读打开原生库，按 step 找到生成记录并解出模型与计时
// 库不存在视为无计时（不算失败）；被占用时立即返回错误，不阻塞同步
func readAntigravityGenerations(path string, steps []int64) (map[int64]antigravityGeneration, error) {
	generations := make(map[int64]antigravityGeneration, len(steps))
	if info, err := os.Stat(path); err != nil || !info.Mode().IsRegular() {
		return generations, nil
	}
	// 路径需按 URI 转义，否则目录名含 #、% 等字符时无法打开
	db, err := sql.Open("sqlite", sessionSQLiteReadOnlyURI(path)+"&_pragma=busy_timeout(0)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	for _, step := range steps {
		var metadata []byte
		err := db.QueryRow("SELECT metadata FROM steps WHERE idx = ? AND step_type = 15", step).Scan(&metadata)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			// 表结构不认识属于永久性差异，按无原生数据处理，避免每轮都重新解析
			if isNoSuchTableErr(err) {
				return generations, nil
			}
			return nil, err
		}
		generationIndex, ok := antigravityGenerationIndex(metadata)
		if !ok {
			continue
		}
		var data []byte
		err = db.QueryRow("SELECT data FROM gen_metadata WHERE idx = ?", generationIndex).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			if isNoSuchTableErr(err) {
				return generations, nil
			}
			return nil, err
		}
		generation := decodeAntigravityGeneration(data)
		if generation.Model != "" || generation.LatencySec > 0 {
			generations[step] = generation
		}
	}
	return generations, nil
}

// antigravityGenerationIndex steps.metadata 字段 20 → 3 为生成序号（为 0 时 protobuf 会省略该字段）
func antigravityGenerationIndex(metadata []byte) (int64, bool) {
	reference, ok := protoLastBytesField(metadata, 20)
	if !ok {
		return 0, false
	}
	index, _ := protoLastVarintField(reference, 3)
	return int64(index), true
}

// decodeAntigravityGeneration gen_metadata.data 字段 1 为响应元数据：
// 19 = 模型、21 = 分档模型 ID、11 = 首字等待时长、12 = 其余流式时长（均为 google.protobuf.Duration）
func decodeAntigravityGeneration(data []byte) antigravityGeneration {
	var generation antigravityGeneration
	response, ok := protoLastBytesField(data, 1)
	if !ok {
		return generation
	}
	base := ""
	if value, ok := protoLastBytesField(response, 19); ok {
		base = strings.TrimSpace(string(value))
	}
	tiered := ""
	if value, ok := protoLastBytesField(response, 21); ok {
		tiered = strings.TrimSpace(string(value))
	}
	orchestratorModel := ""
	switch base {
	case "gemini-pro-orchestrator":
		orchestratorModel = "gemini-4-carbon"
	case "gemini-pro-orchestrator-argon", "gemini-pro-orchestrator-g1-pro":
		orchestratorModel = "gemini-4-argon"
	case "gemini-pro-orchestrator-neon":
		orchestratorModel = "gemini-4-neon"
	case "gemini-flash-worker-agy":
		orchestratorModel = "gemini-4-hydrogen"
	}
	if orchestratorModel != "" {
		tieredID := antigravityModelID(tiered)
		if antigravityIsOrchestrator(tieredID) || tieredID == "gemini-flash" || tieredID == "gemini-flash-worker-agy" {
			tiered = ""
		}
	}
	for _, candidate := range []string{tiered, orchestratorModel, base} {
		if candidate != "" && !strings.EqualFold(candidate, "unknown") {
			generation.Model = candidate
			break
		}
	}

	firstToken, firstOK := protoDurationField(response, 11)
	streaming, streamOK := protoDurationField(response, 12)
	if firstOK && streamOK {
		latency := firstToken + streaming
		if latency > 0 {
			generation.FirstTokenSec = firstToken.Seconds()
			generation.LatencySec = latency.Seconds()
		}
	}
	return generation
}

// protoLastBytesField 在 protobuf 报文中查找指定字段号的长度分隔值（同字段重复出现时以最后一次为准）
func protoLastBytesField(message []byte, field protowire.Number) ([]byte, bool) {
	var result []byte
	found := false
	for len(message) > 0 {
		number, wireType, length := protowire.ConsumeTag(message)
		if length < 0 {
			return result, found
		}
		message = message[length:]
		if number == field && wireType == protowire.BytesType {
			value, size := protowire.ConsumeBytes(message)
			if size < 0 {
				return result, found
			}
			result, found = value, true
			message = message[size:]
			continue
		}
		size := protowire.ConsumeFieldValue(number, wireType, message)
		if size < 0 {
			return result, found
		}
		message = message[size:]
	}
	return result, found
}

// protoLastVarintField 在 protobuf 报文中查找指定字段号的 varint 值
func protoLastVarintField(message []byte, field protowire.Number) (uint64, bool) {
	var result uint64
	found := false
	for len(message) > 0 {
		number, wireType, length := protowire.ConsumeTag(message)
		if length < 0 {
			return result, found
		}
		message = message[length:]
		if number == field && wireType == protowire.VarintType {
			value, size := protowire.ConsumeVarint(message)
			if size < 0 {
				return result, found
			}
			result, found = value, true
			message = message[size:]
			continue
		}
		size := protowire.ConsumeFieldValue(number, wireType, message)
		if size < 0 {
			return result, found
		}
		message = message[size:]
	}
	return result, found
}

// protoDurationField 解出 google.protobuf.Duration（1 = seconds，2 = nanos），负值视为无效
func protoDurationField(message []byte, field protowire.Number) (time.Duration, bool) {
	value, ok := protoLastBytesField(message, field)
	if !ok {
		return 0, false
	}
	seconds, _ := protoLastVarintField(value, 1)
	nanos, _ := protoLastVarintField(value, 2)
	duration := time.Duration(int64(seconds))*time.Second + time.Duration(int32(nanos))
	if duration < 0 {
		return 0, false
	}
	return duration, true
}
