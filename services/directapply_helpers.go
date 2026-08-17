package services

import (
	"strings"
)

// providerFilePathNoCreate 返回 provider 配置文件路径（不创建目录）
// 用于只读操作场景，避免副作用
func providerFilePathNoCreate(kind string) (string, error) {
	return providerConfigPath(kind, false)
}

// loadProviderSnapshot 只读加载 provider 列表（不触发迁移和保存）
// 从统一存储读取（自定义 CLI 平台自动回退 JSON 文件），用于直连应用的 provider 查找
func loadProviderSnapshot(kind string) ([]Provider, error) {
	return LoadProvidersFromStore(kind)
}

// findProviderByID 在 provider 列表中按 ID 查找
// 返回找到的 Provider 和是否找到
func findProviderByID(providers []Provider, id int64) (Provider, bool) {
	for _, p := range providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// normalizeURLTrimSlash 标准化 URL：去除所有尾部斜杠和空白
func normalizeURLTrimSlash(url string) string {
	return strings.TrimRight(strings.TrimSpace(url), "/")
}

// urlsEqualFold 不区分大小写比较两个 URL（已标准化）
func urlsEqualFold(a, b string) bool {
	return strings.EqualFold(normalizeURLTrimSlash(a), normalizeURLTrimSlash(b))
}
