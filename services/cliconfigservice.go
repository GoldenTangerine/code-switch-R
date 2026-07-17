package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const legacyCodexBackupConfigName = "cc" + "-studio" + ".back.config.toml"

// CliConfigService CLI 配置管理服务
// 管理 Claude Code、Codex、Gemini 的 CLI 配置文件
type CliConfigService struct {
	relayAddr   string
	homeDir     string // 缓存的用户家目录（已校验）
	homeErr     error  // 家目录获取错误
	appSettings *AppSettingsService
}

// NewCliConfigService 创建 CLI 配置服务
func NewCliConfigService(relayAddr string, appSettings *AppSettingsService) *CliConfigService {
	home, err := getUserHomeDir()
	return &CliConfigService{
		relayAddr:   relayAddr,
		homeDir:     home,
		homeErr:     err,
		appSettings: appSettings,
	}
}

func (s *CliConfigService) proxyAuthField() string {
	if s.appSettings == nil {
		return claudeProxyAuthFieldAuthToken
	}
	settings, err := s.appSettings.GetAppSettings()
	if err != nil {
		return claudeProxyAuthFieldAuthToken
	}
	return normalizeClaudeProxyAuthField(settings.ClaudeProxyAuthField)
}

// requireHome 校验家目录是否可用
func (s *CliConfigService) requireHome() error {
	if s.homeErr != nil {
		return fmt.Errorf("无法获取用户家目录: %w", s.homeErr)
	}
	if s.homeDir == "" || s.homeDir == "." || !filepath.IsAbs(s.homeDir) {
		return fmt.Errorf("无法获取用户家目录: homeDir 未初始化或无效")
	}
	return nil
}

// CLIPlatform CLI 平台类型
type CLIPlatform string

const (
	PlatformClaude CLIPlatform = "claude"
	PlatformCodex  CLIPlatform = "codex"
	PlatformGemini CLIPlatform = "gemini"
)

// CLIConfigField 配置字段信息
type CLIConfigField struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Locked   bool   `json:"locked"`
	Hint     string `json:"hint,omitempty"`
	Type     string `json:"type"` // "string", "boolean", "object"
	Required bool   `json:"required,omitempty"`
}

// CLIConfigFile 配置文件预览（用于前端显示原始内容）
type CLIConfigFile struct {
	Path    string `json:"path"`
	Format  string `json:"format,omitempty"` // "json", "toml", "env"
	Content string `json:"content"`
}

// CLIConfig CLI 配置数据
type CLIConfig struct {
	Platform     CLIPlatform            `json:"platform"`
	Fields       []CLIConfigField       `json:"fields"`
	RawContent   string                 `json:"rawContent,omitempty"`   // 原始文件内容（用于高级编辑）
	RawFiles     []CLIConfigFile        `json:"rawFiles,omitempty"`     // 多文件内容预览
	ConfigFormat string                 `json:"configFormat,omitempty"` // "json" 或 "toml"
	EnvContent   map[string]string      `json:"envContent,omitempty"`   // Gemini .env 内容
	FilePath     string                 `json:"filePath,omitempty"`     // 配置文件路径
	Editable     map[string]interface{} `json:"editable,omitempty"`     // 可编辑字段的当前值
}

// CLIConfigSnapshots CLI 配置快照（用于旧预览链路）
type CLIConfigSnapshots struct {
	CurrentFiles []CLIConfigFile `json:"currentFiles"`
	PreviewFiles []CLIConfigFile `json:"previewFiles"`
	Mode         string          `json:"mode"` // "proxy" | "direct"
}

// CLITemplate CLI 配置模板
type CLITemplate struct {
	Template        map[string]interface{} `json:"template"`
	IsGlobalDefault bool                   `json:"isGlobalDefault"`
}

type CLIEditorContent struct {
	Format       string           `json:"format,omitempty"`
	Content      string           `json:"content"`
	LockedFields []CLIConfigField `json:"lockedFields,omitempty"`
}

type CLINormalizedEditorContent struct {
	Editable     map[string]interface{} `json:"editable"`
	Format       string                 `json:"format,omitempty"`
	Content      string                 `json:"content"`
	LockedFields []CLIConfigField       `json:"lockedFields,omitempty"`
}

// CLITemplates 所有平台的模板存储
type CLITemplates struct {
	Claude CLITemplate `json:"claude"`
	Codex  CLITemplate `json:"codex"`
	Gemini CLITemplate `json:"gemini"`
}

func cloneCLIEditableMap(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}

	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		return map[string]interface{}{}
	}

	var clone map[string]interface{}
	if err := json.Unmarshal(payload, &clone); err != nil || clone == nil {
		return map[string]interface{}{}
	}

	return clone
}

func hasCLIEditorProviderInput(platform CLIPlatform, apiURL string, apiKey string) bool {
	baseURL := strings.TrimSpace(apiURL)
	token := strings.TrimSpace(apiKey)

	if platform == PlatformGemini {
		return baseURL != "" || token != ""
	}

	return baseURL != "" && token != ""
}

func parseJSONRootObject(content string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return map[string]interface{}{}, nil
	}

	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			line, column := jsonOffsetToLineColumn(trimmed, syntaxErr.Offset)
			return nil, fmt.Errorf("JSON 第 %d 行第 %d 列格式无效: %v", line, column, err)
		}
		return nil, fmt.Errorf("JSON 格式无效: %w", err)
	}

	data, ok := parsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("JSON 根节点必须是对象")
	}
	if data == nil {
		return map[string]interface{}{}, nil
	}
	return data, nil
}

func jsonOffsetToLineColumn(content string, offset int64) (int, int) {
	if offset < 1 {
		return 1, 1
	}

	line := 1
	column := 1
	index := int64(0)
	for _, r := range content {
		index += int64(len(string(r)))
		if index >= offset {
			return line, column
		}
		if r == '\n' {
			line += 1
			column = 1
			continue
		}
		column += 1
	}

	return line, column
}

func parseEditorEnvContent(content string) (map[string]string, error) {
	result := make(map[string]string)
	normalizedContent := strings.ReplaceAll(content, "\r\n", "\n")
	normalizedContent = strings.ReplaceAll(normalizedContent, "\r", "\n")
	lines := strings.Split(normalizedContent, "\n")

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		eqIndex := strings.Index(trimmed, "=")
		if eqIndex <= 0 {
			return nil, fmt.Errorf(".env 第 %d 行格式无效，必须是 KEY=VALUE", index+1)
		}

		key := strings.TrimSpace(trimmed[:eqIndex])
		value := strings.TrimSpace(trimmed[eqIndex+1:])
		if key == "" {
			return nil, fmt.Errorf(".env 第 %d 行缺少变量名", index+1)
		}
		if !isValidEnvKey(key) {
			return nil, fmt.Errorf(".env 第 %d 行变量名无效: %s", index+1, key)
		}
		result[key] = value
	}

	return result, nil
}

func stripClaudeLockedEditableFields(value map[string]interface{}) map[string]interface{} {
	nextValue := cloneCLIEditableMap(value)
	envValue, ok := nextValue["env"].(map[string]interface{})
	if ok && envValue != nil {
		delete(envValue, "ANTHROPIC_BASE_URL")
		delete(envValue, claudeAuthTokenEnvKey)
		if len(envValue) == 0 {
			delete(nextValue, "env")
		} else {
			nextValue["env"] = envValue
		}
	}
	return nextValue
}

func stripCodexLockedEditableFields(value map[string]interface{}) map[string]interface{} {
	nextValue := cloneCLIEditableMap(value)
	delete(nextValue, "model_provider")
	delete(nextValue, "preferred_auth_method")

	modelProvidersValue, ok := nextValue["model_providers"].(map[string]interface{})
	if !ok || modelProvidersValue == nil {
		return nextValue
	}

	delete(modelProvidersValue, codexProviderKey)
	if len(modelProvidersValue) == 0 {
		delete(nextValue, "model_providers")
	} else {
		nextValue["model_providers"] = modelProvidersValue
	}

	return nextValue
}

func stripCodexEditorManagedFields(value map[string]interface{}, providerKey string) map[string]interface{} {
	nextValue := stripCodexLockedEditableFields(value)
	trimmedProviderKey := strings.TrimSpace(providerKey)
	if trimmedProviderKey == "" || trimmedProviderKey == codexProviderKey {
		return nextValue
	}

	modelProvidersValue, ok := nextValue["model_providers"].(map[string]interface{})
	if !ok || modelProvidersValue == nil {
		return nextValue
	}

	providerValue, exists := modelProvidersValue[trimmedProviderKey]
	if !exists {
		return nextValue
	}

	providerMap, ok := providerValue.(map[string]interface{})
	if !ok || providerMap == nil {
		delete(modelProvidersValue, trimmedProviderKey)
		if len(modelProvidersValue) == 0 {
			delete(nextValue, "model_providers")
		} else {
			nextValue["model_providers"] = modelProvidersValue
		}
		return nextValue
	}

	cleanedProviderMap := cloneCLIEditableMap(providerMap)
	delete(cleanedProviderMap, "name")
	delete(cleanedProviderMap, "base_url")
	delete(cleanedProviderMap, "wire_api")
	delete(cleanedProviderMap, "requires_openai_auth")

	if len(cleanedProviderMap) == 0 {
		delete(modelProvidersValue, trimmedProviderKey)
	} else {
		modelProvidersValue[trimmedProviderKey] = cleanedProviderMap
	}

	if len(modelProvidersValue) == 0 {
		delete(nextValue, "model_providers")
	} else {
		nextValue["model_providers"] = modelProvidersValue
	}

	return nextValue
}

