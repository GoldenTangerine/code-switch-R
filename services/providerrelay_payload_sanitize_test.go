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

func TestSanitizeRequestLogPayload_RedactsPlainTextCredentials(t *testing.T) {
	input := "Authorization: Basic basic-secret\nx-api-key: api-secret\npassword='password-secret'"
	output := sanitizeRequestLogPayload(input)

	for _, secret := range []string{"basic-secret", "api-secret", "password-secret"} {
		if strings.Contains(output, secret) {
			t.Fatalf("纯文本敏感值未脱敏 %q: %s", secret, output)
		}
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

func TestCaptureRequestLogRequestBody_AlwaysRedactsSessionIdentifiers(t *testing.T) {
	body := []byte(`{"metadata":{"user_id":"user-secret","thread_id":"thread-secret"},"messages":[{"tool_calls":[{"id":"call-secret","type":"function","function":{"name":"x"}}]},{"role":"tool","tool_call_id":"call-secret","content":"ok"}]}`)

	rawLog := &ReqeustLog{
		CapturePayload:  true,
		SanitizePayload: false,
	}
	captureRequestLogRequestBody(rawLog, body)
	for _, secret := range []string{"user-secret", "thread-secret", "call-secret"} {
		if strings.Contains(rawLog.RequestBody, secret) {
			t.Fatalf("关闭通用脱敏时仍必须隐藏会话标识 %q: %s", secret, rawLog.RequestBody)
		}
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
