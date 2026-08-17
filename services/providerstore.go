/**
 * @name: 供应商统一存储
 * @Descripttion: SQLite 单表存储所有应用的供应商数据（公共列 + payload JSON 扩展），含 JSON 文件自动迁移
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 02:40:00
 * @LastEditTime: 2026-08-17 02:40:00
 * @FilePath: services/providerstore.go
 */

package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// providers 统一存储表：一行 = 一个供应商
// 公共列用于按平台过滤与调度（enabled/level/sort），payload 存完整供应商 JSON（全部字段无损）
const createProvidersStoreSQL = `CREATE TABLE IF NOT EXISTS providers_store (
	platform TEXT NOT NULL,
	id TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	level INTEGER NOT NULL DEFAULT 1,
	sort_index INTEGER NOT NULL DEFAULT 0,
	category TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL DEFAULT '{}',
	updated_at INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (platform, id)
)`

// ensureProvidersStoreTable 确保统一存储表与索引存在（由 InitDatabase 调用）
func ensureProvidersStoreTable() error {
	db, err := xdb.DB("default")
	if err != nil {
		return fmt.Errorf("获取数据库连接失败: %w", err)
	}
	if _, err := db.Exec(createProvidersStoreSQL); err != nil {
		return fmt.Errorf("创建 providers_store 表失败: %w", err)
	}
	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_providers_store_platform ON providers_store(platform)"); err != nil {
		return fmt.Errorf("创建 providers_store 平台索引失败: %w", err)
	}
	return nil
}

// providerStoreRow 统一存储行（公共列快照 + 完整 payload）
type providerStoreRow struct {
	Platform  string
	ID        string
	Name      string
	Enabled   bool
	Level     int
	SortIndex int
	Category  string
	Payload   string
}

