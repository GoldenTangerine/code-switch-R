package services

import "testing"

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
