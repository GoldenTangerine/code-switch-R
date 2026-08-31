/**
 * @name: 代理热路径性能基线
 * @Descripttion: 验证并测量代理请求读取、响应转发、负载捕获和脱敏的当前行为与资源成本
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 15:43:27
 * @LastEditTime: 2026-08-31 15:43:27
 * @FilePath: services/providerrelay_hotpath_benchmark_test.go
 */

package services

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var (
	relayBenchmarkBytesSink      []byte
	relayBenchmarkStringSink     string
	relayBenchmarkReadCloserSink io.ReadCloser
	relayBenchmarkIntSink        int64
	relayBenchmarkBoolSink       bool
)

type relayBenchmarkPayloadMode struct {
	name     string
	capture  bool
	sanitize bool
}

type relayBenchmarkForwardMode struct {
	name     string
	useHook  bool
	capture  bool
	sanitize bool
}

func relayBenchmarkPayloadSizes() []struct {
	name string
	size int
} {
	return []struct {
		name string
		size int
	}{
		{name: "1KB", size: 1 << 10},
		{name: "64KB", size: 64 << 10},
		{name: "1MB", size: 1 << 20},
		{name: "near_8MB_limit", size: requestLogPayloadMaxBytes + 4<<10},
	}
}

func relayBenchmarkPayloadModes() []relayBenchmarkPayloadMode {
	return []relayBenchmarkPayloadMode{
		{name: "capture_off", capture: false, sanitize: true},
		{name: "capture_raw", capture: true, sanitize: false},
		{name: "capture_sanitized", capture: true, sanitize: true},
	}
}

func buildRelayBenchmarkJSONPayload(size int, sensitive bool) []byte {
	prefix := `{"model":"gpt-5.3-codex","input":"`
	if sensitive {
		prefix = `{"model":"gpt-5.3-codex","api_key":"relay-secret","input":"`
	}
	suffix := `"}`
	padding := size - len(prefix) - len(suffix)
	if padding < 0 {
		padding = 0
	}
	return []byte(prefix + strings.Repeat("x", padding) + suffix)
}

func buildRelayBenchmarkSSEPayload(minimumSize int) []byte {
	line := `data: {"type":"response.output_text.delta","delta":"hello","api_key":"relay-secret"}` + "\n\n"
	var payload strings.Builder
	payload.Grow(minimumSize + len(line))
	for payload.Len() < minimumSize {
		payload.WriteString(line)
	}
	return []byte(payload.String())
}

func executeRelayBenchmarkPayloadCapture(payload []byte, mode relayBenchmarkPayloadMode) *ReqeustLog {
	requestLog := &ReqeustLog{
		CapturePayload:   mode.capture,
		SanitizePayload:  mode.sanitize,
		requestBodyBytes: payload,
	}
	appendRequestLogResponseBody(requestLog, payload)
	prepareRequestLogPayloadForPersistence(requestLog)
	return requestLog
}

func assertRelayBenchmarkPayloadMode(tb testing.TB, requestLog *ReqeustLog, mode relayBenchmarkPayloadMode) {
	tb.Helper()
	if !mode.capture {
		if requestLog.PayloadCaptured || requestLog.PayloadBytes != 0 || requestLog.RequestBody != "" || requestLog.ResponseBody != "" {
			tb.Fatalf("关闭捕获后的 payload 状态异常: %#v", requestLog)
		}
		return
	}
	if !requestLog.PayloadCaptured || requestLog.PayloadBytes != int64(len(requestLog.RequestBody)+len(requestLog.ResponseBody)) {
		tb.Fatalf("启用捕获后的 payload 统计异常: %#v", requestLog)
	}
	combined := requestLog.RequestBody + requestLog.ResponseBody
	if mode.sanitize {
		if strings.Contains(combined, "relay-secret") || !strings.Contains(combined, requestLogPayloadRedactedValue) {
			tb.Fatalf("启用脱敏后的 payload 异常")
		}
		return
	}
	if !strings.Contains(combined, "relay-secret") {
		tb.Fatalf("关闭脱敏时 payload 不应改变")
	}
}

