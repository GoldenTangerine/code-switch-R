/*
 * @name: Codex 历史迁移服务
 * @Descripttion: 迁移与还原 Codex 会话历史的 provider 分桶
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-07-02 00:00:00
 * @LastEditTime: 2026-07-02 00:00:00
 * @FilePath: services/codex_history_migration.go
 */
package services

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const (
	codexHistoryBackupName        = "codex-history-unify-v1"
	codexHistoryRestoreBackupName = "codex-history-unify-restore-v1"
	codexStateDBName              = "state_5.sqlite"
)

var codexHistoryMigrationMu sync.Mutex

type CodexHistoryMigrationResult struct {
	MigratedJSONLFiles int    `json:"migrated_jsonl_files"`
	MigratedStateRows  int    `json:"migrated_state_rows"`
	SkippedReason      string `json:"skipped_reason,omitempty"`
}

type CodexHistoryRestoreResult struct {
	RestoredJSONLFiles int    `json:"restored_jsonl_files"`
	RestoredStateRows  int    `json:"restored_state_rows"`
	SkippedReason      string `json:"skipped_reason,omitempty"`
}

type codexHistoryLedger struct {
	CodexDir           string   `json:"codex_dir"`
	OfficialSessionIDs []string `json:"official_session_ids"`
	OfficialThreadIDs  []string `json:"official_thread_ids"`
	CreatedAt          string   `json:"created_at"`
}

