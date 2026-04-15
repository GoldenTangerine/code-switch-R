package services

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestClaudeSettingsServiceApplySingleProviderRejectsTransformedAPIFormat(t *testing.T) {
	useIsolatedHomeDir(t)

	providerPath, err := providerFilePath("claude")
	if err != nil {
		t.Fatalf("获取 provider 配置路径失败: %v", err)
	}

	payload, err := json.Marshal(providerEnvelope{
		Providers: []Provider{
			{
				ID:        42,
				Name:      "OpenAI Compatible Claude",
				APIURL:    "https://example.com",
				APIKey:    "test-key",
				APIFormat: claudeAPIFormatOpenAIChat,
			},
		},
	})
	if err != nil {
		t.Fatalf("序列化 provider 配置失败: %v", err)
	}

	if err := os.WriteFile(providerPath, payload, 0o600); err != nil {
		t.Fatalf("写入 provider 配置失败: %v", err)
	}

	service := NewClaudeSettingsService(":18100")
	err = service.ApplySingleProvider(42)
	if err == nil {
		t.Fatal("期望直连应用被拒绝，但实际返回成功")
	}
	if !strings.Contains(err.Error(), "仅支持托管路由") {
		t.Fatalf("错误信息未命中托管路由限制，err=%v", err)
	}
}