func buildLockedField(key string, value interface{}, hint string, fieldType string) CLIConfigField {
	return CLIConfigField{
		Key:    key,
		Value:  anyToString(value),
		Locked: true,
		Hint:   hint,
		Type:   fieldType,
	}
}

func normalizeTomlGenericMap(value interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}

	if typed, ok := value.(map[string]interface{}); ok {
		return cloneCLIEditableMap(typed)
	}

	payload, err := json.Marshal(value)
	if err != nil || len(payload) == 0 {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(payload, &result); err != nil || result == nil {
		return nil
	}

	return result
}

func lookupCodexProviderTable(raw map[string]interface{}, providerKey string) map[string]interface{} {
	if strings.TrimSpace(providerKey) == "" {
		return nil
	}

	modelProvidersValue := normalizeTomlGenericMap(raw["model_providers"])
	if modelProvidersValue == nil {
		return nil
	}

	return normalizeTomlGenericMap(modelProvidersValue[providerKey])
}

func buildClaudeEditorLockedFields(value map[string]interface{}, proxyBaseURL string) []CLIConfigField {
	envValue, _ := value["env"].(map[string]interface{})
	if envValue == nil {
		return []CLIConfigField{}
	}

	lockedFields := make([]CLIConfigField, 0, 2)
	if baseURL := anyToString(envValue["ANTHROPIC_BASE_URL"]); strings.TrimSpace(baseURL) != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"env.ANTHROPIC_BASE_URL",
			baseURL,
			"由系统管理，指向当前生效的服务地址",
			"string",
		))
	}
	if authToken := anyToString(envValue[claudeAuthTokenEnvKey]); strings.TrimSpace(authToken) != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"env.ANTHROPIC_AUTH_TOKEN",
			authToken,
			"由系统管理，当前认证令牌只读展示",
			"string",
		))
	}
	if apiKey := anyToString(envValue[claudeAPIKeyEnvKey]); strings.TrimSpace(apiKey) != "" && isClaudeProxyBaseURL(envValue, proxyBaseURL) {
		lockedFields = append(lockedFields, buildLockedField(
			"env.ANTHROPIC_API_KEY",
			apiKey,
			"由系统管理，当前 API Key 只读展示",
			"string",
		))
	}

	return lockedFields
}

func isClaudeProxyBaseURL(env map[string]interface{}, proxyBaseURL string) bool {
	if env == nil {
		return false
	}
	currentBaseURL := strings.TrimSuffix(strings.TrimSpace(anyToString(env["ANTHROPIC_BASE_URL"])), "/")
	normalizedProxyBaseURL := strings.TrimSuffix(strings.TrimSpace(proxyBaseURL), "/")
	return currentBaseURL != "" && strings.EqualFold(currentBaseURL, normalizedProxyBaseURL)
}

func buildCodexEditorLockedFields(raw map[string]interface{}) []CLIConfigField {
	lockedFields := make([]CLIConfigField, 0, 6)
	currentProviderKey := strings.TrimSpace(anyToString(raw["model_provider"]))
	preferredAuth := strings.TrimSpace(anyToString(raw["preferred_auth_method"]))

	if currentProviderKey != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"model_provider",
			currentProviderKey,
			"由系统管理，指向当前生效的 provider key",
			"string",
		))
	}
	if preferredAuth != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"preferred_auth_method",
			preferredAuth,
			"由系统管理，当前认证方式只读展示",
			"string",
		))
	}
	if currentProviderKey == "" {
		return lockedFields
	}

	providerMap := lookupCodexProviderTable(raw, currentProviderKey)
	if providerMap == nil {
		return lockedFields
	}

	if _, exists := providerMap["base_url"]; exists {
		lockedFields = append(lockedFields, buildLockedField(
			fmt.Sprintf("model_providers.%s.base_url", currentProviderKey),
			providerMap["base_url"],
			"由系统管理，指向当前 provider 的生效地址",
			"string",
		))
	}
	if _, exists := providerMap["name"]; exists {
		lockedFields = append(lockedFields, buildLockedField(
			fmt.Sprintf("model_providers.%s.name", currentProviderKey),
			providerMap["name"],
			"由系统管理，标识当前注入的 provider",
			"string",
		))
	}
	if _, exists := providerMap["wire_api"]; exists {
		lockedFields = append(lockedFields, buildLockedField(
			fmt.Sprintf("model_providers.%s.wire_api", currentProviderKey),
			providerMap["wire_api"],
			"由系统管理，固定使用的 Wire API",
			"string",
		))
	}
	if _, exists := providerMap["requires_openai_auth"]; exists {
		lockedFields = append(lockedFields, buildLockedField(
			fmt.Sprintf("model_providers.%s.requires_openai_auth", currentProviderKey),
			providerMap["requires_openai_auth"],
			"由系统管理，标记是否要求 OpenAI Auth",
			"boolean",
		))
	}

	return lockedFields
}

func buildGeminiEditorLockedFields(envMap map[string]string) []CLIConfigField {
	lockedFields := make([]CLIConfigField, 0, 2)
	if baseURL := strings.TrimSpace(envMap["GOOGLE_GEMINI_BASE_URL"]); baseURL != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"GOOGLE_GEMINI_BASE_URL",
			baseURL,
			"由系统管理，指向当前生效的服务地址",
			"string",
		))
	}
	if apiKey := strings.TrimSpace(envMap["GEMINI_API_KEY"]); apiKey != "" {
		lockedFields = append(lockedFields, buildLockedField(
			"GEMINI_API_KEY",
			apiKey,
			"由系统管理，当前认证令牌只读展示",
			"string",
		))
	}

	return lockedFields
}

func stripGeminiLockedEditableFields(value map[string]string) map[string]interface{} {
	nextValue := make(map[string]interface{})
	for key, entryValue := range value {
		if key == "GOOGLE_GEMINI_BASE_URL" || key == "GEMINI_API_KEY" {
			continue
		}
		nextValue[key] = entryValue
	}
	return nextValue
}

func stripGeminiLockedEditableTemplateFields(value map[string]interface{}) map[string]interface{} {
	nextValue := cloneCLIEditableMap(value)
	delete(nextValue, "GOOGLE_GEMINI_BASE_URL")
	delete(nextValue, "GEMINI_API_KEY")
	return nextValue
}

func normalizeTemplateEditableFields(platform CLIPlatform, value map[string]interface{}) map[string]interface{} {
	switch platform {
	case PlatformClaude:
		return stripClaudeLockedEditableFields(value)
	case PlatformCodex:
		return stripCodexLockedEditableFields(value)
	case PlatformGemini:
		return stripGeminiLockedEditableTemplateFields(value)
	default:
		return cloneCLIEditableMap(value)
	}
}

func resolveCodexEditorProviderKey(providerName string, apiURL string, apiKey string) string {
	trimmedName := strings.TrimSpace(providerName)

	if providers, err := loadProviderSnapshot("codex"); err == nil {
		if trimmedName != "" {
			for _, provider := range providers {
				if strings.EqualFold(strings.TrimSpace(provider.Name), trimmedName) {
					return sanitizeProviderKey(provider.Name, int(provider.ID))
				}
			}
		}
		for _, provider := range providers {
			if urlsEqualFold(provider.APIURL, apiURL) && provider.APIKey == apiKey {
				return sanitizeProviderKey(provider.Name, int(provider.ID))
			}
		}
	}

	if trimmedName != "" {
		key := sanitizeProviderKey(trimmedName, 0)
		if key != "" && key != "provider-0" {
			return key
		}
	}

	return "preview-provider"
}

// getTemplatesPath 获取模板存储路径
func (s *CliConfigService) getTemplatesPath() string {
	return filepath.Join(s.homeDir, ".code-switch", "cli-templates.json")
}

