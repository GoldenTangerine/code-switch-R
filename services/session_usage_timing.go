/**
 * @name: 会话用量耗时估算
 * @Descripttion: 根据 Claude、Codex 会话日志中的时间戳估算单次请求的耗时与首字时间。
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-11-20 14:00:00
 * @LastEditTime: 2026-11-20 14:00:00
 * @FilePath: services/session_usage_timing.go
 */

package services

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
)

const (
	// sessionMinPlausibleLatencyMs 估算耗时短于该值视为起点取错，不记计时
	sessionMinPlausibleLatencyMs int64 = 100
	// sessionMaxPlausibleLatencyMs 估算耗时长于该值多半夹了休眠或重试等待，不记计时
	sessionMaxPlausibleLatencyMs int64 = 60 * 60 * 1000
	// claudeChainSeedBytes 增量读取时向游标前回看的字节数：回复的起点行（用户消息、工具结果）常在游标之前
	claudeChainSeedBytes int64 = 256 * 1024
	// claudeMaxChainHops 沿父链向上查找请求起点的最大步数
	claudeMaxChainHops = 32
)

// estimateSessionLatencyMs 两个时间戳相减估算耗时（含首字等待），超出合理范围返回 false
func estimateSessionLatencyMs(startMs int64, endMs int64) (int64, bool) {
	latency := endMs - startMs
	if latency < sessionMinPlausibleLatencyMs || latency > sessionMaxPlausibleLatencyMs {
		return 0, false
	}
	return latency, true
}

// estimateSessionFirstTokenMs 首字时间 = 开始输出时刻 - 起点，须落在 (0, 耗时] 之间
func estimateSessionFirstTokenMs(startMs int64, firstOutputMs int64, latencyMs int64) (int64, bool) {
	firstToken := firstOutputMs - startMs
	if firstToken <= 0 || firstToken > latencyMs {
		return 0, false
	}
	return firstToken, true
}

// sessionTimestampMs 解析 RFC3339 时间戳为毫秒
func sessionTimestampMs(value string) (int64, bool) {
	parsed := parseSessionTimestamp(value)
	if parsed.IsZero() {
		return 0, false
	}
	return parsed.UnixMilli(), true
}

// claudeChainNode 对话链上的一行，只保留估算请求起点需要的信息
type claudeChainNode struct {
	Parent      string
	TimestampMs int64
	HasTime     bool
	// IsAttachment attachment 行的时间戳不可靠（可能在回复开始后才补记），找起点时跳过
	IsAttachment bool
}

// claudeMessageTiming 同一 message.id 下所有内容块的计时线索
type claudeMessageTiming struct {
	// FirstParent 最先出现的内容块的父行，请求起点沿它向上找
	FirstParent string
	// StartsAtFirstBlock 最先出现的块是否为回复第一块；不是则起点会取晚、速度偏高，宁可不估
	StartsAtFirstBlock bool
	// FirstOutputMs 第一块为思考块时，用块写完时刻减去 thinkingDurationMs 得到开始输出时刻
	FirstOutputMs  int64
	HasFirstOutput bool
	// EndMs 各块写完时刻中最晚的一个，即回复结束
	EndMs  int64
	HasEnd bool
}

// claudeSessionTimingTracker 收集 Claude 会话的对话链与回复计时
type claudeSessionTimingTracker struct {
	chain   map[string]claudeChainNode
	timings map[string]*claudeMessageTiming
}

func newClaudeSessionTimingTracker() *claudeSessionTimingTracker {
	return &claudeSessionTimingTracker{
		chain:   make(map[string]claudeChainNode, 64),
		timings: make(map[string]*claudeMessageTiming, 16),
	}
}

// Observe 记录一行日志：所有带 uuid 的行进入对话链，assistant 行进入回复计时
func (tracker *claudeSessionTimingTracker) Observe(value map[string]any) {
	uuid := sessionStringValue(value["uuid"])
	timestampMs, hasTime := sessionTimestampMs(sessionStringValue(value["timestamp"]))
	if uuid != "" {
		tracker.chain[uuid] = claudeChainNode{
			Parent:       sessionStringValue(value["parentUuid"]),
			TimestampMs:  timestampMs,
			HasTime:      hasTime,
			IsAttachment: sessionStringValue(value["type"]) == "attachment",
		}
	}
	if sessionStringValue(value["type"]) != "assistant" {
		return
	}
	message := mapValue(value["message"])
	messageID := firstString(message, "id")
	if messageID == "" {
		return
	}
	timing, exists := tracker.timings[messageID]
	if !exists {
		// 旧版日志没有 apiBlockIndex，按第一块处理
		startsAtFirstBlock := true
		if rawIndex, ok := value["apiBlockIndex"]; ok {
			index, isNumber := rawIndex.(float64)
			startsAtFirstBlock = isNumber && index == 0
		}
		timing = &claudeMessageTiming{
			FirstParent:        sessionStringValue(value["parentUuid"]),
			StartsAtFirstBlock: startsAtFirstBlock,
		}
		if startsAtFirstBlock && hasTime {
			if startMs, ok := claudeThinkingBlockStartMs(value, message, timestampMs); ok {
				timing.FirstOutputMs = startMs
				timing.HasFirstOutput = true
			}
		}
		tracker.timings[messageID] = timing
	}
	if hasTime && (!timing.HasEnd || timestampMs > timing.EndMs) {
		timing.EndMs = timestampMs
		timing.HasEnd = true
	}
}