func executeRelayBenchmarkForward(payload []byte, contentType string, isStream bool, mode relayBenchmarkForwardMode) (int64, *httptest.ResponseRecorder, *ReqeustLog, error) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{contentType}},
		Body:       io.NopCloser(bytes.NewReader(payload)),
	}
	writer := httptest.NewRecorder()
	if !mode.useHook {
		written, err := forwardRelayResponse(response, writer, isStream)
		return written, writer, nil, err
	}

	requestLog := &ReqeustLog{
		IsStream:        isStream,
		CapturePayload:  mode.capture,
		SanitizePayload: mode.sanitize,
	}
	written, err := forwardRelayResponse(response, writer, isStream, ReqeustLogHook(nil, "codex", requestLog))
	prepareRequestLogPayloadForPersistence(requestLog)
	return written, writer, requestLog, err
}

func assertRelayBenchmarkForward(tb testing.TB, payload []byte, written int64, writer *httptest.ResponseRecorder, requestLog *ReqeustLog, mode relayBenchmarkForwardMode) {
	tb.Helper()
	if written != int64(len(payload)) || writer.Code != http.StatusOK || !bytes.Equal(writer.Body.Bytes(), payload) {
		tb.Fatalf("代理转发结果不一致: written=%d status=%d body=%d want=%d", written, writer.Code, writer.Body.Len(), len(payload))
	}
	if !mode.useHook {
		if requestLog != nil {
			tb.Fatal("无 Hook 场景不应创建请求日志")
		}
		return
	}
	assertRelayBenchmarkPayloadMode(tb, requestLog, relayBenchmarkPayloadMode{
		name:     mode.name,
		capture:  mode.capture,
		sanitize: mode.sanitize,
	})
}

func TestRelayHotPathPayloadCaptureMatrix(t *testing.T) {
	payload := buildRelayBenchmarkJSONPayload(1<<10, true)
	for _, mode := range relayBenchmarkPayloadModes() {
		t.Run(mode.name, func(t *testing.T) {
			requestLog := executeRelayBenchmarkPayloadCapture(payload, mode)
			assertRelayBenchmarkPayloadMode(t, requestLog, mode)
		})
	}
}

func TestRelayHotPathPayloadCaptureTruncatesAtLimit(t *testing.T) {
	payload := buildRelayBenchmarkJSONPayload(requestLogPayloadMaxBytes+4<<10, true)
	mode := relayBenchmarkPayloadMode{name: "capture_raw", capture: true, sanitize: false}
	requestLog := executeRelayBenchmarkPayloadCapture(payload, mode)

	assertRelayBenchmarkPayloadMode(t, requestLog, mode)
	if !requestLog.RequestBodyTruncated || !requestLog.ResponseBodyTruncated {
		t.Fatalf("超过上限的请求和响应均应标记截断: %#v", requestLog)
	}
	if len(requestLog.RequestBody) != requestLogPayloadMaxBytes || len(requestLog.ResponseBody) != requestLogPayloadMaxBytes {
		t.Fatalf("截断长度 request=%d response=%d want=%d", len(requestLog.RequestBody), len(requestLog.ResponseBody), requestLogPayloadMaxBytes)
	}
}

func TestRelayHotPathForwardMatrix(t *testing.T) {
	tests := []struct {
		name        string
		payload     []byte
		contentType string
		isStream    bool
		modes       []relayBenchmarkForwardMode
	}{
		{
			name:        "non_stream",
			payload:     buildRelayBenchmarkJSONPayload(1<<10, false),
			contentType: "application/json",
			modes: []relayBenchmarkForwardMode{
				{name: "copy_no_hook", useHook: false},
				{name: "hook_capture_off", useHook: true, capture: false, sanitize: true},
			},
		},
		{
			name:        "sse",
			payload:     buildRelayBenchmarkSSEPayload(4 << 10),
			contentType: "text/event-stream",
			isStream:    true,
			modes: []relayBenchmarkForwardMode{
				{name: "no_hook", useHook: false},
				{name: "capture_off", useHook: true, capture: false, sanitize: true},
				{name: "capture_raw", useHook: true, capture: true, sanitize: false},
				{name: "capture_sanitized", useHook: true, capture: true, sanitize: true},
			},
		},
	}

	for _, tt := range tests {
		for _, mode := range tt.modes {
			t.Run(tt.name+"/"+mode.name, func(t *testing.T) {
				written, writer, requestLog, err := executeRelayBenchmarkForward(tt.payload, tt.contentType, tt.isStream, mode)
				if err != nil {
					t.Fatal(err)
				}
				assertRelayBenchmarkForward(t, tt.payload, written, writer, requestLog, mode)
			})
		}
	}
}