// GetConfig 获取指定平台的 CLI 配置
func (s *CliConfigService) GetConfig(platform string) (*CLIConfig, error) {
	if err := s.requireHome(); err != nil {
		return nil, err
	}

	p := CLIPlatform(platform)
	switch p {
	case PlatformClaude:
		return s.getClaudeConfig()
	case PlatformCodex:
		return s.getCodexConfig()
	case PlatformGemini:
		return s.getGeminiConfig()
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

func resolveCLIConfigPreviewMode(apiUrl string, apiKey string, previewMode string) (string, error) {
	previewModeTrim := strings.ToLower(strings.TrimSpace(previewMode))
	switch previewModeTrim {
	case "":
		if strings.TrimSpace(apiUrl) != "" || strings.TrimSpace(apiKey) != "" {
			return "direct", nil
		}
		return "proxy", nil
	case "current", "direct", "proxy":
		return previewModeTrim, nil
	default:
		return "", fmt.Errorf("无效的 previewMode: %s（允许值: current, direct, proxy）", previewMode)
	}
}

// GetConfigSnapshots 获取指定平台的配置快照，用于前端展示"当前(磁盘)"与"预览(激活后)"对比。
// 这是纯 dry-run 接口：不会对任何文件进行写入。
//
// previewMode 参数：
//   - "current": Preview = Current（不做任何注入，适用于新建供应商空输入）
//   - "direct": 模拟直连应用 ApplySingleProvider() 的写入结果
//   - "proxy": 模拟启用代理 EnableProxy() 的写入结果
//   - "" (空字符串): 兼容旧逻辑，若 apiUrl/apiKey 任一非空则为 direct，否则为 proxy
func (s *CliConfigService) GetConfigSnapshots(platform string, apiUrl string, apiKey string, previewMode string) (*CLIConfigSnapshots, error) {
	if err := s.requireHome(); err != nil {
		return nil, err
	}

	p := CLIPlatform(platform)

	readText := func(path string) (string, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return "", nil
			}
			return "", err
		}
		return string(content), nil
	}

	effectiveMode, err := resolveCLIConfigPreviewMode(apiUrl, apiKey, previewMode)
	if err != nil {
		return nil, err
	}

	// 用于旧代码兼容的布尔标志
	previewDirect := effectiveMode == "direct"

	switch p {
	case PlatformClaude:
		configPath := s.getClaudeConfigPath()

		currentContent, err := readText(configPath)
		if err != nil {
			return nil, fmt.Errorf("读取 Claude 配置失败: %w", err)
		}

		currentFiles := []CLIConfigFile{
			{Path: configPath, Format: "json", Content: currentContent},
		}

		// 计算当前模式：是否指向本地代理
		currentMode := "direct"
		if strings.TrimSpace(currentContent) != "" {
			var payload map[string]any
			if err := json.Unmarshal([]byte(currentContent), &payload); err == nil {
				env, _ := payload["env"].(map[string]any)
				if env != nil {
					baseURLVal := anyToString(env["ANTHROPIC_BASE_URL"])
					enabled := strings.EqualFold(
						strings.TrimSuffix(strings.TrimSpace(baseURLVal), "/"),
						strings.TrimSuffix(strings.TrimSpace(s.baseURL()), "/"),
					)
					if enabled {
						currentMode = "proxy"
					}
				}
			}
		}

		// current 模式：Preview = Current（不做任何注入）
		if effectiveMode == "current" {
			// 深拷贝 currentFiles 避免引用共享
			previewFiles := make([]CLIConfigFile, len(currentFiles))
			copy(previewFiles, currentFiles)
			return &CLIConfigSnapshots{
				CurrentFiles: currentFiles,
				PreviewFiles: previewFiles,
				Mode:         currentMode,
			}, nil
		}

		// 构造预览：最小侵入，仅更新锁定字段
		previewData := make(map[string]any)
		if strings.TrimSpace(currentContent) != "" {
			if err := json.Unmarshal([]byte(currentContent), &previewData); err != nil {
				previewData = make(map[string]any)
			}
		}
		env, _ := previewData["env"].(map[string]any)
		if env == nil {
			env = make(map[string]any)
		}
		if previewDirect {
			env["ANTHROPIC_BASE_URL"] = normalizeURLTrimSlash(apiUrl)
			env[claudeAuthTokenEnvKey] = apiKey
		} else {
			env["ANTHROPIC_BASE_URL"] = s.baseURL()
			applyClaudeProxyAuthEnv(env, s.proxyAuthField())
		}
		previewData["env"] = env

		previewBytes, err := json.MarshalIndent(previewData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化 Claude 预览配置失败: %w", err)
		}

		previewFiles := []CLIConfigFile{
			{Path: configPath, Format: "json", Content: string(previewBytes)},
		}

		return &CLIConfigSnapshots{
			CurrentFiles: currentFiles,
			PreviewFiles: previewFiles,
			Mode:         currentMode,
		}, nil

	case PlatformCodex:
		configPath := s.getCodexConfigPath()
		authPath := s.getCodexAuthPath()

		currentConfig, err := readText(configPath)
		if err != nil {
			return nil, fmt.Errorf("读取 Codex 配置失败: %w", err)
		}
		currentAuth, err := readText(authPath)
		if err != nil {
			return nil, fmt.Errorf("读取 Codex 认证文件失败: %w", err)
		}

		currentFiles := []CLIConfigFile{
			{Path: configPath, Format: "toml", Content: currentConfig},
			{Path: authPath, Format: "json", Content: currentAuth},
		}

		// 计算当前模式：是否指向本地代理
		// 向后兼容：同时检查 code-switch-r（新）和 code-switch（旧）两个 key
		currentMode := "direct"
		if strings.TrimSpace(currentConfig) != "" {
			var cfg codexConfig
			if err := toml.Unmarshal([]byte(currentConfig), &cfg); err == nil {
				proxyKeys := []string{codexProviderKey, "code-switch"}
				for _, key := range proxyKeys {
					provider, ok := cfg.ModelProviders[key]
					if ok && strings.EqualFold(cfg.ModelProvider, key) && strings.EqualFold(provider.BaseURL, s.baseURL()) {
						currentMode = "proxy"
						break
					}
				}
			}
		}

		// current 模式：Preview = Current（不做任何注入）
		if effectiveMode == "current" {
			// 深拷贝 currentFiles 避免引用共享
			previewFiles := make([]CLIConfigFile, len(currentFiles))
			copy(previewFiles, currentFiles)
			return &CLIConfigSnapshots{
				CurrentFiles: currentFiles,
				PreviewFiles: previewFiles,
				Mode:         currentMode,
			}, nil
		}

		// 解析现有 TOML
		raw := make(map[string]any)
		if strings.TrimSpace(currentConfig) != "" {
			if err := toml.Unmarshal([]byte(currentConfig), &raw); err != nil {
				raw = make(map[string]any)
			}
		}

		// 解析现有 auth.json（用于 proxy 模式保留其他字段）
		authPayload := make(map[string]any)
		if strings.TrimSpace(currentAuth) != "" {
			if err := json.Unmarshal([]byte(currentAuth), &authPayload); err != nil {
				authPayload = make(map[string]any)
			}
		}

		if previewDirect {
			// 复用 provider 快照推导 providerKey
			providerKey := "preview-provider"
			if providers, err := loadProviderSnapshot("codex"); err == nil {
				for _, p := range providers {
					if urlsEqualFold(p.APIURL, apiUrl) && p.APIKey == apiKey {
						providerKey = sanitizeProviderKey(p.Name, int(p.ID))
						break
					}
				}
			}

			raw["preferred_auth_method"] = "apikey"
			raw["model_provider"] = providerKey

			modelProviders := ensureTomlTable(raw, "model_providers")
			providerCfg := ensureProviderTable(modelProviders, providerKey)
			providerCfg["name"] = providerKey
			providerCfg["base_url"] = normalizeURLTrimSlash(apiUrl)
			providerCfg["wire_api"] = "responses"
			providerCfg["requires_openai_auth"] = false
			modelProviders[providerKey] = providerCfg
			raw["model_providers"] = modelProviders

			// direct 模式：只保留 OPENAI_API_KEY（与 writeDirectApplyAuthFile 一致）
			authPayload = map[string]any{"OPENAI_API_KEY": apiKey}
		} else {
			raw["preferred_auth_method"] = "apikey"
			raw["model_provider"] = "code-switch-r"

			if _, exists := raw["model"]; !exists {
				raw["model"] = "gpt-5-codex"
			}

			modelProviders := ensureTomlTable(raw, "model_providers")
			providerCfg := ensureProviderTable(modelProviders, "code-switch-r")
			providerCfg["name"] = "code-switch-r"
			providerCfg["base_url"] = s.baseURL()
			providerCfg["wire_api"] = "responses"
			providerCfg["requires_openai_auth"] = false
			modelProviders["code-switch-r"] = providerCfg
			raw["model_providers"] = modelProviders

			// proxy 模式：保留其他字段，只更新 OPENAI_API_KEY（与 writeAuthFile 一致）
			authPayload["OPENAI_API_KEY"] = "code-switch-r"
		}

		tomlBytes, err := toml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex 预览配置失败: %w", err)
		}
		cleaned := stripModelProvidersHeader(tomlBytes)

		authBytes, err := json.MarshalIndent(authPayload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex auth 预览失败: %w", err)
		}

		previewFiles := []CLIConfigFile{
			{Path: configPath, Format: "toml", Content: string(cleaned)},
			{Path: authPath, Format: "json", Content: string(authBytes)},
		}

		return &CLIConfigSnapshots{
			CurrentFiles: currentFiles,
			PreviewFiles: previewFiles,
			Mode:         currentMode,
		}, nil

	case PlatformGemini:
		envPath := s.getGeminiEnvPath()
		currentEnv, err := readText(envPath)
		if err != nil {
			return nil, fmt.Errorf("读取 Gemini .env 失败: %w", err)
		}

		currentFiles := []CLIConfigFile{
			{Path: envPath, Format: "env", Content: currentEnv},
		}

		// 计算当前模式：是否指向本地代理
		currentMode := "direct"
		if strings.TrimSpace(currentEnv) != "" {
			envMap := parseEnvFile(currentEnv)
			if strings.EqualFold(strings.TrimSpace(envMap["GOOGLE_GEMINI_BASE_URL"]), strings.TrimSpace(s.geminiBaseURL())) {
				currentMode = "proxy"
			}
		}

		// current 模式：Preview = Current（不做任何注入）
		if effectiveMode == "current" {
			// 深拷贝 currentFiles 避免引用共享
			previewFiles := make([]CLIConfigFile, len(currentFiles))
			copy(previewFiles, currentFiles)
			return &CLIConfigSnapshots{
				CurrentFiles: currentFiles,
				PreviewFiles: previewFiles,
				Mode:         currentMode,
			}, nil
		}

		envMap := parseEnvFile(currentEnv)
		if envMap == nil {
			envMap = make(map[string]string)
		}

		if previewDirect {
			if strings.TrimSpace(apiUrl) != "" {
				envMap["GOOGLE_GEMINI_BASE_URL"] = strings.TrimSpace(apiUrl)
			} else {
				delete(envMap, "GOOGLE_GEMINI_BASE_URL")
			}
			if strings.TrimSpace(apiKey) != "" {
				envMap["GEMINI_API_KEY"] = strings.TrimSpace(apiKey)
			} else {
				delete(envMap, "GEMINI_API_KEY")
			}
		} else {
			envMap["GOOGLE_GEMINI_BASE_URL"] = s.geminiBaseURL()
			envMap["GEMINI_API_KEY"] = "code-switch-r"
		}

		previewFiles := []CLIConfigFile{
			{Path: envPath, Format: "env", Content: buildGeminiEnvContent(envMap)},
		}

		return &CLIConfigSnapshots{
			CurrentFiles: currentFiles,
			PreviewFiles: previewFiles,
			Mode:         currentMode,
		}, nil

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// GetConfigSnapshotsWithEditable 基于当前编辑中的 editable 生成预览快照。
// CurrentFiles 始终来自磁盘真实内容；PreviewFiles 则基于 editable 进行 dry-run 生成，
// 这样前端 JSON 编辑器和“预览效果”可以共享同一份逻辑来源。
func (s *CliConfigService) GetConfigSnapshotsWithEditable(
	platform string,
	editable map[string]interface{},
	apiUrl string,
	apiKey string,
	previewMode string,
) (*CLIConfigSnapshots, error) {
	baseSnapshots, err := s.GetConfigSnapshots(platform, apiUrl, apiKey, previewMode)
	if err != nil {
		return nil, err
	}

	effectiveMode, err := resolveCLIConfigPreviewMode(apiUrl, apiKey, previewMode)
	if err != nil {
		return nil, err
	}

	cloneEditable := make(map[string]interface{})
	if editable != nil {
		bytes, marshalErr := json.Marshal(editable)
		if marshalErr != nil {
			return nil, fmt.Errorf("复制 editable 失败: %w", marshalErr)
		}
		if len(bytes) > 0 {
			if unmarshalErr := json.Unmarshal(bytes, &cloneEditable); unmarshalErr != nil {
				return nil, fmt.Errorf("复制 editable 失败: %w", unmarshalErr)
			}
		}
	}

	switch CLIPlatform(platform) {
	case PlatformClaude:
		configPath := s.getClaudeConfigPath()
		currentContent := ""
		if len(baseSnapshots.CurrentFiles) > 0 {
			currentContent = baseSnapshots.CurrentFiles[0].Content
		}

		previewData := cloneEditable
		if previewData == nil {
			previewData = make(map[string]interface{})
		}

		env, _ := previewData["env"].(map[string]interface{})
		if env == nil {
			env = make(map[string]interface{})
		}

		switch effectiveMode {
		case "direct":
			env["ANTHROPIC_BASE_URL"] = normalizeURLTrimSlash(apiUrl)
			env[claudeAuthTokenEnvKey] = apiKey
		case "proxy":
			env["ANTHROPIC_BASE_URL"] = s.baseURL()
			applyClaudeProxyAuthEnv(env, s.proxyAuthField())
		case "current":
			currentData := make(map[string]interface{})
			if strings.TrimSpace(currentContent) != "" {
				if err := json.Unmarshal([]byte(currentContent), &currentData); err == nil {
					currentEnv, _ := currentData["env"].(map[string]interface{})
					if currentEnv != nil {
						if baseURL := anyToString(currentEnv["ANTHROPIC_BASE_URL"]); baseURL != "" {
							env["ANTHROPIC_BASE_URL"] = baseURL
						}
						for _, key := range []string{claudeAuthTokenEnvKey, claudeAPIKeyEnvKey} {
							if value := anyToString(currentEnv[key]); value != "" {
								env[key] = value
							}
						}
					}
				}
			}
			if _, ok := env["ANTHROPIC_BASE_URL"]; !ok {
				env["ANTHROPIC_BASE_URL"] = s.baseURL()
			}
			if _, hasAuthToken := env[claudeAuthTokenEnvKey]; !hasAuthToken {
				if _, hasAPIKey := env[claudeAPIKeyEnvKey]; !hasAPIKey {
					applyClaudeProxyAuthEnv(env, s.proxyAuthField())
				}
			}
		}

		previewData["env"] = env
		previewBytes, err := json.MarshalIndent(previewData, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化 Claude 预览配置失败: %w", err)
		}

		baseSnapshots.PreviewFiles = []CLIConfigFile{
			{Path: configPath, Format: "json", Content: string(previewBytes)},
		}
		return baseSnapshots, nil

	case PlatformCodex:
		configPath := s.getCodexConfigPath()
		authPath := s.getCodexAuthPath()

		currentConfig := ""
		if len(baseSnapshots.CurrentFiles) > 0 {
			currentConfig = baseSnapshots.CurrentFiles[0].Content
		}
		currentAuth := ""
		if len(baseSnapshots.CurrentFiles) > 1 {
			currentAuth = baseSnapshots.CurrentFiles[1].Content
		}

		raw := make(map[string]interface{})
		if strings.TrimSpace(currentConfig) != "" {
			if err := toml.Unmarshal([]byte(currentConfig), &raw); err != nil {
				raw = make(map[string]interface{})
			}
		}
		if raw == nil {
			raw = make(map[string]interface{})
		}

		delete(raw, "model")
		delete(raw, "model_reasoning_effort")
		delete(raw, "disable_response_storage")
		for key, value := range cloneEditable {
			if key == "model_provider" || key == "preferred_auth_method" || strings.HasPrefix(key, "model_providers.") {
				continue
			}
			raw[key] = value
		}

		authPayload := make(map[string]interface{})
		if strings.TrimSpace(currentAuth) != "" {
			if err := json.Unmarshal([]byte(currentAuth), &authPayload); err != nil {
				authPayload = make(map[string]interface{})
			}
		}
		if authPayload == nil {
			authPayload = make(map[string]interface{})
		}

		switch effectiveMode {
		case "direct":
			providerKey := "preview-provider"
			if providers, err := loadProviderSnapshot("codex"); err == nil {
				for _, p := range providers {
					if urlsEqualFold(p.APIURL, apiUrl) && p.APIKey == apiKey {
						providerKey = sanitizeProviderKey(p.Name, int(p.ID))
						break
					}
				}
			}

			raw["preferred_auth_method"] = "apikey"
			raw["model_provider"] = providerKey

			modelProviders := ensureTomlTable(raw, "model_providers")
			providerCfg := ensureProviderTable(modelProviders, providerKey)
			providerCfg["name"] = providerKey
			providerCfg["base_url"] = normalizeURLTrimSlash(apiUrl)
			providerCfg["wire_api"] = "responses"
			providerCfg["requires_openai_auth"] = false
			modelProviders[providerKey] = providerCfg
			raw["model_providers"] = modelProviders

			authPayload = map[string]interface{}{"OPENAI_API_KEY": apiKey}
		case "proxy":
			raw["preferred_auth_method"] = "apikey"
			raw["model_provider"] = "code-switch-r"
			if _, exists := raw["model"]; !exists {
				raw["model"] = "gpt-5-codex"
			}

			modelProviders := ensureTomlTable(raw, "model_providers")
			providerCfg := ensureProviderTable(modelProviders, "code-switch-r")
			providerCfg["name"] = "code-switch-r"
			providerCfg["base_url"] = s.baseURL()
			providerCfg["wire_api"] = "responses"
			providerCfg["requires_openai_auth"] = false
			modelProviders["code-switch-r"] = providerCfg
			raw["model_providers"] = modelProviders

			authPayload["OPENAI_API_KEY"] = "code-switch-r"
		}

		tomlBytes, err := toml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex 预览配置失败: %w", err)
		}
		cleaned := stripModelProvidersHeader(tomlBytes)

		authBytes, err := json.MarshalIndent(authPayload, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex auth 预览失败: %w", err)
		}

		baseSnapshots.PreviewFiles = []CLIConfigFile{
			{Path: configPath, Format: "toml", Content: string(cleaned)},
			{Path: authPath, Format: "json", Content: string(authBytes)},
		}
		return baseSnapshots, nil

	case PlatformGemini:
		envPath := s.getGeminiEnvPath()
		currentEnv := ""
		if len(baseSnapshots.CurrentFiles) > 0 {
			currentEnv = baseSnapshots.CurrentFiles[0].Content
		}

		envMap := parseEnvFile(currentEnv)
		if envMap == nil {
			envMap = make(map[string]string)
		}

		for key := range envMap {
			if key != "GOOGLE_GEMINI_BASE_URL" && key != "GEMINI_API_KEY" {
				delete(envMap, key)
			}
		}
		for key, value := range cloneEditable {
			if key == "GOOGLE_GEMINI_BASE_URL" || key == "GEMINI_API_KEY" {
				continue
			}
			envMap[key] = fmt.Sprintf("%v", value)
		}

		switch effectiveMode {
		case "direct":
			if strings.TrimSpace(apiUrl) != "" {
				envMap["GOOGLE_GEMINI_BASE_URL"] = strings.TrimSpace(apiUrl)
			} else {
				delete(envMap, "GOOGLE_GEMINI_BASE_URL")
			}
			if strings.TrimSpace(apiKey) != "" {
				envMap["GEMINI_API_KEY"] = strings.TrimSpace(apiKey)
			} else {
				delete(envMap, "GEMINI_API_KEY")
			}
		case "proxy":
			envMap["GOOGLE_GEMINI_BASE_URL"] = s.geminiBaseURL()
			envMap["GEMINI_API_KEY"] = "code-switch-r"
		}

		baseSnapshots.PreviewFiles = []CLIConfigFile{
			{Path: envPath, Format: "env", Content: buildGeminiEnvContent(envMap)},
		}
		return baseSnapshots, nil

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// SaveConfig 保存 CLI 配置
func (s *CliConfigService) SaveConfig(
	platform string,
	editable map[string]interface{},
	apiURL string,
	apiKey string,
	providerName string,
) error {
	if err := s.requireHome(); err != nil {
		return err
	}

	p := CLIPlatform(platform)
	switch p {
	case PlatformClaude:
		return s.saveClaudeConfig(editable, apiURL, apiKey)
	case PlatformCodex:
		return s.saveCodexConfig(editable, apiURL, apiKey, providerName)
	case PlatformGemini:
		return s.saveGeminiConfig(editable, apiURL, apiKey)
	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}
}

// SaveConfigFileContent 保存指定配置文件内容（预览区高级编辑）
// 为避免越权写文件，只允许写入本服务管理的固定路径文件
func (s *CliConfigService) SaveConfigFileContent(platform string, filePath string, content string) error {
	if err := s.requireHome(); err != nil {
		return err
	}

	p := CLIPlatform(platform)
	cleaned := filepath.Clean(filePath)

	switch p {
	case PlatformClaude:
		expected := filepath.Clean(s.getClaudeConfigPath())
		if !samePath(cleaned, expected) {
			return fmt.Errorf("非法文件路径: %s", filePath)
		}
		return s.saveClaudeConfigContent(expected, content)
	case PlatformCodex:
		configPath := filepath.Clean(s.getCodexConfigPath())
		authPath := filepath.Clean(s.getCodexAuthPath())
		if samePath(cleaned, configPath) {
			return s.saveCodexConfigContent(configPath, content)
		}
		if samePath(cleaned, authPath) {
			return s.saveCodexAuthContent(authPath, content)
		}
		return fmt.Errorf("非法文件路径: %s", filePath)
	case PlatformGemini:
		envPath := filepath.Clean(s.getGeminiEnvPath())
		if !samePath(cleaned, envPath) {
			return fmt.Errorf("非法文件路径: %s", filePath)
		}
		return s.saveGeminiEnvContent(envPath, content)
	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}
}

// GetTemplate 获取指定平台的全局模板
func (s *CliConfigService) GetTemplate(platform string) (*CLITemplate, error) {
	if err := s.requireHome(); err != nil {
		return nil, err
	}

	templates, err := s.loadTemplates()
	if err != nil {
		return nil, err
	}

	switch CLIPlatform(platform) {
	case PlatformClaude:
		return &templates.Claude, nil
	case PlatformCodex:
		return &templates.Codex, nil
	case PlatformGemini:
		return &templates.Gemini, nil
	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// SetTemplate 设置指定平台的全局模板
func (s *CliConfigService) SetTemplate(platform string, template map[string]interface{}, isGlobalDefault bool) error {
	if err := s.requireHome(); err != nil {
		return err
	}

	templates, err := s.loadTemplates()
	if err != nil {
		// 如果文件不存在，创建新的模板
		templates = &CLITemplates{}
	}

	tpl := CLITemplate{
		Template:        template,
		IsGlobalDefault: isGlobalDefault,
	}

	switch CLIPlatform(platform) {
	case PlatformClaude:
		templates.Claude = tpl
	case PlatformCodex:
		templates.Codex = tpl
	case PlatformGemini:
		templates.Gemini = tpl
	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}

	return s.saveTemplates(templates)
}

func (s *CliConfigService) RenderTemplateEditorContent(
	platform string,
	template map[string]interface{},
) (*CLIEditorContent, error) {
	platformType := CLIPlatform(platform)
	value := normalizeTemplateEditableFields(platformType, template)

	switch platformType {
	case PlatformClaude, PlatformGemini:
		rendered, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化模板失败: %w", err)
		}
		return &CLIEditorContent{
			Format:  "json",
			Content: string(rendered),
		}, nil

	case PlatformCodex:
		if len(value) == 0 {
			return &CLIEditorContent{
				Format:  "toml",
				Content: "",
			}, nil
		}

		rendered, err := toml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex 模板失败: %w", err)
		}
		return &CLIEditorContent{
			Format:  "toml",
			Content: string(stripModelProvidersHeader(rendered)),
		}, nil

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

func (s *CliConfigService) NormalizeTemplateEditorContent(
	platform string,
	content string,
) (*CLINormalizedEditorContent, error) {
	platformType := CLIPlatform(platform)
	var editable map[string]interface{}

	switch platformType {
	case PlatformClaude, PlatformGemini:
		value, err := parseJSONRootObject(content)
		if err != nil {
			return nil, err
		}
		editable = value

	case PlatformCodex:
		trimmed := strings.TrimSpace(content)
		editable = make(map[string]interface{})
		if trimmed != "" {
			if err := toml.Unmarshal([]byte(trimmed), &editable); err != nil {
				return nil, fmt.Errorf("TOML 格式无效: %w", err)
			}
		}

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}

	editable = normalizeTemplateEditableFields(platformType, editable)
	rendered, err := s.RenderTemplateEditorContent(platform, editable)
	if err != nil {
		return nil, err
	}

	return &CLINormalizedEditorContent{
		Editable: editable,
		Format:   rendered.Format,
		Content:  rendered.Content,
	}, nil
}

func (s *CliConfigService) RenderEditorContent(
	platform string,
	editable map[string]interface{},
	apiURL string,
	apiKey string,
	providerName string,
) (*CLIEditorContent, error) {
	if err := s.requireHome(); err != nil {
		return nil, err
	}

	switch CLIPlatform(platform) {
	case PlatformClaude:
		value := stripClaudeLockedEditableFields(editable)
		envValue, _ := value["env"].(map[string]interface{})
		if envValue == nil {
			envValue = make(map[string]interface{})
		}

		if hasCLIEditorProviderInput(PlatformClaude, apiURL, apiKey) {
			envValue["ANTHROPIC_BASE_URL"] = normalizeURLTrimSlash(apiURL)
			envValue[claudeAuthTokenEnvKey] = apiKey
		} else {
			currentContent, err := os.ReadFile(s.getClaudeConfigPath())
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("读取 Claude 配置失败: %w", err)
			}
			if len(currentContent) > 0 {
				currentValue, err := parseJSONRootObject(string(currentContent))
				if err != nil {
					return nil, err
				}
				if currentEnv, ok := currentValue["env"].(map[string]interface{}); ok && currentEnv != nil {
					if baseURL := anyToString(currentEnv["ANTHROPIC_BASE_URL"]); baseURL != "" {
						envValue["ANTHROPIC_BASE_URL"] = baseURL
					}
					for _, key := range []string{claudeAuthTokenEnvKey, claudeAPIKeyEnvKey} {
						if value := anyToString(currentEnv[key]); value != "" {
							envValue[key] = value
						}
					}
				}
			}
		}

		if len(envValue) > 0 {
			value["env"] = envValue
		} else {
			delete(value, "env")
		}

		rendered, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("序列化 Claude 配置失败: %w", err)
		}
		return &CLIEditorContent{
			Format:       "json",
			Content:      string(rendered),
			LockedFields: buildClaudeEditorLockedFields(value, s.baseURL()),
		}, nil

	case PlatformCodex:
		directMode := hasCLIEditorProviderInput(PlatformCodex, apiURL, apiKey)
		raw := stripCodexLockedEditableFields(editable)
		if directMode {
			providerKey := resolveCodexEditorProviderKey(providerName, apiURL, apiKey)
			raw = stripCodexEditorManagedFields(editable, providerKey)
			raw["preferred_auth_method"] = codexPreferredAuth
			raw["model_provider"] = providerKey

			modelProviders := ensureTomlTable(raw, "model_providers")
			providerCfg := ensureProviderTable(modelProviders, providerKey)
			providerCfg["name"] = providerKey
			providerCfg["base_url"] = normalizeURLTrimSlash(apiURL)
			providerCfg["wire_api"] = codexWireAPI
			providerCfg["requires_openai_auth"] = false
			modelProviders[providerKey] = providerCfg
			raw["model_providers"] = modelProviders
		} else {
			currentContent, err := os.ReadFile(s.getCodexConfigPath())
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("读取 Codex 配置失败: %w", err)
			}

			currentRaw := make(map[string]interface{})
			if len(currentContent) > 0 {
				if err := toml.Unmarshal(currentContent, &currentRaw); err != nil {
					return nil, fmt.Errorf("解析 Codex 配置失败: %w", err)
				}
			}

			currentProviderKey := anyToString(currentRaw["model_provider"])
			if currentProviderKey != "" {
				raw["model_provider"] = currentProviderKey
			}

			currentPreferredAuth := anyToString(currentRaw["preferred_auth_method"])
			if currentPreferredAuth != "" {
				raw["preferred_auth_method"] = currentPreferredAuth
			}

			if currentProviderKey != "" {
				if currentProviders, ok := currentRaw["model_providers"].(map[string]interface{}); ok && currentProviders != nil {
					if currentProviderValue, exists := currentProviders[currentProviderKey]; exists {
						modelProviders := ensureTomlTable(raw, "model_providers")
						if providerMap, ok := currentProviderValue.(map[string]interface{}); ok && providerMap != nil {
							modelProviders[currentProviderKey] = cloneCLIEditableMap(providerMap)
						}
						raw["model_providers"] = modelProviders
					}
				}
			}
		}

		tomlBytes, err := toml.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("序列化 Codex 配置失败: %w", err)
		}

		return &CLIEditorContent{
			Format:       "toml",
			Content:      string(stripModelProvidersHeader(tomlBytes)),
			LockedFields: buildCodexEditorLockedFields(raw),
		}, nil

	case PlatformGemini:
		envMap := make(map[string]string)
		for key, value := range editable {
			if key == "GOOGLE_GEMINI_BASE_URL" || key == "GEMINI_API_KEY" {
				continue
			}
			envMap[key] = fmt.Sprintf("%v", value)
		}

		if hasCLIEditorProviderInput(PlatformGemini, apiURL, apiKey) {
			if strings.TrimSpace(apiURL) != "" {
				envMap["GOOGLE_GEMINI_BASE_URL"] = strings.TrimSpace(apiURL)
			}
			if strings.TrimSpace(apiKey) != "" {
				envMap["GEMINI_API_KEY"] = strings.TrimSpace(apiKey)
			}
		} else {
			currentContent, err := os.ReadFile(s.getGeminiEnvPath())
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("读取 Gemini 配置失败: %w", err)
			}
			if len(currentContent) > 0 {
				currentEnv := parseEnvFile(string(currentContent))
				if baseURL := strings.TrimSpace(currentEnv["GOOGLE_GEMINI_BASE_URL"]); baseURL != "" {
					envMap["GOOGLE_GEMINI_BASE_URL"] = baseURL
				}
				if token := strings.TrimSpace(currentEnv["GEMINI_API_KEY"]); token != "" {
					envMap["GEMINI_API_KEY"] = token
				}
			}
		}

		return &CLIEditorContent{
			Format:       "env",
			Content:      serializeEnvFile(envMap),
			LockedFields: buildGeminiEditorLockedFields(envMap),
		}, nil

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

func (s *CliConfigService) NormalizeEditorContent(
	platform string,
	content string,
	apiURL string,
	apiKey string,
	providerName string,
) (*CLINormalizedEditorContent, error) {
	if err := s.requireHome(); err != nil {
		return nil, err
	}

	switch CLIPlatform(platform) {
	case PlatformClaude:
		value, err := parseJSONRootObject(content)
		if err != nil {
			return nil, err
		}

		editable := stripClaudeLockedEditableFields(value)
		rendered, err := s.RenderEditorContent(platform, editable, apiURL, apiKey, providerName)
		if err != nil {
			return nil, err
		}

		return &CLINormalizedEditorContent{
			Editable:     editable,
			Format:       rendered.Format,
			Content:      rendered.Content,
			LockedFields: rendered.LockedFields,
		}, nil

	case PlatformCodex:
		trimmed := strings.TrimSpace(content)
		raw := make(map[string]interface{})
		if trimmed != "" {
			if err := toml.Unmarshal([]byte(trimmed), &raw); err != nil {
				return nil, fmt.Errorf("TOML 格式无效: %w", err)
			}
		}

		editable := stripCodexLockedEditableFields(raw)
		if hasCLIEditorProviderInput(PlatformCodex, apiURL, apiKey) {
			providerKey := resolveCodexEditorProviderKey(providerName, apiURL, apiKey)
			editable = stripCodexEditorManagedFields(raw, providerKey)
		}
		rendered, err := s.RenderEditorContent(platform, editable, apiURL, apiKey, providerName)
		if err != nil {
			return nil, err
		}

		return &CLINormalizedEditorContent{
			Editable:     editable,
			Format:       rendered.Format,
			Content:      rendered.Content,
			LockedFields: rendered.LockedFields,
		}, nil

	case PlatformGemini:
		envValue, err := parseEditorEnvContent(content)
		if err != nil {
			return nil, err
		}

		editable := stripGeminiLockedEditableFields(envValue)
		rendered, err := s.RenderEditorContent(platform, editable, apiURL, apiKey, providerName)
		if err != nil {
			return nil, err
		}

		return &CLINormalizedEditorContent{
			Editable:     editable,
			Format:       rendered.Format,
			Content:      rendered.Content,
			LockedFields: rendered.LockedFields,
		}, nil

	default:
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}
}

// RestoreDefault 恢复默认配置
func (s *CliConfigService) RestoreDefault(platform string) error {
	if err := s.requireHome(); err != nil {
		return err
	}

	p := CLIPlatform(platform)

	// 从备份恢复
	var configPath string
	switch p {
	case PlatformClaude:
		configPath = s.getClaudeConfigPath()
	case PlatformCodex:
		configPath = s.getCodexConfigPath()
	case PlatformGemini:
		configPath = s.getGeminiEnvPath()
	default:
		return fmt.Errorf("不支持的平台: %s", platform)
	}

	// 查找最新的备份文件（支持 *.bak.<timestamp> 格式）
	backupPath, err := FindLatestBackup(configPath)
	if err != nil {
		// 尝试兼容旧格式的备份文件
		switch p {
		case PlatformCodex:
			for _, legacy := range []string{
				filepath.Join(filepath.Dir(configPath), "code-switch.back.config.toml"),
				filepath.Join(filepath.Dir(configPath), legacyCodexBackupConfigName),
			} {
				if FileExists(legacy) {
					backupPath, err = legacy, nil
					break
				}
			}
		case PlatformGemini:
			legacy := configPath + ".code-switch.backup"
			if FileExists(legacy) {
				backupPath, err = legacy, nil
			}
		}
	}
	if err != nil {
		return err
	}

	return RestoreBackup(backupPath, configPath)
}

// baseURL 获取代理 URL
func (s *CliConfigService) baseURL() string {
	addr := strings.TrimSpace(s.relayAddr)
	if addr == "" {
		addr = ":18100"
	}
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "127.0.0.1" + host
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	return host
}

// geminiBaseURL 获取 Gemini 代理 URL（包含 /gemini 前缀）
func (s *CliConfigService) geminiBaseURL() string {
	return s.baseURL() + "/gemini"
}

// ========== Claude 配置操作 ==========

func (s *CliConfigService) getClaudeConfigPath() string {
	return filepath.Join(s.homeDir, ".claude", "settings.json")
}

func (s *CliConfigService) getClaudeConfig() (*CLIConfig, error) {
	configPath := s.getClaudeConfigPath()
	config := &CLIConfig{
		Platform:     PlatformClaude,
		ConfigFormat: "json",
		FilePath:     configPath,
		Fields:       []CLIConfigField{},
		Editable:     make(map[string]interface{}),
	}

	// 读取现有配置
	var data map[string]interface{}
	if content, err := os.ReadFile(configPath); err == nil {
		raw := string(content)
		config.RawContent = raw
		config.RawFiles = append(config.RawFiles, CLIConfigFile{
			Path:    configPath,
			Format:  "json",
			Content: raw,
		})
		if err := json.Unmarshal(content, &data); err != nil {
			return nil, fmt.Errorf("解析 Claude 配置失败: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 Claude 配置失败: %w", err)
	}

	// 构建字段列表
	config.Fields = append(config.Fields, buildClaudeEditorLockedFields(data, s.baseURL())...)

	// 可编辑字段
	env, _ := data["env"].(map[string]interface{})

	model := ""
	if m, ok := data["model"].(string); ok {
		model = m
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "model",
		Value:  model,
		Locked: false,
		Type:   "string",
	})
	config.Editable["model"] = model

	alwaysThinking := false
	if at, ok := data["alwaysThinkingEnabled"].(bool); ok {
		alwaysThinking = at
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "alwaysThinkingEnabled",
		Value:  fmt.Sprintf("%v", alwaysThinking),
		Locked: false,
		Type:   "boolean",
	})
	config.Editable["alwaysThinkingEnabled"] = alwaysThinking

	plugins := make(map[string]interface{})
	if ep, ok := data["enabledPlugins"].(map[string]interface{}); ok {
		plugins = ep
	}
	pluginsJSON, _ := json.Marshal(plugins)
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "enabledPlugins",
		Value:  string(pluginsJSON),
		Locked: false,
		Type:   "object",
	})
	config.Editable["enabledPlugins"] = plugins

	// 检查是否有其他未知的 env 变量（排除锁定的）
	if env != nil {
		proxyMode := isClaudeProxyBaseURL(env, s.baseURL())
		for k, v := range env {
			if k != "ANTHROPIC_BASE_URL" && k != claudeAuthTokenEnvKey && (!proxyMode || k != claudeAPIKeyEnvKey) {
				config.Fields = append(config.Fields, CLIConfigField{
					Key:    "env." + k,
					Value:  fmt.Sprintf("%v", v),
					Locked: false,
					Type:   "string",
				})
				if config.Editable["env"] == nil {
					config.Editable["env"] = make(map[string]interface{})
				}
				config.Editable["env"].(map[string]interface{})[k] = v
			}
		}
	}

	for k, v := range data {
		if k == "env" || k == "model" || k == "alwaysThinkingEnabled" || k == "enabledPlugins" {
			continue
		}
		config.Editable[k] = v
	}

	return config, nil
}