func MigrateCodexHistoryToUnifiedBucket(includeThirdParty bool) (CodexHistoryMigrationResult, error) {
	codexHistoryMigrationMu.Lock()
	defer codexHistoryMigrationMu.Unlock()

	codexDir, err := codexConfigDirPath()
	if err != nil {
		return CodexHistoryMigrationResult{}, err
	}
	configPath := filepath.Join(codexDir, codexConfigFileName)
	if !codexConfigRoutesUnified(configPath) {
		return CodexHistoryMigrationResult{SkippedReason: "live_not_unified"}, nil
	}

	// 当前只迁入官方会话，避免第三方旧会话丢失原 provider 分桶后无法精确恢复。
	_ = includeThirdParty
	sourceProviders := map[string]bool{codexOfficialProvider: true}

	backupRoot, err := createCodexHistoryBackupRoot(codexHistoryBackupName)
	if err != nil {
		return CodexHistoryMigrationResult{}, err
	}
	ledger := codexHistoryLedger{
		CodexDir:  canonicalPathString(codexDir),
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	jsonlFiles := collectCodexJSONLFiles(codexDir)
	migratedJSONL := 0
	for _, file := range jsonlFiles {
		changed, officialIDs, err := rewriteCodexJSONLProvider(file, backupRoot, sourceProviders, codexProviderKey, true, nil)
		if err != nil {
			return CodexHistoryMigrationResult{}, err
		}
		ledger.OfficialSessionIDs = appendUniqueStrings(ledger.OfficialSessionIDs, officialIDs...)
		if changed {
			migratedJSONL++
		}
	}

	stateDBs := collectCodexStateDBs(codexDir)
	migratedRows := 0
	for _, dbPath := range stateDBs {
		rows, officialIDs, err := migrateCodexStateDBProvider(dbPath, backupRoot, sourceProviders, codexProviderKey)
		if err != nil {
			return CodexHistoryMigrationResult{}, err
		}
		ledger.OfficialThreadIDs = appendUniqueStrings(ledger.OfficialThreadIDs, officialIDs...)
		migratedRows += rows
	}

	if migratedJSONL == 0 && migratedRows == 0 {
		_ = os.RemoveAll(backupRoot)
		return CodexHistoryMigrationResult{SkippedReason: "nothing_to_migrate"}, nil
	}
	if err := writeCodexHistoryLedger(backupRoot, ledger); err != nil {
		return CodexHistoryMigrationResult{}, err
	}
	return CodexHistoryMigrationResult{MigratedJSONLFiles: migratedJSONL, MigratedStateRows: migratedRows}, nil
}

func HasCodexUnifiedHistoryBackup() bool {
	parent, err := codexHistoryBackupParent(codexHistoryBackupName)
	if err != nil {
		return false
	}
	codexDir, err := codexConfigDirPath()
	if err != nil {
		return false
	}
	sessionIDs, threadIDs, err := collectCodexOfficialLedger(parent, canonicalPathString(codexDir))
	return err == nil && (len(sessionIDs) > 0 || len(threadIDs) > 0)
}

func RestoreCodexUnifiedHistoryFromBackups() (CodexHistoryRestoreResult, error) {
	codexHistoryMigrationMu.Lock()
	defer codexHistoryMigrationMu.Unlock()

	if loadCodexRuntimeSettings().UnifyCodexSessionHistory {
		return CodexHistoryRestoreResult{SkippedReason: "unify_toggle_on"}, nil
	}
	codexDir, err := codexConfigDirPath()
	if err != nil {
		return CodexHistoryRestoreResult{}, err
	}
	parent, err := codexHistoryBackupParent(codexHistoryBackupName)
	if err != nil {
		return CodexHistoryRestoreResult{}, err
	}
	sessionIDs, threadIDs, err := collectCodexOfficialLedger(parent, canonicalPathString(codexDir))
	if err != nil {
		return CodexHistoryRestoreResult{}, err
	}
	if len(sessionIDs) == 0 && len(threadIDs) == 0 {
		return CodexHistoryRestoreResult{SkippedReason: "no_backup_ledger"}, nil
	}
	restoreRoot, err := createCodexHistoryBackupRoot(codexHistoryRestoreBackupName)
	if err != nil {
		return CodexHistoryRestoreResult{}, err
	}

	restoredJSONL := 0
	for _, file := range collectCodexJSONLFiles(codexDir) {
		changed, _, err := rewriteCodexJSONLProvider(file, restoreRoot, nil, codexOfficialProvider, false, sessionIDs)
		if err != nil {
			return CodexHistoryRestoreResult{}, err
		}
		if changed {
			restoredJSONL++
		}
	}
	restoredRows := 0
	for _, dbPath := range collectCodexStateDBs(codexDir) {
		rows, err := restoreCodexStateDBProvider(dbPath, restoreRoot, threadIDs)
		if err != nil {
			return CodexHistoryRestoreResult{}, err
		}
		restoredRows += rows
	}
	if restoredJSONL == 0 && restoredRows == 0 {
		_ = os.RemoveAll(restoreRoot)
		return CodexHistoryRestoreResult{SkippedReason: "nothing_to_restore"}, nil
	}
	return CodexHistoryRestoreResult{RestoredJSONLFiles: restoredJSONL, RestoredStateRows: restoredRows}, nil
}

func (css *CodexSettingsService) HasCodexUnifiedHistoryBackup() (bool, error) {
	return HasCodexUnifiedHistoryBackup(), nil
}

func (css *CodexSettingsService) RestoreCodexUnifiedHistory() (CodexHistoryRestoreResult, error) {
	return RestoreCodexUnifiedHistoryFromBackups()
}

func codexConfigDirPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, codexSettingsDir), nil
}

func codexConfigRoutesUnified(configPath string) bool {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return false
	}
	var cfg codexConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return false
	}
	return strings.TrimSpace(cfg.ModelProvider) == codexProviderKey
}

func createCodexHistoryBackupRoot(name string) (string, error) {
	parent, err := codexHistoryBackupParent(name)
	if err != nil {
		return "", err
	}
	root := filepath.Join(parent, time.Now().Format("20060102-150405.000000000"))
	return root, os.MkdirAll(root, 0o755)
}

func codexHistoryBackupParent(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, appSettingsDir, "backups", name), nil
}

func canonicalPathString(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

func collectCodexJSONLFiles(codexDir string) []string {
	var files []string
	for _, name := range []string{"sessions", "archived_sessions"} {
		root := filepath.Join(codexDir, name)
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil || entry.IsDir() {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".jsonl") {
				files = append(files, path)
			}
			return nil
		})
	}
	return files
}

