package services

import (
	"strings"
	"testing"
)

func TestSanitizeRequestLogPayload_RedactsSensitiveFields(t *testing.T) {
	input := `{"api_key":"sk-live-123","authorization":"Bearer abcxyz","password":"p@ss","safe":"ok"}`
	output := sanitizeRequestLogPayload(input)

	if strings.Contains(output, "sk-live-123") || strings.Contains(output, "abcxyz") || strings.Contains(output, "p@ss") {
		t.Fatalf("敏感值未被完整脱敏: %s", output)
	}
	if !strings.Contains(output, requestLogPayloadRedactedValue) {
		t.Fatalf("脱敏结果缺少占位值: %s", output)
	}
	if !strings.Contains(output, `"safe":"ok"`) {
		t.Fatalf("非敏感字段被误伤: %s", output)
	}
}

func TestSanitizeRequestLogPayload_LeavesCleanPayloadUntouched(t *testing.T) {
	input := `{"model":"gpt-5.3-codex","stream":true}`
	output := sanitizeRequestLogPayload(input)
	if output != input {
		t.Fatalf("无敏感字段 payload 不应被修改，got=%s", output)
	}
}

func TestCaptureRequestLogRequestBody_RespectsSanitizeSwitch(t *testing.T) {
	body := []byte(`{"api_key":"secret-1","content":"hi"}`)

	sanitizedLog := &ReqeustLog{
		CapturePayload:  true,
		SanitizePayload: true,
	}
	captureRequestLogRequestBody(sanitizedLog, body)
	if strings.Contains(sanitizedLog.RequestBody, "secret-1") {
		t.Fatalf("启用脱敏时 request body 仍包含明文: %s", sanitizedLog.RequestBody)
	}

	rawLog := &ReqeustLog{
		CapturePayload:  true,
		SanitizePayload: false,
	}
	captureRequestLogRequestBody(rawLog, body)
	if !strings.Contains(rawLog.RequestBody, "secret-1") {
		t.Fatalf("关闭脱敏时 request body 不应被修改: %s", rawLog.RequestBody)
	}
}

func TestMaterializeRequestLogResponseBody_RespectsSanitizeSwitch(t *testing.T) {
	chunk := []byte(`{"authorization":"Bearer xyz-token","ok":true}`)

	sanitizedLog := &ReqeustLog{
		CapturePayload:  true,
		SanitizePayload: true,
	}
	appendRequestLogResponseBody(sanitizedLog, chunk)
	materializeRequestLogResponseBody(sanitizedLog)
	if strings.Contains(sanitizedLog.ResponseBody, "xyz-token") {
		t.Fatalf("启用脱敏时 response body 仍包含明文: %s", sanitizedLog.ResponseBody)
	}

	rawLog := &ReqeustLog{
		CapturePayload:  true,
		SanitizePayload: false,
	}
	appendRequestLogResponseBody(rawLog, chunk)
	materializeRequestLogResponseBody(rawLog)
	if !strings.Contains(rawLog.ResponseBody, "xyz-token") {
		t.Fatalf("关闭脱敏时 response body 不应被修改: %s", rawLog.ResponseBody)
	}
}

func TestIsRequestLogPayloadSanitizationEnabled_DefaultTrueWithoutSettings(t *testing.T) {
	prs := &ProviderRelayService{}
	if !prs.isRequestLogPayloadSanitizationEnabled() {
		t.Fatalf("未注入 appSettings 时，payload 脱敏应默认开启")
	}
}