func (s *CliConfigService) saveClaudeConfig(editable map[string]interface{}, apiURL string, apiKey string) error {
	configPath := s.getClaudeConfigPath()
	data := make(map[string]interface{})

	// 创建备份
	if _, err := CreateBackup(configPath); err != nil {
		// 备份失败不阻止保存，只记录警告
		fmt.Printf("创建备份失败: %v\n", err)
	}

	env := make(map[string]interface{})
	directMode := hasCLIEditorProviderInput(PlatformClaude, apiURL, apiKey)
	if directMode {
		env["ANTHROPIC_BASE_URL"] = normalizeURLTrimSlash(apiURL)
		env[claudeAuthTokenEnvKey] = apiKey
	} else {
		env["ANTHROPIC_BASE_URL"] = s.baseURL()
		applyClaudeProxyAuthEnv(env, s.proxyAuthField())
	}

	// 锁定字段列表（这些字段不允许用户覆盖）
	lockedFields := map[string]bool{
		"env.ANTHROPIC_BASE_URL":   true,
		"env.ANTHROPIC_AUTH_TOKEN": true,
	}
	if !directMode {
		lockedFields["env.ANTHROPIC_API_KEY"] = true
	}

	// 合并用户编辑的所有字段（除了锁定字段）
	for k, v := range editable {
		// 跳过锁定字段
		if lockedFields[k] || lockedFields["env."+k] {
			continue
		}

		// 特殊处理 env：合并而不是覆盖
		if k == "env" {
			if customEnv, ok := v.(map[string]interface{}); ok {
				for ek, ev := range customEnv {
					if ek != "ANTHROPIC_BASE_URL" && ek != claudeAuthTokenEnvKey && (directMode || ek != claudeAPIKeyEnvKey) {
						env[ek] = ev
					}
				}
			}
			continue
		}

		// 其他字段直接覆盖
		data[k] = v
	}

	data["env"] = env

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(configPath)); err != nil {
		return err
	}

	// 原子写入
	return AtomicWriteJSON(configPath, data)
}