func collectCodexStateDBs(codexDir string) []string {
	var files []string
	_ = filepath.WalkDir(codexDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil || entry.IsDir() {
			return nil
		}
		if entry.Name() == codexStateDBName {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func rewriteCodexJSONLProvider(filePath string, backupRoot string, sourceProviders map[string]bool, targetProvider string, collectOfficial bool, restoreIDs map[string]bool) (bool, []string, error) {
	input, err := os.Open(filePath)
	if err != nil {
		return false, nil, err
	}
	defer input.Close()

	var lines []string
	var officialIDs []string
	changed := false
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		next, officialID, lineChanged := rewriteCodexSessionMetaLine(line, sourceProviders, targetProvider, collectOfficial, restoreIDs)
		if officialID != "" {
			officialIDs = append(officialIDs, officialID)
		}
		if lineChanged {
			changed = true
		}
		lines = append(lines, next)
	}
	if err := scanner.Err(); err != nil {
		return false, nil, err
	}
	if !changed {
		return false, officialIDs, nil
	}
	if err := backupCodexHistoryFile(filePath, backupRoot, "jsonl"); err != nil {
		return false, nil, err
	}
	return true, officialIDs, AtomicWriteBytes(filePath, []byte(strings.Join(lines, "\n")+"\n"))
}

func rewriteCodexSessionMetaLine(line string, sourceProviders map[string]bool, targetProvider string, collectOfficial bool, restoreIDs map[string]bool) (string, string, bool) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return line, "", false
	}
	if anyToString(payload["type"]) != "session_meta" {
		return line, "", false
	}
	current := anyToString(payload["model_provider"])
	if current == "" {
		current = anyToString(payload["provider_id"])
	}
	sessionID := anyToString(payload["id"])
	if sessionID == "" {
		sessionID = anyToString(payload["session_id"])
	}
	if restoreIDs != nil {
		if sessionID == "" || !restoreIDs[sessionID] || current != codexProviderKey {
			return line, "", false
		}
		payload["model_provider"] = targetProvider
		delete(payload, "provider_id")
		return marshalCodexJSONLine(payload, line)
	}
	if !sourceProviders[current] {
		return line, "", false
	}
	officialID := ""
	if collectOfficial && current == codexOfficialProvider {
		officialID = sessionID
	}
	payload["model_provider"] = targetProvider
	delete(payload, "provider_id")
	next, _, changed := marshalCodexJSONLine(payload, line)
	return next, officialID, changed
}

func marshalCodexJSONLine(payload map[string]any, fallback string) (string, string, bool) {
	data, err := json.Marshal(payload)
	if err != nil {
		return fallback, "", false
	}
	next := string(data)
	return next, "", next != fallback
}

func backupCodexHistoryFile(filePath string, backupRoot string, group string) error {
	codexDir, err := codexConfigDirPath()
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(codexDir, filePath)
	if err != nil {
		rel = filepath.Base(filePath)
	}
	backupPath := filepath.Join(backupRoot, group, rel)
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}
	return copyFileBytes(filePath, backupPath)
}

func copyFileBytes(src string, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeCodexHistoryLedger(root string, ledger codexHistoryLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteBytes(filepath.Join(root, "ledger.json"), data)
}

func collectCodexOfficialLedger(parent string, codexDir string) (map[string]bool, map[string]bool, error) {
	sessionIDs := map[string]bool{}
	threadIDs := map[string]bool{}
	entries, err := os.ReadDir(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return sessionIDs, threadIDs, nil
		}
		return nil, nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(parent, entry.Name(), "ledger.json"))
		if err != nil {
			continue
		}
		var ledger codexHistoryLedger
		if err := json.Unmarshal(data, &ledger); err != nil || ledger.CodexDir != codexDir {
			continue
		}
		for _, id := range ledger.OfficialSessionIDs {
			if strings.TrimSpace(id) != "" {
				sessionIDs[id] = true
			}
		}
		for _, id := range ledger.OfficialThreadIDs {
			if strings.TrimSpace(id) != "" {
				threadIDs[id] = true
			}
		}
	}
	return sessionIDs, threadIDs, nil
}