// claudeThinkingBlockStartMs 第一块为思考块时，开始输出时刻 = 块写完时刻 - thinkingDurationMs
func claudeThinkingBlockStartMs(value map[string]any, message map[string]any, blockEndMs int64) (int64, bool) {
	durationMs, ok := value["thinkingDurationMs"].(float64)
	if !ok || durationMs <= 0 {
		return 0, false
	}
	content, _ := message["content"].([]any)
	if len(content) == 0 || sessionStringValue(mapValue(content[0])["type"]) != "thinking" {
		return 0, false
	}
	return blockEndMs - int64(durationMs), true
}

// resolveStartMs 请求起点：从回复第一块的父行向上，跳过 attachment，取第一个其他行的时间戳
func (tracker *claudeSessionTimingTracker) resolveStartMs(parent string) (int64, bool) {
	cursor := parent
	for hop := 0; hop < claudeMaxChainHops && cursor != ""; hop++ {
		node, ok := tracker.chain[cursor]
		if !ok {
			return 0, false
		}
		if !node.IsAttachment {
			return node.TimestampMs, node.HasTime
		}
		cursor = node.Parent
	}
	return 0, false
}

// Estimate 估算指定回复的耗时与首字（秒）；仅对已写完（有 stop_reason）且看得到第一块的回复估算
func (tracker *claudeSessionTimingTracker) Estimate(messageID string, hasStopReason bool) (float64, float64) {
	timing, ok := tracker.timings[messageID]
	if !ok || !hasStopReason || !timing.StartsAtFirstBlock || !timing.HasEnd {
		return 0, 0
	}
	startMs, ok := tracker.resolveStartMs(timing.FirstParent)
	if !ok {
		return 0, 0
	}
	latencyMs, ok := estimateSessionLatencyMs(startMs, timing.EndMs)
	if !ok {
		return 0, 0
	}
	firstTokenSec := 0.0
	if timing.HasFirstOutput {
		if firstTokenMs, ok := estimateSessionFirstTokenMs(startMs, timing.FirstOutputMs, latencyMs); ok {
			firstTokenSec = float64(firstTokenMs) / 1000
		}
	}
	return float64(latencyMs) / 1000, firstTokenSec
}