// saveClaudeConfigContent 将预览区编辑的 settings.json 写入磁盘，并强制覆盖代理锁定字段
func (s *CliConfigService) saveClaudeConfigContent(configPath string, content string) error {
	data := make(map[string]interface{})
	// 空内容允许，视为从空配置开始
	if strings.TrimSpace(content) != "" {
		if err := json.Unmarshal([]byte(content), &data); err != nil {
			return fmt.Errorf("解析 Claude 配置失败: %w", err)
		}
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	// 强制写入锁定字段
	env, _ := data["env"].(map[string]interface{})
	if env == nil {
		env = make(map[string]interface{})
	}
	env["ANTHROPIC_BASE_URL"] = s.baseURL()
	applyClaudeProxyAuthEnv(env, s.proxyAuthField())
	data["env"] = env

	// 创建备份（文件不存在时 CreateBackup 会返回空路径并忽略）
	if _, err := CreateBackup(configPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(configPath)); err != nil {
		return err
	}

	return AtomicWriteJSON(configPath, data)
}

// ========== Codex 配置操作 ==========

func (s *CliConfigService) getCodexConfigPath() string {
	return filepath.Join(s.homeDir, ".codex", "config.toml")
}

func (s *CliConfigService) getCodexAuthPath() string {
	return filepath.Join(s.homeDir, ".codex", "auth.json")
}

func (s *CliConfigService) getCodexConfig() (*CLIConfig, error) {
	configPath := s.getCodexConfigPath()
	config := &CLIConfig{
		Platform:     PlatformCodex,
		ConfigFormat: "toml",
		FilePath:     configPath,
		Fields:       []CLIConfigField{},
		Editable:     make(map[string]interface{}),
	}

	// 读取现有配置
	var data map[string]interface{}
	if content, err := os.ReadFile(configPath); err == nil {
		raw := string(content)
		config.RawContent = raw
		config.RawFiles = append(config.RawFiles, CLIConfigFile{
			Path:    configPath,
			Format:  "toml",
			Content: raw,
		})
		if err := toml.Unmarshal(content, &data); err != nil {
			return nil, fmt.Errorf("解析 Codex 配置失败: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 Codex 配置失败: %w", err)
	}

	// 读取 auth.json 预览
	authPath := s.getCodexAuthPath()
	if authContent, err := os.ReadFile(authPath); err == nil {
		config.RawFiles = append(config.RawFiles, CLIConfigFile{
			Path:    authPath,
			Format:  "json",
			Content: string(authContent),
		})
	}

	config.Fields = append(config.Fields, buildCodexEditorLockedFields(data)...)

	// 可编辑字段
	model := "gpt-5-codex"
	if m, ok := data["model"].(string); ok {
		model = m
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "model",
		Value:  model,
		Locked: false,
		Type:   "string",
	})
	config.Editable["model"] = model

	reasoningEffort := "xhigh"
	if re, ok := data["model_reasoning_effort"].(string); ok {
		reasoningEffort = re
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "model_reasoning_effort",
		Value:  reasoningEffort,
		Locked: false,
		Type:   "string",
	})
	config.Editable["model_reasoning_effort"] = reasoningEffort

	disableStorage := true
	if ds, ok := data["disable_response_storage"].(bool); ok {
		disableStorage = ds
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "disable_response_storage",
		Value:  fmt.Sprintf("%v", disableStorage),
		Locked: false,
		Type:   "boolean",
	})
	config.Editable["disable_response_storage"] = disableStorage

	for key, value := range data {
		switch key {
		case "model", "model_reasoning_effort", "disable_response_storage", "model_provider", "preferred_auth_method":
			continue
		case "model_providers":
			if modelProviders, ok := value.(map[string]interface{}); ok && modelProviders != nil {
				nextProviders := cloneCLIEditableMap(modelProviders)
				delete(nextProviders, codexProviderKey)
				if len(nextProviders) > 0 {
					config.Editable[key] = nextProviders
				}
			}
		default:
			config.Editable[key] = value
		}
	}

	return config, nil
}