// loadProviderStoreRows 按平台读取全部行（按保存顺序 sort_index 排序，id 兜底稳定）
// 不能按 id 排序：字符串 id（gemini-2 / gemini-10）按词典序会打乱用户保存的顺序
func loadProviderStoreRows(platform string) ([]providerStoreRow, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return nil, err
	}
	rows, err := db.Query(`
		SELECT platform, id, name, enabled, level, sort_index, category, payload
		FROM providers_store
		WHERE platform = ?
		ORDER BY sort_index, id
	`, platform)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]providerStoreRow, 0)
	for rows.Next() {
		var row providerStoreRow
		var enabled int
		if err := rows.Scan(&row.Platform, &row.ID, &row.Name, &enabled, &row.Level, &row.SortIndex, &row.Category, &row.Payload); err != nil {
			return nil, err
		}
		row.Enabled = enabled != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

// replaceProviderStoreRows 全量替换指定平台的行（DELETE + 逐条 INSERT 打包为单事务，原子生效）
// 事务失败时自动用替换前的原行恢复，避免半写入中间态
func replaceProviderStoreRows(platform string, rows []providerStoreRow) error {
	originalRows, originalErr := loadProviderStoreRows(platform)
	if originalErr != nil {
		// 原数据读取失败时 fail-fast：拿不到快照就无法回滚，直接返回错误不动现有数据
		return fmt.Errorf("读取平台 %s 原供应商数据失败: %w", platform, originalErr)
	}

	if err := replaceProviderStoreRowsNoRollback(platform, rows); err != nil {
		if len(originalRows) > 0 {
			if restoreErr := replaceProviderStoreRowsNoRollback(platform, originalRows); restoreErr != nil {
				return fmt.Errorf("%w；恢复平台 %s 原供应商数据也失败: %v", err, platform, restoreErr)
			}
			log.Printf("[ProviderStore] 平台 %s 保存失败已恢复原数据: %v", platform, err)
		}
		return err
	}
	return nil
}

// replaceProviderStoreRowsNoRollback 将 DELETE + 全部 INSERT 打包成一组事务任务提交
// 单事务内原子生效，进程崩溃/任一语句失败都不会出现「已清空但未写回」的截断中间态
func replaceProviderStoreRowsNoRollback(platform string, rows []providerStoreRow) error {
	if GlobalDBQueue == nil {
		return fmt.Errorf("数据库写入队列未初始化")
	}
	tasks := []WriteTask{{
		SQL:  "DELETE FROM providers_store WHERE platform = ?",
		Args: []interface{}{platform},
	}}
	if len(rows) == 0 {
		// 保留「已初始化但为空」状态（哨兵行），Load 时返回空切片而非 nil
		tasks = append(tasks, WriteTask{
			SQL: `
			INSERT OR REPLACE INTO providers_store (platform, id, name, enabled, level, sort_index, category, payload, updated_at)
			VALUES (?, ?, '', 0, 1, 0, '', '[]', ?)
		`,
			Args: []interface{}{platform, providerStoreEmptySentinelID, time.Now().Unix()},
		})
	} else {
		now := time.Now().Unix()
		for _, row := range rows {
			tasks = append(tasks, WriteTask{
				SQL: `
			INSERT OR REPLACE INTO providers_store (platform, id, name, enabled, level, sort_index, category, payload, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
				Args: []interface{}{
					platform, row.ID, row.Name, providerStoreBoolToInt(row.Enabled),
					row.Level, row.SortIndex, row.Category, row.Payload, now,
				},
			})
		}
	}
	if err := GlobalDBQueue.ExecTxGroup(tasks); err != nil {
		return fmt.Errorf("替换平台 %s 供应商失败: %w", platform, err)
	}
	return nil
}

func providerStoreBoolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

// ========== Provider（Claude / Codex / OpenCode）适配层 ==========
// 自定义 CLI（custom:{toolId}）保持 JSON 文件存储，不走统一存储（见 loadCustomCLIProvidersFallback）

// providerStorePayload Provider 的 payload 序列化（直接复用既有 JSON 结构，全字段无损）
type providerStorePayload struct {
	Provider
}

// LoadProvidersFromStore 从统一存储读取指定平台的 Provider 列表
// 自定义 CLI（custom:{toolId}）保持 JSON 文件存储，回退文件读写
func LoadProvidersFromStore(kind string) ([]Provider, error) {
	if IsCustomPlatform(kind) {
		return loadCustomCLIProvidersFallback(kind)
	}
	platform := NormalizePlatform(kind)
	rows, err := loadProviderStoreRows(platform)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		// 未初始化：返回 nil，保留前端默认预设初始化路径
		return nil, nil
	}
	providers := make([]Provider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var payload providerStorePayload
		if err := json.Unmarshal([]byte(row.Payload), &payload); err != nil {
			log.Printf("[ProviderStore] 平台 %s 供应商 %s payload 解析失败: %v", platform, row.ID, err)
			continue
		}
		if payload.ID == 0 {
			if parsed, parseErr := strconv.ParseInt(row.ID, 10, 64); parseErr == nil {
				payload.ID = parsed
			}
		}
		providers = append(providers, payload.Provider)
	}
	return providers, nil
}

// SaveProvidersToStore 将 Provider 列表全量写入统一存储
// 自定义 CLI（custom:{toolId}）保持 JSON 文件存储，回退文件读写
func SaveProvidersToStore(kind string, providers []Provider) error {
	if IsCustomPlatform(kind) {
		return saveCustomCLIProvidersFallback(kind, providers)
	}
	platform := NormalizePlatform(kind)
	rows := make([]providerStoreRow, 0, len(providers))
	seenIDs := make(map[string]bool, len(providers))
	for index, provider := range providers {
		id := strconv.FormatInt(provider.ID, 10)
		if seenIDs[id] {
			// 同平台内重复 ID 会被 (platform, id) 主键吞掉后一行，静默丢数据，必须显式报错
			return fmt.Errorf("平台 %s 内供应商 ID %s 重复（名称: %s）", platform, id, provider.Name)
		}
		seenIDs[id] = true
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化供应商 %s 失败: %w", provider.Name, err)
		}
		rows = append(rows, providerStoreRow{
			Platform:  platform,
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(platform, rows)
}

// loadCustomCLIProvidersFallback 自定义 CLI 平台继续使用 JSON 文件存储
func loadCustomCLIProvidersFallback(kind string) ([]Provider, error) {
	path, err := providerConfigPath(kind, false)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var envelope providerEnvelope
	if len(data) == 0 {
		return []Provider{}, nil
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, err
	}
	return envelope.Providers, nil
}

// saveCustomCLIProvidersFallback 自定义 CLI 平台继续使用 JSON 文件存储（原子写）
func saveCustomCLIProvidersFallback(kind string, providers []Provider) error {
	path, err := providerConfigPath(kind, true)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(providerEnvelope{Providers: providers}, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ========== Gemini 适配层 ==========

// LoadGeminiProvidersFromStore 从统一存储读取 Gemini 供应商列表
func LoadGeminiProvidersFromStore() ([]GeminiProvider, error) {
	rows, err := loadProviderStoreRows(string(PlatformGemini))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	providers := make([]GeminiProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var provider GeminiProvider
		if err := json.Unmarshal([]byte(row.Payload), &provider); err != nil {
			log.Printf("[ProviderStore] Gemini 供应商 %s payload 解析失败: %v", row.ID, err)
			continue
		}
		if provider.ID == "" {
			provider.ID = row.ID
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

// SaveGeminiProvidersToStore 将 Gemini 供应商列表全量写入统一存储
func SaveGeminiProvidersToStore(providers []GeminiProvider) error {
	rows := make([]providerStoreRow, 0, len(providers))
	for index, provider := range providers {
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化 Gemini 供应商 %s 失败: %w", provider.Name, err)
		}
		id := provider.ID
		if id == "" {
			id = fmt.Sprintf("gemini-%d", index+1)
		}
		rows = append(rows, providerStoreRow{
			Platform:  string(PlatformGemini),
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(string(PlatformGemini), rows)
}

// ========== OpenCode 适配层 ==========

// LoadOpenCodeProvidersFromStore 从统一存储读取 OpenCode 供应商列表
func LoadOpenCodeProvidersFromStore() ([]OpenCodeProvider, error) {
	rows, err := loadProviderStoreRows(string(PlatformOpenCode))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	providers := make([]OpenCodeProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var provider OpenCodeProvider
		if err := json.Unmarshal([]byte(row.Payload), &provider); err != nil {
			log.Printf("[ProviderStore] OpenCode 供应商 %s payload 解析失败: %v", row.ID, err)
			continue
		}
		if provider.ID == "" {
			provider.ID = row.ID
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

// SaveOpenCodeProvidersToStore 将 OpenCode 供应商列表全量写入统一存储
func SaveOpenCodeProvidersToStore(providers []OpenCodeProvider) error {
	rows := make([]providerStoreRow, 0, len(providers))
	for index, provider := range providers {
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化 OpenCode 供应商 %s 失败: %w", provider.Name, err)
		}
		id := provider.ID
		if id == "" {
			id = fmt.Sprintf("opencode-%d", index+1)
		}
		rows = append(rows, providerStoreRow{
			Platform:  string(PlatformOpenCode),
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(string(PlatformOpenCode), rows)
}

// ========== OpenClaw 适配层 ==========

// LoadOpenClawProvidersFromStore 从统一存储读取 OpenClaw 供应商列表
func LoadOpenClawProvidersFromStore() ([]OpenClawProvider, error) {
	rows, err := loadProviderStoreRows(string(PlatformOpenClaw))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	providers := make([]OpenClawProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var provider OpenClawProvider
		if err := json.Unmarshal([]byte(row.Payload), &provider); err != nil {
			log.Printf("[ProviderStore] OpenClaw 供应商 %s payload 解析失败: %v", row.ID, err)
			continue
		}
		if provider.ID == "" {
			provider.ID = row.ID
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

// SaveOpenClawProvidersToStore 将 OpenClaw 供应商列表全量写入统一存储（id 使用字符串 ID）
func SaveOpenClawProvidersToStore(providers []OpenClawProvider) error {
	rows := make([]providerStoreRow, 0, len(providers))
	for index, provider := range providers {
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化 OpenClaw 供应商 %s 失败: %w", provider.Name, err)
		}
		id := provider.ID
		if id == "" {
			id = fmt.Sprintf("openclaw-%d", index+1)
		}
		rows = append(rows, providerStoreRow{
			Platform:  string(PlatformOpenClaw),
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(string(PlatformOpenClaw), rows)
}

// ========== Hermes 适配层 ==========

// LoadHermesProvidersFromStore 从统一存储读取 Hermes 供应商列表
func LoadHermesProvidersFromStore() ([]HermesProvider, error) {
	rows, err := loadProviderStoreRows(string(PlatformHermes))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	providers := make([]HermesProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var provider HermesProvider
		if err := json.Unmarshal([]byte(row.Payload), &provider); err != nil {
			log.Printf("[ProviderStore] Hermes 供应商 %s payload 解析失败: %v", row.ID, err)
			continue
		}
		if provider.ID == "" {
			provider.ID = row.ID
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

// SaveHermesProvidersToStore 将 Hermes 供应商列表全量写入统一存储（id 使用字符串 ID）
func SaveHermesProvidersToStore(providers []HermesProvider) error {
	rows := make([]providerStoreRow, 0, len(providers))
	for index, provider := range providers {
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化 Hermes 供应商 %s 失败: %w", provider.Name, err)
		}
		id := provider.ID
		if id == "" {
			id = fmt.Sprintf("hermes-%d", index+1)
		}
		rows = append(rows, providerStoreRow{
			Platform:  string(PlatformHermes),
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(string(PlatformHermes), rows)
}

// ========== Pi 适配层 ==========

// LoadPiProvidersFromStore 从统一存储读取 Pi 供应商列表
func LoadPiProvidersFromStore() ([]PiProvider, error) {
	rows, err := loadProviderStoreRows(string(PlatformPi))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	providers := make([]PiProvider, 0, len(rows))
	for _, row := range rows {
		if row.ID == providerStoreEmptySentinelID {
			continue
		}
		var provider PiProvider
		if err := json.Unmarshal([]byte(row.Payload), &provider); err != nil {
			log.Printf("[ProviderStore] Pi 供应商 %s payload 解析失败: %v", row.ID, err)
			continue
		}
		if provider.ID == "" {
			provider.ID = row.ID
		}
		providers = append(providers, provider)
	}
	return providers, nil
}

// SavePiProvidersToStore 将 Pi 供应商列表全量写入统一存储（id 使用字符串 ID）
func SavePiProvidersToStore(providers []PiProvider) error {
	rows := make([]providerStoreRow, 0, len(providers))
	for index, provider := range providers {
		payload, err := json.Marshal(provider)
		if err != nil {
			return fmt.Errorf("序列化 Pi 供应商 %s 失败: %w", provider.Name, err)
		}
		id := provider.ID
		if id == "" {
			id = fmt.Sprintf("pi-%d", index+1)
		}
		rows = append(rows, providerStoreRow{
			Platform:  string(PlatformPi),
			ID:        id,
			Name:      provider.Name,
			Enabled:   provider.Enabled,
			Level:     provider.Level,
			SortIndex: index,
			Category:  provider.Category,
			Payload:   string(payload),
		})
	}
	return replaceProviderStoreRows(string(PlatformPi), rows)
}

// ========== JSON 文件 → SQLite 自动迁移 ==========
// 在 InitDatabase 内、写入队列启动之前以单事务直连执行（对齐 migrateBlacklistIdentityKeyWithDB 先例）
// 全部源迁移成功才 commit + 重命名 .bak；任一失败整体回滚、不动原文件，下次启动重试

// migrateLegacyProviderEntry 待迁移的旧 JSON 源（路径 + 行构造回调）
type migrateLegacyProviderEntry struct {
	platform string
	jsonPath string
	buildRow func(tx *sql.Tx) (int, error) // 在事务内插入行，返回迁移行数
}

// MigrateProviderJSONFilesToStore 事务式迁移（由 InitDatabase 调用，队列启动前）
func MigrateProviderJSONFilesToStore(db *sql.DB) error {
	// 第一遍：收集全部待迁移源（单平台 JSON 损坏时跳过该平台并记录，不影响其他平台）
	entries, err := collectLegacyProviderEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	// 单事务：逐平台检查存储为空则插入
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var migratedEntries []migrateLegacyProviderEntry
	var skipRenamedEntries []migrateLegacyProviderEntry // 存储已初始化的平台：旧文件直接补 .bak，避免每次启动重复解析
	var renamedCount int
	for _, entry := range entries {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM providers_store WHERE platform = ?", entry.platform).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			// 该平台已迁移过或已在新存储上使用，跳过插入
			skipRenamedEntries = append(skipRenamedEntries, entry)
			continue
		}
		inserted, err := entry.buildRow(tx)
		if err != nil {
			return fmt.Errorf("平台 %s 迁移插入失败: %w", entry.platform, err)
		}
		if inserted > 0 {
			migratedEntries = append(migratedEntries, entry)
			renamedCount += inserted
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true

	// commit 成功后再重命名 .bak（rename 失败仅告警不回滚，数据已入库）
	for _, entry := range migratedEntries {
		if err := os.Rename(entry.jsonPath, entry.jsonPath+".bak"); err != nil {
			log.Printf("[ProviderStore] 备份旧配置 %s 失败（数据已迁移入库，仅备份失败）: %v", entry.jsonPath, err)
		} else {
			log.Printf("[ProviderStore] 平台 %s 已迁移到 SQLite，原文件备份为 %s", entry.platform, filepath.Base(entry.jsonPath)+".bak")
		}
	}
	// 存储已初始化的平台：旧 JSON 已过时，同样备份为 .bak（消除每次启动的重复解析）
	for _, entry := range skipRenamedEntries {
		if err := os.Rename(entry.jsonPath, entry.jsonPath+".bak"); err != nil {
			log.Printf("[ProviderStore] 备份过时旧配置 %s 失败（存储已初始化，不影响数据）: %v", entry.jsonPath, err)
		} else {
			log.Printf("[ProviderStore] 平台 %s 存储已初始化，旧文件直接备份为 %s", entry.platform, filepath.Base(entry.jsonPath)+".bak")
		}
	}
	if renamedCount > 0 {
		log.Printf("[ProviderStore] JSON→SQLite 迁移完成，共迁移 %d 个供应商", renamedCount)
	}
	return nil
}

// collectLegacyProviderEntries 收集存在且未备份过的旧 JSON 源
// Claude/Codex：envelope{providers:[]Provider}；OpenCode：envelope{providers:[]OpenCodeProvider}；Gemini：裸数组 []GeminiProvider
// 单平台解析失败：跳过该平台并记录日志（原文件保留），其余平台正常迁移
func collectLegacyProviderEntries() ([]migrateLegacyProviderEntry, error) {
	entries := make([]migrateLegacyProviderEntry, 0)

	// Claude / Codex（Provider envelope 格式）
	for _, platformConst := range []CLIPlatform{PlatformClaude, PlatformCodex} {
		kind := string(platformConst)
		path, err := providerConfigPath(kind, false)
		if err != nil {
			return nil, err
		}
		if !providerConfigFileExists(path) {
			// 主路径不存在：回退顶层旧位置（~/.code-switch/codex.json）
			legacyPath, legacyErr := legacyProviderFilePathNoCreate(kind)
			if legacyErr != nil || legacyPath == "" || !providerConfigFileExists(legacyPath) {
				continue
			}
			path = legacyPath
		}
		if providerConfigFileExists(path + ".bak") {
			continue
		}
		platform := NormalizePlatform(kind)
		providers, _, err := readLegacyProviderJSONAtPath(path)
		if err != nil {
			// 单平台 JSON 损坏：跳过该平台并记录，不阻塞其他平台迁移（原文件保留，用户可手工修复）
			log.Printf("[ProviderStore] 解析旧配置 %s 失败，跳过平台 %s 的迁移: %v", path, platform, err)
			continue
		}
		if len(providers) == 0 {
			continue
		}
		entries = append(entries, migrateLegacyProviderEntry{
			platform: platform,
			jsonPath: path,
			buildRow: func(tx *sql.Tx) (int, error) {
				return insertProviderRowsTx(tx, platform, providers)
			},
		})
	}

	// OpenCode（OpenCodeProvider envelope 格式）
	opencodePath, err := providerConfigPath(string(PlatformOpenCode), false)
	if err != nil {
		return nil, err
	}
	if providerConfigFileExists(opencodePath) && !providerConfigFileExists(opencodePath+".bak") {
		data, err := os.ReadFile(opencodePath)
		if err != nil {
			return nil, err
		}
		var envelope opencodeProviderEnvelope
		parseFailed := false
		if len(data) > 0 {
			if err := json.Unmarshal(data, &envelope); err != nil {
				// 单平台 JSON 损坏：跳过该平台并记录，不阻塞其他平台迁移
				log.Printf("[ProviderStore] 解析旧配置 %s 失败，跳过 OpenCode 平台的迁移: %v", opencodePath, err)
				parseFailed = true
			}
		}
		if !parseFailed && len(envelope.Providers) > 0 {
			providers := envelope.Providers
			entries = append(entries, migrateLegacyProviderEntry{
				platform: string(PlatformOpenCode),
				jsonPath: opencodePath,
				buildRow: func(tx *sql.Tx) (int, error) {
					return insertRowsJSONTx(tx, string(PlatformOpenCode), len(providers), func(index int) (string, string, bool, int, string, any, error) {
						provider := providers[index]
						return provider.ID, provider.Name, provider.Enabled, provider.Level, provider.Category, provider, nil
					})
				},
			})
		}
	}

	// Gemini（裸数组格式 gemini-providers.json）
	geminiPath := getGeminiProvidersPath()
	if providerConfigFileExists(geminiPath) && !providerConfigFileExists(geminiPath+".bak") {
		data, err := os.ReadFile(geminiPath)
		if err != nil {
			return nil, err
		}
		var providers []GeminiProvider
		parseFailed := false
		if len(data) > 0 {
			if err := json.Unmarshal(data, &providers); err != nil {
				// 单平台 JSON 损坏：跳过该平台并记录，不阻塞其他平台迁移
				log.Printf("[ProviderStore] 解析旧配置 %s 失败，跳过 Gemini 平台的迁移: %v", geminiPath, err)
				parseFailed = true
			}
		}
		if !parseFailed && len(providers) > 0 {
			entries = append(entries, migrateLegacyProviderEntry{
				platform: string(PlatformGemini),
				jsonPath: geminiPath,
				buildRow: func(tx *sql.Tx) (int, error) {
					return insertRowsJSONTx(tx, string(PlatformGemini), len(providers), func(index int) (string, string, bool, int, string, any, error) {
						provider := providers[index]
						id := provider.ID
						if id == "" {
							id = fmt.Sprintf("gemini-legacy-%d", index+1)
						}
						provider.ID = id
						return id, provider.Name, provider.Enabled, provider.Level, provider.Category, provider, nil
					})
				},
			})
		}
	}

	return entries, nil
}

// insertProviderRowsTx 事务内插入 Provider 行（INSERT OR IGNORE 幂等）
func insertProviderRowsTx(tx *sql.Tx, platform string, providers []Provider) (int, error) {
	return insertRowsJSONTx(tx, platform, len(providers), func(index int) (string, string, bool, int, string, any, error) {
		provider := providers[index]
		return strconv.FormatInt(provider.ID, 10), provider.Name, provider.Enabled, provider.Level, provider.Category, provider, nil
	})
}

// insertRowsJSONTx 事务内逐条插入任意类型供应商行
// rowOf 返回：id、name、enabled、level、category、payload 对象
func insertRowsJSONTx(tx *sql.Tx, platform string, count int, rowOf func(index int) (string, string, bool, int, string, any, error)) (int, error) {
	inserted := 0
	now := time.Now().Unix()
	for index := 0; index < count; index++ {
		id, name, enabled, level, category, payloadObj, err := rowOf(index)
		if err != nil {
			return inserted, err
		}
		payload, err := json.Marshal(payloadObj)
		if err != nil {
			return inserted, err
		}
		result, err := tx.Exec(`
			INSERT OR IGNORE INTO providers_store (platform, id, name, enabled, level, sort_index, category, payload, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, platform, id, name, providerStoreBoolToInt(enabled), level, index, category, string(payload), now)
		if err != nil {
			return inserted, err
		}
		if affected, affectErr := result.RowsAffected(); affectErr == nil && affected > 0 {
			inserted++
		}
	}
	return inserted, nil
}

// providerStoreEmptySentinelID 空列表哨兵行 id：区分「未初始化（无行，Load 返回 nil）」
// 与「已初始化但供应商为空（哨兵行，Load 返回空切片）」——前端默认预设初始化依赖该区别
const providerStoreEmptySentinelID = "__empty__"

// readLegacyProviderJSONAtPath 从指定路径读取旧 JSON envelope
func readLegacyProviderJSONAtPath(path string) ([]Provider, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var envelope providerEnvelope
	if len(data) == 0 {
		return []Provider{}, true, nil
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, true, err
	}
	return envelope.Providers, true, nil
}
