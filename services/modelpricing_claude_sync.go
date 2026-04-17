package services

import (
	"fmt"
	stdhtml "html"
	"io"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	modelpricing "codeswitch/resources/model-pricing"

	xhtml "golang.org/x/net/html"
)

const (
	claudePricingPageURL      = "https://platform.claude.com/docs/en/about-claude/pricing"
	claudePricingPageMaxBytes = 8 * 1024 * 1024
	claudePreviewCacheTTL     = 30 * time.Minute
)

const (
	claudeHeaderModel        = "model"
	claudeHeaderBaseInput    = "baseinput"
	claudeHeaderCacheWrite5m = "5mcachewrites"
	claudeHeaderCacheWrite1h = "1hcachewrites"
	claudeHeaderCacheHits    = "cachehitsrefreshes"
	claudeHeaderOutput       = "output"
)

var numericValuePattern = regexp.MustCompile(`[-+]?\d+(?:\.\d+)?`)
var alnumTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

var claudePricingRequiredHeaders = []string{
	claudeHeaderModel,
	claudeHeaderBaseInput,
	claudeHeaderCacheWrite5m,
	claudeHeaderCacheWrite1h,
	claudeHeaderCacheHits,
	claudeHeaderOutput,
}

var claudePricingHeaderAliases = map[string][]string{
	claudeHeaderModel:        {"model"},
	claudeHeaderBaseInput:    {"baseinput", "baseinputtokens"},
	claudeHeaderCacheWrite5m: {"5mcachewrites", "5mcachewrite"},
	claudeHeaderCacheWrite1h: {"1hcachewrites", "1hcachewrite"},
	claudeHeaderCacheHits:    {"cachehitsrefreshes", "cachehits"},
	claudeHeaderOutput:       {"output", "outputtokens"},
}

var claudeOfficialDisplayNameToModels = map[string][]string{
	"claude opus 4.6":   {"claude-opus-4-6"},
	"claude opus 4.5":   {"claude-opus-4-5", "claude-opus-4-5-20251101", "claude-opus-4-5-20250929", "anthropic/claude-opus-4-5"},
	"claude opus 4.1":   {"claude-opus-4-1", "claude-opus-4-1-20250805"},
	"claude opus 4":     {"claude-opus-4", "claude-opus-4-20250514"},
	"claude sonnet 4.6": {"claude-sonnet-4-6"},
	"claude sonnet 4.5": {"claude-sonnet-4-5", "claude-sonnet-4-5-20250929", "anthropic/claude-sonnet-4-5"},
	"claude sonnet 4":   {"claude-sonnet-4", "claude-sonnet-4-20250514"},
	"claude sonnet 3.7": {"claude-sonnet-3-7", "claude-3-7-sonnet-latest", "claude-3-7-sonnet-20250219"},
	"claude haiku 4.5":  {"claude-haiku-4-5", "claude-haiku-4-5-20251001"},
	"claude haiku 3.5":  {"claude-haiku-3-5", "claude-3-5-haiku-latest", "claude-3-5-haiku-20241022"},
	"claude opus 3":     {"claude-opus-3", "claude-3-opus-latest", "claude-3-opus-20240229"},
	"claude haiku 3":    {"claude-haiku-3", "claude-3-haiku-20240307"},
}

type ModelPricingSyncResult struct {
	Provider           string   `json:"provider"`
	SyncedAt           string   `json:"synced_at"`
	TotalModels        int      `json:"total_models"`
	CreatedModels      int      `json:"created_models"`
	UpdatedModels      int      `json:"updated_models"`
	ChangedModels      int      `json:"changed_models"`
	UnchangedModels    int      `json:"unchanged_models"`
	UnrecognizedModels []string `json:"unrecognized_models,omitempty"`
}

type ClaudeOfficialPricingPreviewResult struct {
	Provider           string                            `json:"provider"`
	FetchedAt          string                            `json:"fetched_at"`
	Rows               []ClaudeOfficialPricingPreviewRow `json:"rows"`
	UnrecognizedModels []string                          `json:"unrecognized_models,omitempty"`
}