func (s *CliConfigService) saveCodexConfig(
	editable map[string]interface{},
	apiURL string,
	apiKey string,
	providerName string,
) error {
	configPath := s.getCodexConfigPath()
	authPath := s.getCodexAuthPath()
	raw := stripCodexLockedEditableFields(editable)
	if raw == nil {
		raw = make(map[string]interface{})
	}

	// 创建备份
	if _, err := CreateBackup(configPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}
	if _, err := CreateBackup(authPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	authPayload := make(map[string]interface{})
	if content, err := os.ReadFile(authPath); err == nil && strings.TrimSpace(string(content)) != "" {
		if err := json.Unmarshal(content, &authPayload); err != nil {
			authPayload = make(map[string]interface{})
		}
	}
	if authPayload == nil {
		authPayload = make(map[string]interface{})
	}

	raw["preferred_auth_method"] = codexPreferredAuth

	modelProviders := ensureTomlTable(raw, "model_providers")
	if hasCLIEditorProviderInput(PlatformCodex, apiURL, apiKey) {
		providerKey := resolveCodexEditorProviderKey(providerName, apiURL, apiKey)
		raw["model_provider"] = providerKey

		provider := ensureProviderTable(modelProviders, providerKey)
		provider["name"] = providerKey
		provider["base_url"] = normalizeURLTrimSlash(apiURL)
		provider["wire_api"] = codexWireAPI
		provider["requires_openai_auth"] = false
		modelProviders[providerKey] = provider

		authPayload["OPENAI_API_KEY"] = apiKey
	} else {
		raw["model_provider"] = codexProviderKey
		if _, exists := raw["model"]; !exists {
			raw["model"] = codexDefaultModel
		}

		provider := ensureProviderTable(modelProviders, codexProviderKey)
		provider["name"] = codexProviderKey
		provider["base_url"] = s.baseURL()
		provider["wire_api"] = codexWireAPI
		provider["requires_openai_auth"] = false
		modelProviders[codexProviderKey] = provider

		authPayload["OPENAI_API_KEY"] = codexTokenValue
	}
	raw["model_providers"] = modelProviders

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(configPath)); err != nil {
		return err
	}
	if err := EnsureDir(filepath.Dir(authPath)); err != nil {
		return err
	}

	// 序列化 TOML
	tomlData, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化 TOML 失败: %w", err)
	}

	// 清理多余的 [model_providers] 头
	cleaned := stripModelProvidersHeader(tomlData)

	if err := AtomicWriteBytes(configPath, cleaned); err != nil {
		return err
	}

	return AtomicWriteJSON(authPath, authPayload)
}