func migrateCodexStateDBProvider(dbPath string, backupRoot string, sourceProviders map[string]bool, targetProvider string) (int, []string, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, nil, err
	}
	defer db.Close()
	tables, err := codexStateProviderTables(db)
	if err != nil {
		return 0, nil, err
	}
	total := 0
	var officialIDs []string
	for _, table := range tables {
		ids, _ := collectCodexStateIDs(db, table, codexOfficialProvider)
		officialIDs = appendUniqueStrings(officialIDs, ids...)
		for source := range sourceProviders {
			rows, err := updateCodexStateProvider(db, table, source, targetProvider, dbPath, backupRoot)
			if err != nil {
				return 0, nil, err
			}
			total += rows
		}
	}
	return total, officialIDs, nil
}

func restoreCodexStateDBProvider(dbPath string, backupRoot string, threadIDs map[string]bool) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	tables, err := codexStateProviderTables(db)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, table := range tables {
		for id := range threadIDs {
			rows, err := updateCodexStateProviderByID(db, table, id, codexProviderKey, codexOfficialProvider, dbPath, backupRoot)
			if err != nil {
				return 0, err
			}
			total += rows
		}
	}
	return total, nil
}

type codexStateProviderTable struct {
	Name     string
	IDColumn string
}

func codexStateProviderTables(db *sql.DB) ([]codexStateProviderTable, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []codexStateProviderTable
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns := tableColumns(db, name)
		if !columns["model_provider"] {
			continue
		}
		idColumn := ""
		for _, candidate := range []string{"thread_id", "session_id", "conversation_id", "id"} {
			if columns[candidate] {
				idColumn = candidate
				break
			}
		}
		if idColumn != "" {
			tables = append(tables, codexStateProviderTable{Name: name, IDColumn: idColumn})
		}
	}
	return tables, rows.Err()
}

func tableColumns(db *sql.DB, table string) map[string]bool {
	columns := map[string]bool{}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return columns
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err == nil {
			columns[name] = true
		}
	}
	return columns
}

func collectCodexStateIDs(db *sql.DB, table codexStateProviderTable, provider string) ([]string, error) {
	rows, err := db.Query(
		fmt.Sprintf("SELECT %s FROM %s WHERE model_provider = ?", quoteSQLiteIdent(table.IDColumn), quoteSQLiteIdent(table.Name)),
		provider,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil && strings.TrimSpace(id) != "" {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

func updateCodexStateProvider(db *sql.DB, table codexStateProviderTable, source string, target string, dbPath string, backupRoot string) (int, error) {
	count, err := countCodexStateProviderRows(db, table, source, "")
	if err != nil || count == 0 {
		return 0, err
	}
	if err := backupCodexHistoryFile(dbPath, backupRoot, "state"); err != nil {
		return 0, err
	}
	result, err := db.Exec(
		fmt.Sprintf("UPDATE %s SET model_provider = ? WHERE model_provider = ?", quoteSQLiteIdent(table.Name)),
		target,
		source,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func updateCodexStateProviderByID(db *sql.DB, table codexStateProviderTable, id string, source string, target string, dbPath string, backupRoot string) (int, error) {
	count, err := countCodexStateProviderRows(db, table, source, id)
	if err != nil || count == 0 {
		return 0, err
	}
	if err := backupCodexHistoryFile(dbPath, backupRoot, "state"); err != nil {
		return 0, err
	}
	result, err := db.Exec(
		fmt.Sprintf("UPDATE %s SET model_provider = ? WHERE model_provider = ? AND %s = ?", quoteSQLiteIdent(table.Name), quoteSQLiteIdent(table.IDColumn)),
		target,
		source,
		id,
	)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func countCodexStateProviderRows(db *sql.DB, table codexStateProviderTable, provider string, id string) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE model_provider = ?", quoteSQLiteIdent(table.Name))
	args := []any{provider}
	if id != "" {
		query += fmt.Sprintf(" AND %s = ?", quoteSQLiteIdent(table.IDColumn))
		args = append(args, id)
	}
	var count int
	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func quoteSQLiteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func appendUniqueStrings(values []string, next ...string) []string {
	seen := make(map[string]bool, len(values)+len(next))
	for _, value := range values {
		seen[value] = true
	}
	for _, value := range next {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		values = append(values, value)
		seen[value] = true
	}
	return values
}
