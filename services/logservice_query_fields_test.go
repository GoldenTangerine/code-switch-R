package services

import (
	"testing"

	"github.com/daodao97/xgo/xdb"
)

func TestRequestLogListSelectFields_ExcludePayloadColumns(t *testing.T) {
	t.Helper()
	forbidden := map[string]struct{}{
		"request_body":            {},
		"response_body":           {},
		"request_body_truncated":  {},
		"response_body_truncated": {},
	}
	for _, field := range requestLogListSelectFields {
		if _, ok := forbidden[field]; ok {
			t.Fatalf("列表查询字段不应包含 payload 列: %s", field)
		}
	}
}

func TestRequestLogListSelectFieldsIncludePerformanceColumns(t *testing.T) {
	required := map[string]bool{
		"proxy_prepare_ms":      false,
		"dns_ms":                false,
		"connect_ms":            false,
		"tls_ms":                false,
		"upstream_ttfb_ms":      false,
		"proxy_stream_delay_ms": false,
		"connection_reused":     false,
	}
	for _, field := range requestLogListSelectFields {
		if _, ok := required[field]; ok {
			required[field] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Fatalf("日志列表缺少性能字段: %s", field)
		}
	}
}

func TestRequestLogListSelectFieldsIncludeModelRouteColumns(t *testing.T) {
	required := map[string]bool{
		"mapped_model":          false,
		"model_mapping_pattern": false,
		"model_mapping_target":  false,
		"model_override":        false,
		"model_route_captured":  false,
	}
	for _, field := range requestLogListSelectFields {
		if _, ok := required[field]; ok {
			required[field] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Fatalf("日志列表缺少模型路由字段: %s", field)
		}
	}
}

func TestRequestLogListSelectFieldsIncludeStreamDiagnosticColumns(t *testing.T) {
	required := map[string]bool{
		"stream_last_event":           false,
		"stream_terminal_event":       false,
		"stream_error_kind":           false,
		"stream_compaction_requested": false,
		"stream_compaction_observed":  false,
		"stream_bytes":                false,
		"upstream_protocol":           false,
	}
	for _, field := range requestLogListSelectFields {
		if _, ok := required[field]; ok {
			required[field] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Fatalf("日志列表缺少流诊断字段: %s", field)
		}
	}
}

func TestBuildRequestLogListMapsStreamDiagnostics(t *testing.T) {
	logs := buildRequestLogList([]xdb.Record{{
		"id":                          int64(7),
		"is_stream":                   1,
		"stream_last_event":           "response.output_item.done",
		"stream_terminal_event":       "",
		"stream_error_kind":           "missing_terminal",
		"stream_compaction_requested": 1,
		"stream_compaction_observed":  1,
		"stream_bytes":                int64(2048),
		"upstream_protocol":           "HTTP/2.0",
	}}, nil)

	if len(logs) != 1 {
		t.Fatalf("日志数量 = %d，期望 1", len(logs))
	}
	got := logs[0]
	if got.StreamLastEvent != "response.output_item.done" || got.StreamErrorKind != "missing_terminal" {
		t.Fatalf("流事件映射错误: %#v", got)
	}
	if !got.StreamCompactionRequested || !got.StreamCompactionObserved || got.StreamBytes != 2048 || got.UpstreamProtocol != "HTTP/2.0" {
		t.Fatalf("流诊断映射错误: %#v", got)
	}
}

func TestRequestLogPayloadDetailSelectFields_IncludePayloadColumns(t *testing.T) {
	t.Helper()
	required := map[string]bool{
		"id":                      false,
		"request_body":            false,
		"response_body":           false,
		"request_body_truncated":  false,
		"response_body_truncated": false,
	}
	for _, field := range requestLogPayloadDetailSelectFields {
		if _, ok := required[field]; ok {
			required[field] = true
		}
	}
	for field, found := range required {
		if !found {
			t.Fatalf("payload 详情字段缺失: %s", field)
		}
	}
}

func TestRequestLogFailureListSelectFields_IncludeResponsePayloadColumnsOnly(t *testing.T) {
	t.Helper()
	required := map[string]bool{
		"response_body":           false,
		"response_body_truncated": false,
	}
	forbidden := map[string]struct{}{
		"request_body":           {},
		"request_body_truncated": {},
	}

	for _, field := range requestLogFailureListSelectFields {
		if _, ok := required[field]; ok {
			required[field] = true
		}
		if _, ok := forbidden[field]; ok {
			t.Fatalf("失败日志列表字段不应包含 request payload 列: %s", field)
		}
	}

	for field, found := range required {
		if !found {
			t.Fatalf("失败日志列表字段缺失: %s", field)
		}
	}
}