type ClaudeOfficialPricingPreviewRow struct {
	DisplayName                 string   `json:"display_name"`
	TargetModels                []string `json:"target_models,omitempty"`
	InputCostPerToken           float64  `json:"input_cost_per_token"`
	OutputCostPerToken          float64  `json:"output_cost_per_token"`
	CacheCreationInputTokenCost float64  `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCost     float64  `json:"cache_read_input_token_cost"`
	Ephemeral1hCostPerToken     float64  `json:"ephemeral_1h_cost_per_token"`
	IsRecognized                bool     `json:"is_recognized"`
}

type claudeOfficialModelPricing struct {
	DisplayName           string
	InputPerToken         float64
	CacheCreate5mPerToken float64
	CacheCreate1hPerToken float64
	CacheReadPerToken     float64
	OutputPerToken        float64
}

type claudePricingPreviewCache struct {
	rows      []claudeOfficialModelPricing
	fetchedAt time.Time
}

type claudeSyncPricingEntry struct {
	Pricing claudeOfficialModelPricing
	Source  string
}

func (mps *ModelPricingService) PreviewClaudeOfficialPricing() (ClaudeOfficialPricingPreviewResult, error) {
	result := ClaudeOfficialPricingPreviewResult{
		Provider:  "claude",
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if mps == nil {
		return result, fmt.Errorf("nil pricing service")
	}

	rows, err := loadClaudeOfficialPricingRows()
	if err != nil {
		return result, err
	}

	mps.storeClaudePricingPreviewCache(rows, time.Now().UTC())

	previewRows, unrecognized := buildClaudePricingPreviewRows(rows)
	result.Rows = previewRows
	result.UnrecognizedModels = unrecognized
	return result, nil
}

func (mps *ModelPricingService) SyncClaudeOfficialPricing() (ModelPricingSyncResult, error) {
	result := ModelPricingSyncResult{
		Provider: "claude",
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if mps == nil {
		return result, fmt.Errorf("nil pricing service")
	}

	rows, err := mps.loadClaudeOfficialPricingRowsForSync()
	if err != nil {
		return result, err
	}

	syncMap, unrecognized := buildClaudeSyncPricingMap(rows)
	result.UnrecognizedModels = unrecognized
	if len(syncMap) == 0 {
		if len(unrecognized) > 0 {
			return result, fmt.Errorf("未识别到可同步模型: %s", strings.Join(unrecognized, ", "))
		}
		return result, fmt.Errorf("未生成可同步的 Claude 模型价格")
	}

	mps.mu.Lock()
	defer mps.mu.Unlock()

	newOverrides := cloneModelPricingOverrides(mps.overrides)
	if newOverrides.Meta == nil {
		newOverrides.Meta = make(map[string]modelPricingMeta)
	}

	models := make([]string, 0, len(syncMap))
	for model := range syncMap {
		models = append(models, model)
	}
	sort.Strings(models)

	for _, model := range models {
		syncEntry := syncMap[model]
		pricing := syncEntry.Pricing
		source := normalizeModelPricingSource(syncEntry.Source)
		if source == "" {
			source = modelPricingSourceClaudeSync
		}
		result.TotalModels++

		oldEntry, hadPriceOverride := newOverrides.Pricing[model]
		oldEphemeral, hadEphemeralOverride := newOverrides.Ephemeral1h[model]
		oldMeta, hadMeta := newOverrides.Meta[model]

		entry := oldEntry
		if !hadPriceOverride && mps.effective != nil {
			if existing, ok := mps.effective.PricingEntryExact(model); ok {
				entry = existing
			}
		}

		entry.InputCostPerToken = pricing.InputPerToken
		entry.HasInputCostPerToken = true
		entry.OutputCostPerToken = pricing.OutputPerToken
		entry.HasOutputCostPerToken = true
		entry.CacheCreationInputTokenCost = pricing.CacheCreate5mPerToken
		entry.HasCacheCreationInputTokenCost = true
		entry.CacheReadInputTokenCost = pricing.CacheReadPerToken
		entry.HasCacheReadInputTokenCost = true
		entry.CacheCreationInputTokenCostAbove1Hr = pricing.CacheCreate1hPerToken

		newOverrides.Pricing[model] = entry
		newOverrides.Ephemeral1h[model] = pricing.CacheCreate1hPerToken
		newOverrides.Meta[model] = modelPricingMeta{
			Source:    source,
			UpdatedAt: result.SyncedAt,
		}

		changed := !hadPriceOverride ||
			!pricingEntriesAlmostEqual(oldEntry, entry) ||
			!hadEphemeralOverride ||
			!floatAlmostEqual(oldEphemeral, pricing.CacheCreate1hPerToken) ||
			!hadMeta ||
			normalizeModelPricingSource(oldMeta.Source) != source
		if !changed {
			result.UnchangedModels++
			continue
		}

		result.ChangedModels++
		if hadPriceOverride || hadEphemeralOverride || hadMeta {
			result.UpdatedModels++
		} else {
			result.CreatedModels++
		}
	}

	if result.ChangedModels == 0 {
		return result, nil
	}

	if err := saveModelPricingOverridesToDB(newOverrides); err != nil {
		return result, err
	}

	mps.overrides = newOverrides
	mps.rebuildLocked()
	return result, nil
}

func loadClaudeOfficialPricingRows() ([]claudeOfficialModelPricing, error) {
	pageHTML, err := fetchClaudePricingPageHTML()
	if err != nil {
		return nil, err
	}

	rows, err := parseClaudeModelPricingTable(pageHTML)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("未解析到 Claude 模型价格表")
	}
	return rows, nil
}

func (mps *ModelPricingService) loadClaudeOfficialPricingRowsForSync() ([]claudeOfficialModelPricing, error) {
	if cachedRows, ok := mps.consumeClaudePricingPreviewCache(time.Now().UTC()); ok {
		return cachedRows, nil
	}
	return loadClaudeOfficialPricingRows()
}

func (mps *ModelPricingService) storeClaudePricingPreviewCache(rows []claudeOfficialModelPricing, fetchedAt time.Time) {
	if mps == nil {
		return
	}
	mps.mu.Lock()
	mps.claudePricingPreview = claudePricingPreviewCache{
		rows:      cloneClaudeOfficialModelPricings(rows),
		fetchedAt: fetchedAt,
	}
	mps.mu.Unlock()
}

func (mps *ModelPricingService) consumeClaudePricingPreviewCache(now time.Time) ([]claudeOfficialModelPricing, bool) {
	if mps == nil {
		return nil, false
	}

	mps.mu.Lock()
	defer mps.mu.Unlock()

	cache := mps.claudePricingPreview
	if len(cache.rows) == 0 {
		return nil, false
	}
	if !cache.fetchedAt.IsZero() && now.Sub(cache.fetchedAt) > claudePreviewCacheTTL {
		mps.claudePricingPreview = claudePricingPreviewCache{}
		return nil, false
	}

	mps.claudePricingPreview = claudePricingPreviewCache{}
	return cloneClaudeOfficialModelPricings(cache.rows), true
}

func fetchClaudePricingPageHTML() (string, error) {
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, claudePricingPageURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "code-switch-r/claude-pricing-sync")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 Claude 官方价格页失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, claudePricingPageMaxBytes))
	if err != nil {
		return "", fmt.Errorf("读取 Claude 官方价格页失败: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 256 {
			snippet = snippet[:256]
		}
		return "", fmt.Errorf("Claude 官方价格页返回异常状态 %d: %s", resp.StatusCode, snippet)
	}

	return string(body), nil
}

func parseClaudeModelPricingTable(pageHTML string) ([]claudeOfficialModelPricing, error) {
	root, err := xhtml.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil, fmt.Errorf("解析 Claude 官方价格页 HTML 失败: %w", err)
	}

	table, headerIndex := findClaudeModelPricingTable(root)
	if table == nil {
		return nil, fmt.Errorf("未找到 Claude Model pricing 表格")
	}

	rows := extractTableRows(table)
	result := make([]claudeOfficialModelPricing, 0, len(rows))
	for rowIndex, row := range rows {
		cells := extractCells(row, "td")
		if len(cells) == 0 {
			continue
		}

		modelNameRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderModel)
		if err != nil {
			return nil, fmt.Errorf("读取第 %d 行模型名失败: %w", rowIndex+1, err)
		}
		modelName := normalizeModelDisplayName(modelNameRaw)
		if modelName == "" {
			continue
		}

		baseInputRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderBaseInput)
		if err != nil {
			return nil, fmt.Errorf("读取模型 %s 的 Base input 失败: %w", modelName, err)
		}
		baseInput, err := parseUSDPerMTok(baseInputRaw)
		if err != nil {
			return nil, fmt.Errorf("解析模型 %s 的 Base input 失败: %w", modelName, err)
		}

		cacheWrite5mRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderCacheWrite5m)
		if err != nil {
			return nil, fmt.Errorf("读取模型 %s 的 5m cache writes 失败: %w", modelName, err)
		}
		cacheWrite5m, err := parseUSDPerMTok(cacheWrite5mRaw)
		if err != nil {
			return nil, fmt.Errorf("解析模型 %s 的 5m cache writes 失败: %w", modelName, err)
		}

		cacheWrite1hRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderCacheWrite1h)
		if err != nil {
			return nil, fmt.Errorf("读取模型 %s 的 1h cache writes 失败: %w", modelName, err)
		}
		cacheWrite1h, err := parseUSDPerMTok(cacheWrite1hRaw)
		if err != nil {
			return nil, fmt.Errorf("解析模型 %s 的 1h cache writes 失败: %w", modelName, err)
		}

		cacheReadRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderCacheHits)
		if err != nil {
			return nil, fmt.Errorf("读取模型 %s 的 cache hits 失败: %w", modelName, err)
		}
		cacheRead, err := parseUSDPerMTok(cacheReadRaw)
		if err != nil {
			return nil, fmt.Errorf("解析模型 %s 的 cache hits 失败: %w", modelName, err)
		}

		outputRaw, err := readCellByHeader(cells, headerIndex, claudeHeaderOutput)
		if err != nil {
			return nil, fmt.Errorf("读取模型 %s 的 output 失败: %w", modelName, err)
		}
		output, err := parseUSDPerMTok(outputRaw)
		if err != nil {
			return nil, fmt.Errorf("解析模型 %s 的 output 失败: %w", modelName, err)
		}

		result = append(result, claudeOfficialModelPricing{
			DisplayName:           modelName,
			InputPerToken:         baseInput / 1_000_000,
			CacheCreate5mPerToken: cacheWrite5m / 1_000_000,
			CacheCreate1hPerToken: cacheWrite1h / 1_000_000,
			CacheReadPerToken:     cacheRead / 1_000_000,
			OutputPerToken:        output / 1_000_000,
		})
	}

	return result, nil
}

func findClaudeModelPricingTable(root *xhtml.Node) (*xhtml.Node, map[string]int) {
	tables := make([]*xhtml.Node, 0, 8)
	collectElementsByTag(root, "table", &tables)
	for _, table := range tables {
		headerIndex, err := resolveClaudePricingHeaderIndex(table)
		if err != nil {
			continue
		}
		return table, headerIndex
	}
	return nil, nil
}

func collectElementsByTag(node *xhtml.Node, tag string, out *[]*xhtml.Node) {
	if node == nil {
		return
	}
	if node.Type == xhtml.ElementNode && strings.EqualFold(node.Data, tag) {
		*out = append(*out, node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		collectElementsByTag(child, tag, out)
	}
}

func resolveClaudePricingHeaderIndex(table *xhtml.Node) (map[string]int, error) {
	headerCells := extractHeaderCells(table)
	if len(headerCells) == 0 {
		return nil, fmt.Errorf("thead/th 为空")
	}

	normalizedHeaderIndex := make(map[string]int, len(headerCells))
	for cellIndex, cell := range headerCells {
		key := normalizeHeader(nodeText(cell))
		if key == "" {
			continue
		}
		if _, exists := normalizedHeaderIndex[key]; !exists {
			normalizedHeaderIndex[key] = cellIndex
		}
	}

	index := make(map[string]int, len(claudePricingRequiredHeaders))
	for _, required := range claudePricingRequiredHeaders {
		resolvedIndex, ok := resolveHeaderIndexByAliases(normalizedHeaderIndex, claudePricingHeaderAliases[required])
		if !ok {
			return nil, fmt.Errorf("缺少表头列: %s", required)
		}
		index[required] = resolvedIndex
	}
	return index, nil
}

func resolveHeaderIndexByAliases(headerIndex map[string]int, aliases []string) (int, bool) {
	for _, alias := range aliases {
		if idx, ok := headerIndex[alias]; ok {
			return idx, true
		}
	}
	return 0, false
}

func extractHeaderCells(table *xhtml.Node) []*xhtml.Node {
	theadNodes := make([]*xhtml.Node, 0, 1)
	collectElementsByTag(table, "thead", &theadNodes)
	if len(theadNodes) == 0 {
		return nil
	}
	return extractCells(theadNodes[0], "th")
}

func normalizeHeader(raw string) string {
	tokens := alnumTokenPattern.FindAllString(strings.ToLower(stdhtml.UnescapeString(raw)), -1)
	return strings.Join(tokens, "")
}

func extractTableRows(table *xhtml.Node) []*xhtml.Node {
	tbodyNodes := make([]*xhtml.Node, 0, 1)
	collectElementsByTag(table, "tbody", &tbodyNodes)
	if len(tbodyNodes) == 0 {
		return nil
	}
	rows := make([]*xhtml.Node, 0, 16)
	collectElementsByTag(tbodyNodes[0], "tr", &rows)
	return rows
}

func extractCells(row *xhtml.Node, tag string) []*xhtml.Node {
	cells := make([]*xhtml.Node, 0, 8)
	collectElementsByTag(row, tag, &cells)
	return cells
}

func readCellByHeader(cells []*xhtml.Node, headerIndex map[string]int, header string) (string, error) {
	idx, ok := headerIndex[header]
	if !ok {
		return "", fmt.Errorf("未找到列 %s", header)
	}
	if idx < 0 || idx >= len(cells) {
		return "", fmt.Errorf("列 %s 超出范围: index=%d, cells=%d", header, idx, len(cells))
	}
	return nodeText(cells[idx]), nil
}

func nodeText(node *xhtml.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*xhtml.Node)
	walk = func(current *xhtml.Node) {
		if current == nil {
			return
		}
		if current.Type == xhtml.TextNode {
			builder.WriteString(current.Data)
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	text := stdhtml.UnescapeString(builder.String())
	return strings.Join(strings.Fields(text), " ")
}

func normalizeModelDisplayName(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, "(deprecated)", "")
	value = strings.ReplaceAll(value, "(Deprecated)", "")
	value = strings.Join(strings.Fields(value), " ")
	return strings.TrimSpace(value)
}

func parseUSDPerMTok(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("空值")
	}
	number := numericValuePattern.FindString(value)
	if number == "" {
		return 0, fmt.Errorf("未找到数字: %s", value)
	}
	parsed, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}

func buildClaudeSyncPricingMap(rows []claudeOfficialModelPricing) (map[string]claudeSyncPricingEntry, []string) {
	result := make(map[string]claudeSyncPricingEntry)
	unrecognizedSet := make(map[string]struct{})
	for _, row := range rows {
		targetModels, recognized := resolveClaudePricingTargetModels(row.DisplayName)
		source := modelPricingSourceClaudeSync
		if !recognized {
			fallbackModel := normalizeClaudeFallbackModelKey(row.DisplayName)
			if fallbackModel == "" {
				if strings.TrimSpace(row.DisplayName) != "" {
					unrecognizedSet[row.DisplayName] = struct{}{}
				}
				continue
			}
			targetModels = []string{fallbackModel}
			source = modelPricingSourceManual
		}
		for _, model := range targetModels {
			result[model] = claudeSyncPricingEntry{
				Pricing: row,
				Source:  source,
			}
		}
	}

	unrecognized := make([]string, 0, len(unrecognizedSet))
	for model := range unrecognizedSet {
		unrecognized = append(unrecognized, model)
	}
	sort.Strings(unrecognized)
	return result, unrecognized
}

func buildClaudePricingPreviewRows(rows []claudeOfficialModelPricing) ([]ClaudeOfficialPricingPreviewRow, []string) {
	previewRows := make([]ClaudeOfficialPricingPreviewRow, 0, len(rows))
	unrecognizedSet := make(map[string]struct{})

	for _, row := range rows {
		targetModels, recognized := resolveClaudePricingTargetModels(row.DisplayName)
		if !recognized {
			fallbackModel := normalizeClaudeFallbackModelKey(row.DisplayName)
			if fallbackModel == "" {
				if strings.TrimSpace(row.DisplayName) != "" {
					unrecognizedSet[row.DisplayName] = struct{}{}
				}
			} else {
				targetModels = []string{fallbackModel}
			}
		}

		targets := append([]string(nil), targetModels...)
		sort.Strings(targets)

		previewRows = append(previewRows, ClaudeOfficialPricingPreviewRow{
			DisplayName:                 row.DisplayName,
			TargetModels:                targets,
			InputCostPerToken:           row.InputPerToken,
			OutputCostPerToken:          row.OutputPerToken,
			CacheCreationInputTokenCost: row.CacheCreate5mPerToken,
			CacheReadInputTokenCost:     row.CacheReadPerToken,
			Ephemeral1hCostPerToken:     row.CacheCreate1hPerToken,
			IsRecognized:                recognized,
		})
	}

	sort.Slice(previewRows, func(i, j int) bool {
		return strings.ToLower(previewRows[i].DisplayName) < strings.ToLower(previewRows[j].DisplayName)
	})

	unrecognized := make([]string, 0, len(unrecognizedSet))
	for model := range unrecognizedSet {
		unrecognized = append(unrecognized, model)
	}
	sort.Strings(unrecognized)
	return previewRows, unrecognized
}

func normalizeClaudeFallbackModelKey(displayName string) string {
	normalizedDisplayName := normalizeModelDisplayName(displayName)
	if normalizedDisplayName == "" {
		return ""
	}

	tokens := alnumTokenPattern.FindAllString(strings.ToLower(normalizedDisplayName), -1)
	if len(tokens) == 0 {
		return ""
	}
	return strings.Join(tokens, "-")
}

func resolveClaudePricingTargetModels(displayName string) ([]string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(displayName))
	targetModels, ok := claudeOfficialDisplayNameToModels[normalized]
	if !ok {
		return nil, false
	}
	return uniqueNonEmptyModels(targetModels...), true
}

func uniqueNonEmptyModels(models ...string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		value := strings.TrimSpace(model)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneClaudeOfficialModelPricings(rows []claudeOfficialModelPricing) []claudeOfficialModelPricing {
	if len(rows) == 0 {
		return nil
	}
	cloned := make([]claudeOfficialModelPricing, len(rows))
	copy(cloned, rows)
	return cloned
}

func pricingEntriesAlmostEqual(a modelpricing.PricingEntry, b modelpricing.PricingEntry) bool {
	return floatAlmostEqual(a.InputCostPerToken, b.InputCostPerToken) &&
		floatAlmostEqual(a.OutputCostPerToken, b.OutputCostPerToken) &&
		floatAlmostEqual(a.CacheCreationInputTokenCost, b.CacheCreationInputTokenCost) &&
		floatAlmostEqual(a.CacheReadInputTokenCost, b.CacheReadInputTokenCost) &&
		floatAlmostEqual(a.CacheCreationInputTokenCostAbove1Hr, b.CacheCreationInputTokenCostAbove1Hr)
}

func floatAlmostEqual(a float64, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return false
	}
	return math.Abs(a-b) <= 1e-12
}