// saveCodexConfigContent 将预览区编辑的 config.toml 写入磁盘，并强制覆盖代理锁定字段
func (s *CliConfigService) saveCodexConfigContent(configPath string, content string) error {
	raw := make(map[string]interface{})
	// 空内容允许，视为从空配置开始
	if strings.TrimSpace(content) != "" {
		if err := toml.Unmarshal([]byte(content), &raw); err != nil {
			return fmt.Errorf("解析 Codex 配置失败: %w", err)
		}
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}

	if _, err := CreateBackup(configPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	// 强制写入锁定字段
	raw["model_provider"] = "code-switch-r"
	raw["preferred_auth_method"] = "apikey"

	// 确保 model_providers.code-switch-r 存在并写入锁定字段
	modelProviders, ok := raw["model_providers"].(map[string]interface{})
	if !ok || modelProviders == nil {
		modelProviders = make(map[string]interface{})
	}
	provider, ok := modelProviders["code-switch-r"].(map[string]interface{})
	if !ok || provider == nil {
		provider = make(map[string]interface{})
	}
	provider["name"] = "code-switch-r"
	provider["base_url"] = s.baseURL()
	provider["wire_api"] = "responses"
	provider["requires_openai_auth"] = false
	modelProviders["code-switch-r"] = provider
	raw["model_providers"] = modelProviders

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(configPath)); err != nil {
		return err
	}

	tomlData, err := toml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("序列化 TOML 失败: %w", err)
	}
	cleaned := stripModelProvidersHeader(tomlData)
	return AtomicWriteBytes(configPath, cleaned)
}

