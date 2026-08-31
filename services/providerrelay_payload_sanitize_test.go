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

func TestSanitizeRequestLogPayload_ByteExactCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "json sensitive strings",
			input: `{"API-Key":"sk","authorization":"Bearer abc","password":"pw","safe":"ok"}`,
			want:  `{"API-Key":"[REDACTED]","authorization":"[REDACTED]","password":"[REDACTED]","safe":"ok"}`,
		},
		{
			name:  "plain headers and quotes",
			input: "Authorization: Basic basic-secret\nx-api-key='api-secret'\npassword=password-secret",
			want:  "Authorization: [REDACTED]\nx-api-key=[REDACTED]\npassword=[REDACTED]",
		},
		{
			name:  "query separators and camel case",
			input: "api_key=one&access-token=two&refreshToken=three&safe=four",
			want:  "api_key=[REDACTED]&access-token=[REDACTED]&refreshToken=[REDACTED]&safe=four",
		},
		{
			name:  "session identifiers preserve keys and quote values",
			input: `{"id":7,"userId":"u","session_id":null,"threadId":true,"tool_call_id":"call","safe":"ok"}`,
			want:  `{"id":"[REDACTED]","userId":"[REDACTED]","session_id":"[REDACTED]","threadId":"[REDACTED]","tool_call_id":"[REDACTED]","safe":"ok"}`,
		},
		{
			name:  "invalid json text",
			input: `"thread_id": "thread-secret", API_KEY=plain-secret`,
			want:  `"thread_id": "[REDACTED]", API_KEY=[REDACTED]`,
		},
		{
			name:  "non string sensitive json values stay unchanged",
			input: `{"password":123,"secret":false}`,
			want:  `{"password":123,"secret":false}`,
		},
		{
			name:  "unicode simple fold remains supported",
			input: `{"api_Key":"secret-value","ſession_id":"session-value"}`,
			want:  `{"api_Key":"[REDACTED]","ſession_id":"[REDACTED]"}`,
		},
		{
			name:  "clean payload",
			input: `{"model":"gpt-5.3-codex","stream":true}`,
			want:  `{"model":"gpt-5.3-codex","stream":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRequestLogPayload(tt.input); got != tt.want {
				t.Fatalf("逐字节脱敏结果不一致\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

func TestPrepareRequestLogPayloadForPersistence_CapturedAndPrefilledRequestMatch(t *testing.T) {
	input := `{"api_key":"secret","thread_id":"thread-secret","safe":"ok"}`
	for _, sanitize := range []bool{false, true} {
		t.Run(map[bool]string{false: "raw", true: "sanitized"}[sanitize], func(t *testing.T) {
			captured := &ReqeustLog{
				CapturePayload:   true,
				SanitizePayload:  sanitize,
				requestBodyBytes: []byte(input),
			}
			prefilled := &ReqeustLog{
				CapturePayload:  true,
				SanitizePayload: sanitize,
				RequestBody:     input,
			}

			prepareRequestLogPayloadForPersistence(captured)
			prepareRequestLogPayloadForPersistence(prefilled)

			if captured.RequestBody != prefilled.RequestBody ||
				captured.RequestBodyTruncated != prefilled.RequestBodyTruncated ||
				captured.PayloadBytes != prefilled.PayloadBytes ||
				captured.PayloadCaptured != prefilled.PayloadCaptured {
				t.Fatalf("捕获与预填充请求结果不一致: captured=%#v prefilled=%#v", captured, prefilled)
			}
		})
	}
}

func TestRequestLogPayloadQuickChecks(t *testing.T) {
	tests := []struct {
		name      string
		check     func(string) bool
		matches   []string
		clean     []string
	}{
		{
			name:  "sensitive keywords",
			check: requestLogPayloadMayContainSensitiveKeyword,
			matches: []string{
				`apiKey=value`, `API_KEY=value`, `api-key=value`, `x-api-key=value`, `x-goog-api-key=value`,
				`Authorization: Bearer value`, `Proxy-Authorization: Basic value`,
				`authToken=value`, `auth_token=value`, `auth-token=value`,
				`accessToken=value`, `access_token=value`, `access-token=value`,
				`refreshToken=value`, `refresh_token=value`, `refresh-token=value`,
				`password=value`, `secret=value`, `api_Key=value`,
			},
			clean: []string{
				`{"model":"gpt-5.3-codex","input_tokens":1}`,
				`{"monkey":"value","classification":"public"}`,
				`{"message":"你好，世界"}`,
			},
		},
		{
			name:  "session identifiers",
			check: requestLogPayloadMayContainSessionIdentifier,
			matches: []string{
				`{"user_id":"x"}`, `{"userId":"x"}`, `{"session_id":"x"}`, `{"sessionId":"x"}`,
				`{"conversation_id":"x"}`, `{"conversationId":"x"}`, `{"thread_id":"x"}`, `{"threadId":"x"}`,
				`{"parent_thread_id":"x"}`, `{"parentThreadId":"x"}`, `{"rollout_path":"x"}`, `{"rolloutPath":"x"}`,
				`{"tool_call_id":"x"}`, `{"toolCallId":"x"}`, `{"call_id":"x"}`, `{"callId":"x"}`,
				`{"tool_use_id":"x"}`, `{"toolUseId":"x"}`, `{"previous_response_id":"x"}`, `{"previousResponseId":"x"}`,
				`{"response_id":"x"}`, `{"responseId":"x"}`, `{"ID" : 7}`, `{"ſession_id":"x"}`,
			},
			clean: []string{
				`{"model_id":"x","identity":"public"}`,
				`id: 7`,
				`{"message":"你好，世界"}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, input := range tt.matches {
				if !tt.check(input) {
					t.Fatalf("快速门控漏报 %q", input)
				}
			}
			for _, input := range tt.clean {
				if tt.check(input) {
					t.Fatalf("快速门控误报 %q", input)
				}
			}
		})
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