// SeedBefore 读取游标前一小段，只记录对话链与回复计时（不导入），回看失败不影响导入
func (tracker *claudeSessionTimingTracker) SeedBefore(path string, offset int64) {
	// 注意：包内 min 被 logservice.go 定义为 int 版本，这里手动截断
	length := max(offset, 0)
	if length > claudeChainSeedBytes {
		length = claudeChainSeedBytes
	}
	if length == 0 {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	window := make([]byte, length)
	if _, err := file.ReadAt(window, offset-length); err != nil && err != io.EOF {
		return
	}
	lines := bytes.Split(window, []byte{'\n'})
	if offset > length && len(lines) > 0 {
		// 窗口不是从文件头开始，第一段多半是半行
		lines = lines[1:]
	}
	for _, line := range lines {
		var value map[string]any
		if json.Unmarshal(bytes.TrimSpace(line), &value) == nil {
			tracker.Observe(value)
		}
	}
}

// codexSameFlushSlackMs 两行时间戳相差不超过该值视为同一批写出（真实请求不可能这么快）
const codexSameFlushSlackMs int64 = 100

// codexUsageRecord 新版 Codex 在响应结束时写出的 token_usage_record
type codexUsageRecord struct {
	TimestampMs int64                 `json:"ts"`
	Usage       codexCumulativeTokens `json:"usage"`
}

// codexRequestTimer 按事件顺序估算 Codex 每次请求的耗时与首字，状态随解析游标持久化：
//   - 结束：优先用 token_usage_record；只有 token_count 且它要等工具跑完才写时，改用最后一个输出项的时刻
//   - 开始：本次请求第一个输出项之前最近的起点行（上一次 token_count、工具结果、用户消息、turn_context）
//   - 首字：新版 item_completed 的 started_at_ms 中最早的一项减去起点
//   - task_started / turn_aborted 时作废未结算的请求，避免起点被下一次请求沿用
type codexRequestTimer struct {
	LastBoundaryMs               int64             `json:"last_boundary_ms,omitempty"`
	LastTokenCountMs             int64             `json:"last_token_count_ms,omitempty"`
	RequestStartMs               int64             `json:"request_start_ms,omitempty"`
	LastModelOutputMs            int64             `json:"last_model_output_ms,omitempty"`
	ToolOutputAfterModelOutputMs int64             `json:"tool_output_after_model_output_ms,omitempty"`
	UsageRecord                  *codexUsageRecord `json:"usage_record,omitempty"`
	FirstOutputMs                int64             `json:"first_output_ms,omitempty"`
}

// ObserveLine 按行类型推进计时状态（token_count 行由 Finish 处理）
func (timer *codexRequestTimer) ObserveLine(value map[string]any) {
	timestampMs, ok := sessionTimestampMs(sessionStringValue(value["timestamp"]))
	if !ok {
		return
	}
	payload := mapValue(value["payload"])
	payloadType := sessionStringValue(payload["type"])
	switch sessionStringValue(value["type"]) {
	case "turn_context":
		timer.markBoundary(timestampMs)
	case "token_usage_record":
		usage := mapValue(payload["usage"])
		if len(usage) == 0 {
			timer.UsageRecord = nil
			return
		}
		timer.UsageRecord = &codexUsageRecord{TimestampMs: timestampMs, Usage: parseCodexCumulativeTokens(usage)}
	case "response_item":
		switch {
		case payloadType == "message" && sessionStringValue(payload["role"]) == "assistant":
			timer.markModelOutput(timestampMs)
		case payloadType == "message":
			timer.markBoundary(timestampMs)
		case payloadType == "reasoning" || strings.HasSuffix(payloadType, "_call"):
			timer.markModelOutput(timestampMs)
		case strings.HasSuffix(payloadType, "_output"):
			timer.markBoundary(timestampMs)
			if timer.LastModelOutputMs > 0 {
				timer.ToolOutputAfterModelOutputMs = timestampMs
			}
		}
	case "event_msg":
		switch payloadType {
		case "task_started", "turn_aborted":
			*timer = codexRequestTimer{
				LastBoundaryMs:   max(timer.LastBoundaryMs, timestampMs),
				LastTokenCountMs: timer.LastTokenCountMs,
			}
		case "item_completed":
			itemType := sessionStringValue(mapValue(payload["item"])["type"])
			if itemType != "Reasoning" && itemType != "AgentMessage" {
				return
			}
			startedAt, ok := payload["started_at_ms"].(float64)
			if !ok || startedAt <= 0 {
				return
			}
			if timer.FirstOutputMs == 0 || int64(startedAt) < timer.FirstOutputMs {
				timer.FirstOutputMs = int64(startedAt)
			}
		}
	}
}

func (timer *codexRequestTimer) markBoundary(timestampMs int64) {
	timer.LastBoundaryMs = max(timer.LastBoundaryMs, timestampMs)
}

func (timer *codexRequestTimer) markModelOutput(timestampMs int64) {
	// 旧布局会把上一次响应的输出项补写在其 token_count 之后，这些项不属于下一次请求
	if timer.RequestStartMs == 0 && timer.LastTokenCountMs > 0 && timestampMs-timer.LastTokenCountMs <= codexSameFlushSlackMs {
		return
	}
	if timer.RequestStartMs == 0 {
		timer.RequestStartMs = timer.LastBoundaryMs
	}
	timer.LastModelOutputMs = timestampMs
	timer.ToolOutputAfterModelOutputMs = 0
}

// Finish 遇到有用量的 token_count 时结算本次请求，返回耗时与首字（秒），并把它记为下一次请求的起点
// last 为本次请求自身的用量，用来确认 token_usage_record 指的是同一次请求
func (timer *codexRequestTimer) Finish(tokenCountMs int64, last *codexCumulativeTokens) (float64, float64) {
	startMs := timer.RequestStartMs
	if startMs == 0 {
		startMs = timer.LastBoundaryMs
	}
	firstOutputMs := timer.FirstOutputMs
	recordEndMs := int64(0)
	if record := timer.UsageRecord; record != nil {
		if last == nil || (record.Usage.Input == last.Input && record.Usage.CachedInput == last.CachedInput && record.Usage.Output == last.Output) {
			recordEndMs = record.TimestampMs
		}
	}
	waitedForTools := false
	if timer.ToolOutputAfterModelOutputMs > 0 {
		waitedForTools = tokenCountMs == 0 || tokenCountMs-timer.ToolOutputAfterModelOutputMs <= codexSameFlushSlackMs
	}
	endMs := recordEndMs
	if endMs == 0 {
		if waitedForTools {
			endMs = timer.LastModelOutputMs
		} else {
			endMs = tokenCountMs
		}
	}

	*timer = codexRequestTimer{
		LastBoundaryMs:   max(timer.LastBoundaryMs, tokenCountMs),
		LastTokenCountMs: tokenCountMs,
	}
	if startMs <= 0 || endMs <= 0 {
		return 0, 0
	}
	latencyMs, ok := estimateSessionLatencyMs(startMs, endMs)
	if !ok {
		return 0, 0
	}
	firstTokenSec := 0.0
	if firstOutputMs > 0 {
		if firstTokenMs, ok := estimateSessionFirstTokenMs(startMs, firstOutputMs, latencyMs); ok {
			firstTokenSec = float64(firstTokenMs) / 1000
		}
	}
	return float64(latencyMs) / 1000, firstTokenSec
}