// saveCodexAuthContent 保存 Codex auth.json（仅做 JSON 校验，不强制覆盖内容）
func (s *CliConfigService) saveCodexAuthContent(authPath string, content string) error {
	data := make(map[string]interface{})
	// 空内容允许（可用于清空/重建）
	if strings.TrimSpace(content) != "" {
		if err := json.Unmarshal([]byte(content), &data); err != nil {
			return fmt.Errorf("解析 Codex auth.json 失败: %w", err)
		}
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	if _, err := CreateBackup(authPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(authPath)); err != nil {
		return err
	}

	return AtomicWriteJSON(authPath, data)
}

// ========== Gemini 配置操作 ==========

func (s *CliConfigService) getGeminiEnvPath() string {
	return filepath.Join(s.homeDir, ".gemini", ".env")
}

func (s *CliConfigService) getGeminiConfig() (*CLIConfig, error) {
	envPath := s.getGeminiEnvPath()
	config := &CLIConfig{
		Platform:     PlatformGemini,
		ConfigFormat: "env",
		FilePath:     envPath,
		Fields:       []CLIConfigField{},
		Editable:     make(map[string]interface{}),
		EnvContent:   make(map[string]string),
	}

	// 读取 .env 文件
	if content, err := os.ReadFile(envPath); err == nil {
		raw := string(content)
		config.RawContent = raw
		config.RawFiles = append(config.RawFiles, CLIConfigFile{
			Path:    envPath,
			Format:  "env",
			Content: raw,
		})
		config.EnvContent = parseEnvFile(raw)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("读取 Gemini .env 失败: %w", err)
	}

	config.Fields = append(config.Fields, buildGeminiEditorLockedFields(config.EnvContent)...)

	model := config.EnvContent["GEMINI_MODEL"]
	if model == "" {
		model = "gemini-3-pro-preview"
	}
	config.Fields = append(config.Fields, CLIConfigField{
		Key:    "GEMINI_MODEL",
		Value:  model,
		Locked: false,
		Type:   "string",
	})
	config.Editable["GEMINI_MODEL"] = model

	// 其他自定义环境变量
	for k, v := range config.EnvContent {
		if k != "GOOGLE_GEMINI_BASE_URL" && k != "GEMINI_API_KEY" && k != "GEMINI_MODEL" {
			config.Fields = append(config.Fields, CLIConfigField{
				Key:    k,
				Value:  v,
				Locked: false,
				Type:   "string",
			})
			config.Editable[k] = v
		}
	}

	return config, nil
}

func (s *CliConfigService) saveGeminiConfig(editable map[string]interface{}, apiURL string, apiKey string) error {
	envPath := s.getGeminiEnvPath()

	envMap := make(map[string]string)

	if content, err := os.ReadFile(envPath); err == nil {
		currentEnv := parseEnvFile(string(content))
		if apiKey := currentEnv["GEMINI_API_KEY"]; apiKey != "" {
			envMap["GEMINI_API_KEY"] = apiKey
		}
	}

	// 创建备份
	if _, err := CreateBackup(envPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	if hasCLIEditorProviderInput(PlatformGemini, apiURL, apiKey) {
		if trimmedURL := strings.TrimSpace(apiURL); trimmedURL != "" {
			envMap["GOOGLE_GEMINI_BASE_URL"] = trimmedURL
		} else {
			delete(envMap, "GOOGLE_GEMINI_BASE_URL")
		}
		if trimmedKey := strings.TrimSpace(apiKey); trimmedKey != "" {
			envMap["GEMINI_API_KEY"] = trimmedKey
		} else {
			delete(envMap, "GEMINI_API_KEY")
		}
	} else {
		envMap["GOOGLE_GEMINI_BASE_URL"] = s.geminiBaseURL()
		envMap["GEMINI_API_KEY"] = codexTokenValue
	}

	// 以当前编辑结果为完整来源，只保留非锁定字段
	for k, v := range editable {
		if k == "GOOGLE_GEMINI_BASE_URL" || k == "GEMINI_API_KEY" {
			continue
		}
		if str, ok := v.(string); ok {
			envMap[k] = str
		} else {
			envMap[k] = fmt.Sprintf("%v", v)
		}
	}

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(envPath)); err != nil {
		return err
	}

	// 序列化为 .env 格式
	content := serializeEnvFile(envMap)

	// 原子写入
	return AtomicWriteText(envPath, content)
}

// saveGeminiEnvContent 将预览区编辑的 .env 写入磁盘，并强制覆盖代理锁定字段
func (s *CliConfigService) saveGeminiEnvContent(envPath string, content string) error {
	envMap := parseEnvFile(content)

	// 强制写入锁定字段
	envMap["GOOGLE_GEMINI_BASE_URL"] = s.geminiBaseURL()

	// GEMINI_API_KEY 为系统锁定字段：优先保留磁盘中的现有值；不存在时写入占位值
	existingAPIKey := ""
	if oldContent, err := os.ReadFile(envPath); err == nil {
		oldMap := parseEnvFile(string(oldContent))
		existingAPIKey = oldMap["GEMINI_API_KEY"]
	}
	if existingAPIKey != "" {
		envMap["GEMINI_API_KEY"] = existingAPIKey
	} else if envMap["GEMINI_API_KEY"] == "" {
		envMap["GEMINI_API_KEY"] = "code-switch-r"
	}

	if _, err := CreateBackup(envPath); err != nil {
		fmt.Printf("创建备份失败: %v\n", err)
	}

	// 确保目录存在
	if err := EnsureDir(filepath.Dir(envPath)); err != nil {
		return err
	}

	// 原子写入
	return AtomicWriteText(envPath, serializeEnvFile(envMap))
}

// ========== 模板管理 ==========

func (s *CliConfigService) loadTemplates() (*CLITemplates, error) {
	path := s.getTemplatesPath()
	var templates CLITemplates

	if err := ReadJSONFile(path, &templates); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// 返回空模板
			return &CLITemplates{}, nil
		}
		return nil, err
	}

	return &templates, nil
}

func (s *CliConfigService) saveTemplates(templates *CLITemplates) error {
	path := s.getTemplatesPath()
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	return AtomicWriteJSON(path, templates)
}

// ========== 辅助函数 ==========

// serializeEnvFile 将 map 序列化为 .env 格式
func serializeEnvFile(envMap map[string]string) string {
	var lines []string

	// 按键排序以保证输出稳定
	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	// 简单排序
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}

	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, envMap[key]))
	}

	return strings.Join(lines, "\n")
}

// samePath 跨平台路径比较（Windows 大小写不敏感）
func samePath(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// 注意: parseEnvFile 和 isValidEnvKey 已在 geminiservice.go 中定义
