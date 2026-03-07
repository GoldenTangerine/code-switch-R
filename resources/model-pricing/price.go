package modelpricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode"
)

//go:embed model_prices_and_context_window.json
var pricingFile []byte

var (
	defaultOnce    sync.Once
	defaultService *Service
	defaultErr     error
	nameReplacer   = strings.NewReplacer("-", "", "_", "", ".", "", ":", "", "/", "", " ", "")
)

// Service 提供模型价格相关的计算能力。
type Service struct {
	pricingMap   map[string]*PricingEntry
	normalized   map[string]string
	ephemeral1h  map[string]float64
	longContexts map[string]LongContextPricing
}

// PricingEntry 映射 JSON 内的字段。
type PricingEntry struct {
	InputCostPerToken                   float64 `json:"input_cost_per_token"`
	OutputCostPerToken                  float64 `json:"output_cost_per_token"`
	OutputCostPerReasoningToken         float64 `json:"output_cost_per_reasoning_token"`
	CacheCreationInputTokenCost         float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove1Hr float64 `json:"cache_creation_input_token_cost_above_1hr"`
	CacheCreationInputTokenCostAbove200 float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`
	CacheReadInputTokenCost             float64 `json:"cache_read_input_token_cost"`
	InputCostPerTokenAbove200k          float64 `json:"input_cost_per_token_above_200k_tokens"`
	InputCostPerTokenAbove128k          float64 `json:"input_cost_per_token_above_128k_tokens"`
	OutputCostPerTokenAbove200k         float64 `json:"output_cost_per_token_above_200k_tokens"`
}

// UsageSnapshot 描述一次请求的 token 用量。
type UsageSnapshot struct {
	InputTokens       int
	OutputTokens      int
	ReasoningTokens   int
	CacheCreateTokens int
	CacheReadTokens   int
	CacheCreation     *CacheCreationDetail
}

// CacheCreationDetail 细分缓存创建 tokens。
type CacheCreationDetail struct {
	Ephemeral5mTokens int
	Ephemeral1hTokens int
}

// CostBreakdown 表示一次费用计算的结果。
type CostBreakdown struct {
	InputCost       float64 `json:"input_cost"`
	OutputCost      float64 `json:"output_cost"`
	ReasoningCost   float64 `json:"reasoning_cost"`
	CacheCreateCost float64 `json:"cache_create_cost"`
	CacheReadCost   float64 `json:"cache_read_cost"`
	Ephemeral5mCost float64 `json:"ephemeral_5m_cost"`
	Ephemeral1hCost float64 `json:"ephemeral_1h_cost"`
	TotalCost       float64 `json:"total_cost"`
	HasPricing      bool    `json:"has_pricing"`
	IsLongContext   bool    `json:"is_long_context"`
	PricingModel    string  `json:"pricing_model"`
	FuzzyMatched    bool    `json:"fuzzy_matched"`
}

// LongContextPricing 描述 1M 上下文模型的单价。
type LongContextPricing struct {
	Input  float64
	Output float64
}

// DefaultService 返回单例。
func DefaultService() (*Service, error) {
	defaultOnce.Do(func() {
		defaultService, defaultErr = NewService()
	})
	return defaultService, defaultErr
}

// NewService 从嵌入的 JSON 创建服务实例。
func NewService() (*Service, error) {
	raw := make(map[string]PricingEntry)
	if err := json.Unmarshal(pricingFile, &raw); err != nil {
		return nil, fmt.Errorf("parse pricing file: %w", err)
	}
	pricing := make(map[string]*PricingEntry, len(raw))
	normalized := make(map[string]string, len(raw))
	for key, entry := range raw {
		item := entry
		ensureCachePricing(&item)
		pricing[key] = &item
		norm := normalizeName(key)
		if _, exists := normalized[norm]; !exists {
			normalized[norm] = key
		}
	}
	return &Service{
		pricingMap:   pricing,
		normalized:   normalized,
		ephemeral1h:  buildEphemeral1hPricing(),
		longContexts: buildLongContextPricing(),
	}, nil
}

// CalculateCost 根据模型与 token 用量返回费用明细（美元）。
func (s *Service) CalculateCost(model string, usage UsageSnapshot) CostBreakdown {
	if s == nil || model == "" {
		return CostBreakdown{}
	}
	entry, pricingModel, hasPricing, fuzzyMatched := s.getPricingWithKey(model)
	breakdown := CostBreakdown{
		HasPricing:   hasPricing,
		PricingModel: pricingModel,
		FuzzyMatched: fuzzyMatched,
	}
	if entry == nil && !strings.Contains(strings.ToLower(model), "[1m]") {
		return breakdown
	}
	longTier, useLong := s.longContextTier(model, usage)
	if entry == nil {
		entry = &PricingEntry{}
	}
	if useLong {
		breakdown.IsLongContext = true
		breakdown.InputCost = float64(usage.InputTokens) * longTier.Input
		breakdown.OutputCost = float64(usage.OutputTokens) * longTier.Output
	} else {
		breakdown.InputCost = float64(usage.InputTokens) * entry.InputCostPerToken
		breakdown.OutputCost = float64(usage.OutputTokens) * entry.OutputCostPerToken
	}
	// Reasoning tokens cost (for Gemini thinking models, Codex o1/o3, etc.)
	if usage.ReasoningTokens > 0 && entry.OutputCostPerReasoningToken > 0 {
		breakdown.ReasoningCost = float64(usage.ReasoningTokens) * entry.OutputCostPerReasoningToken
	}
	cacheCreateTokens, cache1hTokens := resolveCacheTokens(usage)
	cachePricingModel := model
	if strings.TrimSpace(pricingModel) != "" {
		cachePricingModel = pricingModel
	}
	cache5mCost := float64(cacheCreateTokens) * entry.CacheCreationInputTokenCost
	cache1hCost := float64(cache1hTokens) * s.getEphemeral1hPricing(cachePricingModel)
	breakdown.Ephemeral5mCost = cache5mCost
	breakdown.Ephemeral1hCost = cache1hCost
	breakdown.CacheCreateCost = cache5mCost + cache1hCost
	breakdown.CacheReadCost = float64(usage.CacheReadTokens) * entry.CacheReadInputTokenCost
	breakdown.TotalCost = breakdown.InputCost + breakdown.OutputCost + breakdown.ReasoningCost + breakdown.CacheCreateCost + breakdown.CacheReadCost
	if breakdown.TotalCost > 0 {
		breakdown.HasPricing = true
	}
	return breakdown
}

func (s *Service) getPricing(model string) (*PricingEntry, bool) {
	entry, _, hasPricing, _ := s.getPricingWithKey(model)
	return entry, hasPricing
}

func (s *Service) getPricingWithKey(model string) (*PricingEntry, string, bool, bool) {
	if model == "" {
		return nil, "", false, false
	}
	if entry, ok := s.pricingMap[model]; ok {
		return entry, model, true, false
	}
	if model == "gpt-5-codex" {
		if entry, ok := s.pricingMap["gpt-5"]; ok {
			return entry, "gpt-5", true, false
		}
	}
	withoutRegion := stripRegionPrefix(model)
	if entry, ok := s.pricingMap[withoutRegion]; ok {
		return entry, withoutRegion, true, false
	}
	withoutProvider := strings.TrimPrefix(withoutRegion, "anthropic.")
	if entry, ok := s.pricingMap[withoutProvider]; ok {
		return entry, withoutProvider, true, false
	}
	normalizedTarget := normalizeName(model)
	if key, ok := s.normalized[normalizedTarget]; ok {
		return s.pricingMap[key], key, true, false
	}
	if key, ok := s.pickBestPricingMatch(model, true); ok {
		if entry, exists := s.pricingMap[key]; exists {
			return entry, key, true, true
		}
	}
	if key, ok := s.pickBestPricingMatch(model, false); ok {
		if entry, exists := s.pricingMap[key]; exists {
			return entry, key, true, true
		}
	}
	return nil, "", false, false
}

func (s *Service) pickBestPricingMatch(model string, containOnly bool) (string, bool) {
	if s == nil || strings.TrimSpace(model) == "" {
		return "", false
	}

	targetNorm := normalizeName(model)
	if targetNorm == "" {
		return "", false
	}
	targetTokens := tokenizeModelName(model)
	familyHint := selectFamilyHint(targetTokens)

	bestKey := ""
	bestScore := -1.0
	for key := range s.pricingMap {
		normKey := normalizeName(key)
		if normKey == "" {
			continue
		}

		if containOnly && !(strings.Contains(normKey, targetNorm) || strings.Contains(targetNorm, normKey)) {
			continue
		}

		if familyHint != "" && !hasFamilyHint(normKey, key, familyHint) {
			continue
		}

		keyTokens := tokenizeModelName(key)
		score := pricingSimilarityScore(targetNorm, normKey, targetTokens, keyTokens)
		if score > bestScore || (math.Abs(score-bestScore) < 1e-9 && bestKey != "" && key < bestKey) {
			bestScore = score
			bestKey = key
		}
	}

	if bestKey == "" {
		return "", false
	}

	minScore := 0.62
	if containOnly {
		minScore = 0.55
	}
	if bestScore < minScore {
		return "", false
	}
	return bestKey, true
}

func pricingSimilarityScore(targetNorm string, keyNorm string, targetTokens []string, keyTokens []string) float64 {
	if targetNorm == "" || keyNorm == "" {
		return 0
	}

	maxLen := max(len(targetNorm), len(keyNorm))
	if maxLen == 0 {
		return 0
	}

	editDistance := levenshteinDistance(targetNorm, keyNorm)
	editSimilarity := 1 - float64(editDistance)/float64(maxLen)
	if editSimilarity < 0 {
		editSimilarity = 0
	}

	prefixSimilarity := float64(commonPrefixLength(targetNorm, keyNorm)) / float64(maxLen)
	tokenSimilarity := tokenJaccardSimilarity(targetTokens, keyTokens)

	return editSimilarity*0.65 + tokenSimilarity*0.25 + prefixSimilarity*0.10
}

func tokenizeModelName(name string) []string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return nil
	}

	tokens := make([]string, 0, 8)
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		token := builder.String()
		builder.Reset()
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	for _, r := range normalized {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func selectFamilyHint(tokens []string) string {
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || isProviderToken(token) || isPureNumberToken(token) {
			continue
		}
		if len(token) >= 3 {
			return token
		}
	}
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || isProviderToken(token) || isPureNumberToken(token) {
			continue
		}
		if len(token) >= 2 {
			return token
		}
	}
	return ""
}

func hasFamilyHint(normKey string, rawKey string, familyHint string) bool {
	if familyHint == "" {
		return true
	}
	if strings.Contains(normKey, familyHint) || strings.Contains(strings.ToLower(rawKey), familyHint) {
		return true
	}
	for _, token := range tokenizeModelName(rawKey) {
		if token == familyHint {
			return true
		}
	}
	return false
}

func isProviderToken(token string) bool {
	switch token {
	case "anthropic", "openai", "google", "meta", "vertex", "vertexai", "azure", "xai", "mistral", "deepseek", "qwen", "alibaba", "moonshot", "kimi":
		return true
	default:
		return false
	}
}

func isPureNumberToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func commonPrefixLength(a string, b string) int {
	maxPrefix := min(len(a), len(b))
	for idx := 0; idx < maxPrefix; idx++ {
		if a[idx] != b[idx] {
			return idx
		}
	}
	return maxPrefix
}

func tokenJaccardSimilarity(a []string, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	setA := make(map[string]struct{}, len(a))
	for _, token := range a {
		if token == "" {
			continue
		}
		setA[token] = struct{}{}
	}
	if len(setA) == 0 {
		return 0
	}

	setB := make(map[string]struct{}, len(b))
	for _, token := range b {
		if token == "" {
			continue
		}
		setB[token] = struct{}{}
	}
	if len(setB) == 0 {
		return 0
	}

	intersection := 0
	for token := range setA {
		if _, ok := setB[token]; ok {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union <= 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func levenshteinDistance(a string, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			current[j] = min(
				current[j-1]+1,
				prev[j]+1,
				prev[j-1]+cost,
			)
		}
		prev = current
	}
	return prev[len(b)]
}

func min(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *Service) longContextTier(model string, usage UsageSnapshot) (LongContextPricing, bool) {
	totalInput := usage.InputTokens + usage.CacheCreateTokens + usage.CacheReadTokens
	if strings.Contains(strings.ToLower(model), "[1m]") && totalInput > 200000 && len(s.longContexts) > 0 {
		if tier, ok := s.longContexts[model]; ok {
			return tier, true
		}
		for _, tier := range s.longContexts {
			return tier, true
		}
	}
	return LongContextPricing{}, false
}

func (s *Service) explicitEphemeral1hPricing(model string) (float64, bool) {
	if s == nil || strings.TrimSpace(model) == "" {
		return 0, false
	}
	if price, ok := s.ephemeral1h[model]; ok && price > 0 {
		return price, true
	}
	entry, pricingModel, hasPricing, _ := s.getPricingWithKey(model)
	if !hasPricing {
		return 0, false
	}
	if strings.TrimSpace(pricingModel) != "" {
		if price, ok := s.ephemeral1h[pricingModel]; ok && price > 0 {
			return price, true
		}
	}
	if entry != nil && entry.CacheCreationInputTokenCostAbove1Hr > 0 {
		return entry.CacheCreationInputTokenCostAbove1Hr, true
	}
	return 0, false
}

func fallbackEphemeral1hPricing(model string) float64 {
	name := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.Contains(name, "opus"):
		return 0.00003
	case strings.Contains(name, "sonnet"):
		return 0.000006
	case strings.Contains(name, "haiku"):
		return 0.0000016
	default:
		return 0
	}
}

func (s *Service) getEphemeral1hPricing(model string) float64 {
	if price, ok := s.explicitEphemeral1hPricing(model); ok {
		return price
	}
	return fallbackEphemeral1hPricing(model)
}

func ensureCachePricing(entry *PricingEntry) {
	if entry == nil {
		return
	}
	if entry.CacheCreationInputTokenCost == 0 && entry.InputCostPerToken > 0 {
		entry.CacheCreationInputTokenCost = entry.InputCostPerToken * 1.25
	}
	if entry.CacheReadInputTokenCost == 0 && entry.InputCostPerToken > 0 {
		entry.CacheReadInputTokenCost = entry.InputCostPerToken * 0.1
	}
}

func stripRegionPrefix(name string) string {
	for _, prefix := range []string{"us.", "eu.", "apac."} {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			return name[len(prefix):]
		}
	}
	return name
}

func normalizeName(name string) string {
	return nameReplacer.Replace(strings.ToLower(name))
}

func resolveCacheTokens(usage UsageSnapshot) (fiveMin int, oneHour int) {
	if usage.CacheCreation == nil {
		return usage.CacheCreateTokens, 0
	}
	five := usage.CacheCreation.Ephemeral5mTokens
	one := usage.CacheCreation.Ephemeral1hTokens
	remaining := usage.CacheCreateTokens - five - one
	if remaining > 0 {
		five += remaining
	}
	if five < 0 {
		five = 0
	}
	if one < 0 {
		one = 0
	}
	return five, one
}

func buildEphemeral1hPricing() map[string]float64 {
	return map[string]float64{
		"claude-opus-4-5":            0.00001,
		"claude-opus-4-5-20251101":   0.00001,
		"claude-opus-4-5-20250929":   0.00001,
		"claude-opus-4-1":            0.00003,
		"claude-opus-4-1-20250805":   0.00003,
		"claude-opus-4":              0.00003,
		"claude-opus-4-20250514":     0.00003,
		"claude-3-opus":              0.00003,
		"claude-3-opus-latest":       0.00003,
		"claude-3-opus-20240229":     0.00003,
		"claude-3-5-sonnet":          0.000006,
		"claude-3-5-sonnet-latest":   0.000006,
		"claude-3-5-sonnet-20241022": 0.000006,
		"claude-3-5-sonnet-20240620": 0.000006,
		"claude-3-sonnet":            0.000006,
		"claude-3-sonnet-20240307":   0.000006,
		"claude-sonnet-3":            0.000006,
		"claude-sonnet-3-5":          0.000006,
		"claude-sonnet-3-7":          0.000006,
		"claude-sonnet-4":            0.000006,
		"claude-sonnet-4-20250514":   0.000006,
		"claude-3-5-haiku":           0.0000016,
		"claude-3-5-haiku-latest":    0.0000016,
		"claude-3-5-haiku-20241022":  0.0000016,
		"claude-3-haiku":             0.0000016,
		"claude-3-haiku-20240307":    0.0000016,
		"claude-haiku-3":             0.0000016,
		"claude-haiku-3-5":           0.0000016,
		"claude-haiku-4-5":           0.000002,
		"claude-haiku-4-5-20251001":  0.000002,
	}
}

func buildLongContextPricing() map[string]LongContextPricing {
	return map[string]LongContextPricing{
		"claude-sonnet-4-20250514[1m]": {
			Input:  0.000006,
			Output: 0.0000225,
		},
	}
}

// Clone 深拷贝 Service（用于在默认价格表基础上叠加自定义覆盖层）。
// 注意：Service 在构建完成后应视为只读；自定义覆盖应通过 Clone 后的新实例来应用。
func (s *Service) Clone() *Service {
	if s == nil {
		return nil
	}

	pricing := make(map[string]*PricingEntry, len(s.pricingMap))
	for key, entry := range s.pricingMap {
		if entry == nil {
			continue
		}
		copied := *entry
		pricing[key] = &copied
	}

	normalized := make(map[string]string, len(s.normalized))
	for key, value := range s.normalized {
		normalized[key] = value
	}

	ephemeral1h := make(map[string]float64, len(s.ephemeral1h))
	for key, value := range s.ephemeral1h {
		ephemeral1h[key] = value
	}

	longContexts := make(map[string]LongContextPricing, len(s.longContexts))
	for key, value := range s.longContexts {
		longContexts[key] = value
	}

	return &Service{
		pricingMap:   pricing,
		normalized:   normalized,
		ephemeral1h:  ephemeral1h,
		longContexts: longContexts,
	}
}

// Models 返回当前 Service 的所有模型 key（未排序）。
func (s *Service) Models() []string {
	if s == nil {
		return nil
	}
	keys := make([]string, 0, len(s.pricingMap))
	for key := range s.pricingMap {
		keys = append(keys, key)
	}
	return keys
}

// PricingEntryExact 返回指定模型 key 的价格条目（仅精确匹配）。
func (s *Service) PricingEntryExact(model string) (PricingEntry, bool) {
	if s == nil || model == "" {
		return PricingEntry{}, false
	}
	entry, ok := s.pricingMap[model]
	if !ok || entry == nil {
		return PricingEntry{}, false
	}
	return *entry, true
}

// Ephemeral1hCostPerToken 返回 1h cache 创建的单价（美元/Token），包含 fallback 逻辑。
func (s *Service) Ephemeral1hCostPerToken(model string) float64 {
	if s == nil || model == "" {
		return 0
	}
	return s.getEphemeral1hPricing(model)
}

// ExplicitEphemeral1hCostPerToken 返回显式声明的 1h cache 创建单价（美元/Token），
// 不使用 family fallback，只返回明确存在的 1h 价格。
func (s *Service) ExplicitEphemeral1hCostPerToken(model string) (float64, bool) {
	return s.explicitEphemeral1hPricing(model)
}

// ApplyOverrides 将覆盖层应用到 Service（会覆盖同名 model key）。
// 输入为完整条目（per-token），用于“自定义价目表”场景；调用方应在 Clone 后的新实例上使用。
func (s *Service) ApplyOverrides(pricingOverrides map[string]PricingEntry, ephemeral1hOverrides map[string]float64) {
	if s == nil {
		return
	}

	for model, entry := range pricingOverrides {
		item := entry
		// 兼容：用户只填了 input/output 时，自动补齐 cache 定价（与默认行为一致）。
		ensureCachePricing(&item)
		s.pricingMap[model] = &item

		normalized := normalizeName(model)
		s.normalized[normalized] = model
	}

	for model, cost := range ephemeral1hOverrides {
		if s.ephemeral1h == nil {
			s.ephemeral1h = make(map[string]float64)
		}
		s.ephemeral1h[model] = cost
	}
}