func BenchmarkRelayRequestRead(b *testing.B) {
	for _, size := range relayBenchmarkPayloadSizes() {
		payload := buildRelayBenchmarkJSONPayload(size.size, false)
		b.Run(size.name, func(b *testing.B) {
			body := io.NopCloser(bytes.NewReader(payload))
			got, err := io.ReadAll(body)
			_ = body.Close()
			if err != nil || !bytes.Equal(got, payload) {
				b.Fatalf("请求读取预检失败: err=%v bytes=%d", err, len(got))
			}

			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				body := io.NopCloser(bytes.NewReader(payload))
				data, err := io.ReadAll(body)
				_ = body.Close()
				if err != nil {
					b.Fatal(err)
				}
				relayBenchmarkBytesSink = data
				relayBenchmarkReadCloserSink = io.NopCloser(bytes.NewReader(data))
			}
		})
	}
}

func BenchmarkRelayPayloadCapture(b *testing.B) {
	for _, size := range relayBenchmarkPayloadSizes() {
		payload := buildRelayBenchmarkJSONPayload(size.size, true)
		for _, mode := range relayBenchmarkPayloadModes() {
			b.Run(size.name+"/"+mode.name, func(b *testing.B) {
				b.ReportAllocs()
				if mode.capture {
					b.SetBytes(int64(len(payload) * 2))
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					requestLog := executeRelayBenchmarkPayloadCapture(payload, mode)
					relayBenchmarkStringSink = requestLog.ResponseBody
					relayBenchmarkIntSink = requestLog.PayloadBytes
				}
			})
		}
	}
}

func BenchmarkRelayPayloadSanitization(b *testing.B) {
	const payloadSize = 1 << 20
	for _, test := range []struct {
		name      string
		sensitive bool
	}{
		{name: "clean_1MB", sensitive: false},
		{name: "sensitive_1MB", sensitive: true},
	} {
		payload := string(buildRelayBenchmarkJSONPayload(payloadSize, test.sensitive))
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				relayBenchmarkStringSink = sanitizeRequestLogPayload(payload)
			}
		})
	}
}

func BenchmarkRelayPayloadQuickChecks(b *testing.B) {
	payload := string(buildRelayBenchmarkJSONPayload(1<<20, false))
	b.Run("session_identifiers_clean_1MB", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			relayBenchmarkStringSink = redactRequestLogSessionIdentifiers(payload)
		}
	})
	b.Run("sensitive_keywords_clean_1MB", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(payload)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			relayBenchmarkBoolSink = requestLogPayloadMayContainSensitiveKeyword(payload)
		}
	})
}

func BenchmarkRelayNonStreamResponse(b *testing.B) {
	modes := []relayBenchmarkForwardMode{
		{name: "copy_no_hook", useHook: false},
		{name: "hook_capture_off", useHook: true, capture: false, sanitize: true},
	}
	for _, size := range relayBenchmarkPayloadSizes() {
		payload := buildRelayBenchmarkJSONPayload(size.size, false)
		for _, mode := range modes {
			b.Run(size.name+"/"+mode.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(payload)))
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					written, writer, _, err := executeRelayBenchmarkForward(payload, "application/json", false, mode)
					if err != nil {
						b.Fatal(err)
					}
					relayBenchmarkIntSink = written
					relayBenchmarkBytesSink = writer.Body.Bytes()
				}
			})
		}
	}
}

func BenchmarkRelaySSEHook(b *testing.B) {
	payload := buildRelayBenchmarkSSEPayload(64 << 10)
	modes := []relayBenchmarkForwardMode{
		{name: "no_hook", useHook: false},
		{name: "capture_off", useHook: true, capture: false, sanitize: true},
		{name: "capture_raw", useHook: true, capture: true, sanitize: false},
		{name: "capture_sanitized", useHook: true, capture: true, sanitize: true},
	}
	for _, mode := range modes {
		b.Run(mode.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				written, writer, requestLog, err := executeRelayBenchmarkForward(payload, "text/event-stream", true, mode)
				if err != nil {
					b.Fatal(err)
				}
				relayBenchmarkIntSink = written
				relayBenchmarkBytesSink = writer.Body.Bytes()
				if requestLog != nil {
					relayBenchmarkStringSink = requestLog.ResponseBody
				}
			}
		})
	}
}
