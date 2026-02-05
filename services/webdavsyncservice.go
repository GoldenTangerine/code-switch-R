package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/daodao97/xgo/xdb"
)

const (
	webdavConfigFileName     = "webdav.json"
	webdavBackupZipName      = "codeswitch-config.zip"
	webdavBackupZipSchemaVer = 1
	webdavZipRootDir         = "code-switch"
)

type WebDAVSyncConfig struct {
	Endpoint       string `json:"endpoint"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	RemoteDir      string `json:"remote_dir"`
	RemoteFile     string `json:"remote_file"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type WebDAVTestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	RemoteURL string `json:"remote_url,omitempty"`
}

type WebDAVSyncResult struct {
	OK         bool   `json:"ok"`
	Message    string `json:"message"`
	RemoteURL  string `json:"remote_url,omitempty"`
	Bytes      int64  `json:"bytes,omitempty"`
	BackupPath string `json:"backup_path,omitempty"`
}

type webdavBackupManifest struct {
	SchemaVersion int      `json:"schema_version"`
	CreatedAt     string   `json:"created_at"`
	Platform      string   `json:"platform"`
	Includes      []string `json:"includes"`
}

type WebDAVSyncService struct {
	mu         sync.Mutex
	configPath string
}

func NewWebDAVSyncService() *WebDAVSyncService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &WebDAVSyncService{
		configPath: filepath.Join(home, ".code-switch", webdavConfigFileName),
	}
}

func (s *WebDAVSyncService) Start() error { return nil }
func (s *WebDAVSyncService) Stop() error  { return nil }

func (s *WebDAVSyncService) GetConfig() (WebDAVSyncConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadConfigLocked()
}

func (s *WebDAVSyncService) SaveConfig(cfg WebDAVSyncConfig) (WebDAVSyncConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalized, err := normalizeWebDAVSyncConfig(cfg)
	if err != nil {
		return WebDAVSyncConfig{}, err
	}

	data, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return WebDAVSyncConfig{}, err
	}
	if err := atomicWriteFile(s.configPath, data, 0o600); err != nil {
		return WebDAVSyncConfig{}, err
	}
	return normalized, nil
}

func (s *WebDAVSyncService) TestConfig(cfg WebDAVSyncConfig) (WebDAVTestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.testConfigLocked(cfg)
}

func (s *WebDAVSyncService) SyncToWebDAV(cfg WebDAVSyncConfig) (WebDAVSyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := normalizeWebDAVSyncConfig(cfg)
	if err != nil {
		return WebDAVSyncResult{}, err
	}

	dirURL, fileURL, err := webdavResolveTargetURLs(cfg)
	if err != nil {
		return WebDAVSyncResult{}, err
	}

	client := newWebDAVClient(cfg.Username, cfg.Password, time.Duration(cfg.TimeoutSeconds)*time.Second)

	// timeout_seconds 主要约束网络请求，不要把本地打包时间也算进去（大配置/大 DB 会被误判超时）
	ensureCtx, ensureCancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	ensureErr := ensureWebDAVDir(ensureCtx, client, cfg.Endpoint, cfg.RemoteDir, dirURL)
	ensureCancel()
	if ensureErr != nil {
		return WebDAVSyncResult{}, ensureErr
	}

	zipPath, _, bytesWritten, err := exportLocalConfigZip()
	if err != nil {
		return WebDAVSyncResult{}, err
	}
	defer os.Remove(zipPath)

	file, err := os.Open(zipPath)
	if err != nil {
		return WebDAVSyncResult{}, err
	}
	defer file.Close()

	putCtx, putCancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	putErr := client.put(putCtx, fileURL, "application/zip", file)
	putCancel()
	if putErr != nil {
		return WebDAVSyncResult{}, putErr
	}

	return WebDAVSyncResult{
		OK:        true,
		Message:   "已同步到 WebDAV",
		RemoteURL: fileURL,
		Bytes:     bytesWritten,
	}, nil
}

func (s *WebDAVSyncService) LoadFromWebDAV(cfg WebDAVSyncConfig) (WebDAVSyncResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, err := normalizeWebDAVSyncConfig(cfg)
	if err != nil {
		return WebDAVSyncResult{}, err
	}

	_, fileURL, err := webdavResolveTargetURLs(cfg)
	if err != nil {
		return WebDAVSyncResult{}, err
	}

	client := newWebDAVClient(cfg.Username, cfg.Password, time.Duration(cfg.TimeoutSeconds)*time.Second)

	// 导入前先做一份本地备份，避免用户把自己坑死
	backupPath, _, _, err := exportLocalConfigZipToBackupDir()
	if err != nil {
		return WebDAVSyncResult{}, fmt.Errorf("本地备份失败: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "codeswitch-webdav-import-*.zip")
	if err != nil {
		return WebDAVSyncResult{}, err
	}
	tmpPath := tmpFile.Name()
	getCtx, getCancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	bytesDownloaded, err := client.getToWriter(getCtx, fileURL, tmpFile)
	getCancel()
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return WebDAVSyncResult{}, err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return WebDAVSyncResult{}, err
	}
	defer os.Remove(tmpPath)

	if err := importConfigZip(tmpPath); err != nil {
		return WebDAVSyncResult{}, err
	}

	return WebDAVSyncResult{
		OK:         true,
		Message:    "已从 WebDAV 加载到本地（已自动备份原配置）",
		RemoteURL:  fileURL,
		Bytes:      bytesDownloaded,
		BackupPath: backupPath,
	}, nil
}

func (s *WebDAVSyncService) loadConfigLocked() (WebDAVSyncConfig, error) {
	cfg := WebDAVSyncConfig{
		Endpoint:       "",
		Username:       "",
		Password:       "",
		RemoteDir:      "",
		RemoteFile:     webdavBackupZipName,
		TimeoutSeconds: 20,
	}
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	normalized, err := normalizeWebDAVSyncConfig(cfg)
	if err != nil {
		// 本地存量配置不阻断，先把能用的字段返回给前端
		return cfg, nil
	}
	return normalized, nil
}

func (s *WebDAVSyncService) testConfigLocked(cfg WebDAVSyncConfig) (WebDAVTestResult, error) {
	cfg, err := normalizeWebDAVSyncConfig(cfg)
	if err != nil {
		return WebDAVTestResult{OK: false, Message: err.Error()}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.TimeoutSeconds)*time.Second)
	defer cancel()

	dirURL, _, err := webdavResolveTargetURLs(cfg)
	if err != nil {
		return WebDAVTestResult{OK: false, Message: err.Error()}, nil
	}

	client := newWebDAVClient(cfg.Username, cfg.Password, time.Duration(cfg.TimeoutSeconds)*time.Second)
	if err := ensureWebDAVDir(ctx, client, cfg.Endpoint, cfg.RemoteDir, dirURL); err != nil {
		return WebDAVTestResult{OK: false, Message: err.Error()}, nil
	}

	// 写入/读取/删除测试文件，确保权限 OK
	testName := fmt.Sprintf(".codeswitch-webdav-test-%s.txt", randomHex(6))
	testURL, err := joinWebDAVURL(dirURL, testName)
	if err != nil {
		return WebDAVTestResult{OK: false, Message: err.Error()}, nil
	}
	testURL = strings.TrimSuffix(testURL, "/")

	payload := []byte("codeswitch webdav test " + time.Now().Format(time.RFC3339Nano))
	reader := bytes.NewReader(payload)
	if err := client.put(ctx, testURL, "text/plain; charset=utf-8", reader); err != nil {
		return WebDAVTestResult{OK: false, Message: "写入测试文件失败: " + err.Error()}, nil
	}

	got, err := client.get(ctx, testURL)
	if err != nil {
		_ = client.delete(ctx, testURL)
		return WebDAVTestResult{OK: false, Message: "读取测试文件失败: " + err.Error()}, nil
	}
	if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(payload)) {
		_ = client.delete(ctx, testURL)
		return WebDAVTestResult{OK: false, Message: "读取内容与写入不一致（可能是网盘做了转码/加密代理）"}, nil
	}

	if err := client.delete(ctx, testURL); err != nil {
		return WebDAVTestResult{OK: false, Message: "删除测试文件失败: " + err.Error()}, nil
	}

	return WebDAVTestResult{OK: true, Message: "WebDAV 配置正常（已验证读写）", RemoteURL: dirURL}, nil
}

func normalizeWebDAVSyncConfig(cfg WebDAVSyncConfig) (WebDAVSyncConfig, error) {
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Username = strings.TrimSpace(cfg.Username)
	// password 允许空格（有些人就这么设），只 trim 两侧常见错误
	cfg.Password = strings.TrimRight(strings.TrimLeft(cfg.Password, " \t\r\n"), " \t\r\n")
	cfg.RemoteDir = strings.TrimSpace(cfg.RemoteDir)
	cfg.RemoteFile = strings.TrimSpace(cfg.RemoteFile)
	if cfg.RemoteFile == "" {
		cfg.RemoteFile = webdavBackupZipName
	}
	if strings.Contains(cfg.RemoteFile, "/") || strings.Contains(cfg.RemoteFile, "\\") {
		return WebDAVSyncConfig{}, fmt.Errorf("远程文件名不应包含路径分隔符")
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 20
	}
	if cfg.TimeoutSeconds > 120 {
		cfg.TimeoutSeconds = 120
	}

	if cfg.Endpoint == "" {
		return WebDAVSyncConfig{}, fmt.Errorf("WebDAV 地址不能为空")
	}
	u, err := urlParseHTTP(cfg.Endpoint)
	if err != nil {
		return WebDAVSyncConfig{}, fmt.Errorf("WebDAV 地址不合法: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return WebDAVSyncConfig{}, fmt.Errorf("WebDAV 地址必须以 http:// 或 https:// 开头")
	}
	return cfg, nil
}

func urlParseHTTP(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("缺少 scheme 或 host")
	}
	return parsed, nil
}

func webdavResolveTargetURLs(cfg WebDAVSyncConfig) (dirURL string, fileURL string, err error) {
	dirURL, err = joinWebDAVURL(cfg.Endpoint, cfg.RemoteDir)
	if err != nil {
		return "", "", err
	}
	dirURL = ensureTrailingSlash(dirURL)

	fileURL, err = joinWebDAVURL(dirURL, cfg.RemoteFile)
	if err != nil {
		return "", "", err
	}
	fileURL = strings.TrimSuffix(fileURL, "/")
	return dirURL, fileURL, nil
}

func ensureWebDAVDir(ctx context.Context, client *webdavClient, endpoint string, remoteDir string, fullDirURL string) error {
	// 先尝试直接访问目录
	propErr := client.propfind(ctx, fullDirURL)
	if propErr == nil {
		return nil
	}

	remoteDir = strings.TrimSpace(remoteDir)
	if remoteDir == "" {
		// 用户没填 remote_dir，就不做创建，直接让错误冒出来
		return fmt.Errorf("WebDAV 目录不可访问: %w（可尝试补全地址或填写远程目录）", propErr)
	}

	segments := splitWebDAVPath(remoteDir)
	if len(segments) == 0 {
		return fmt.Errorf("远程目录不合法")
	}

	// 从 endpoint 开始逐级创建
	base := ensureTrailingSlash(strings.TrimSpace(endpoint))
	current := base
	for _, seg := range segments {
		next, err := joinWebDAVURL(current, seg)
		if err != nil {
			return err
		}
		next = ensureTrailingSlash(next)
		if err := client.mkcol(ctx, next); err != nil {
			return err
		}
		current = next
	}

	// 最终确认一次
	if err := client.propfind(ctx, fullDirURL); err != nil {
		return err
	}
	return nil
}

func splitWebDAVPath(p string) []string {
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return nil
	}
	raw := strings.Split(p, "/")
	out := make([]string, 0, len(raw))
	for _, seg := range raw {
		seg = strings.TrimSpace(seg)
		if seg == "" || seg == "." {
			continue
		}
		if seg == ".." {
			continue
		}
		out = append(out, seg)
	}
	return out
}

func exportLocalConfigZipToBackupDir() (zipPath string, includes []string, bytesWritten int64, err error) {
	codeDir, err := codeSwitchConfigDir()
	if err != nil {
		return "", nil, 0, err
	}
	backupDir := filepath.Join(codeDir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", nil, 0, err
	}
	name := fmt.Sprintf("backup-%s.zip", time.Now().Format("20060102-150405"))
	outPath := filepath.Join(backupDir, name)
	includes, bytesWritten, err = writeConfigZip(outPath)
	if err != nil {
		return "", nil, 0, err
	}
	return outPath, includes, bytesWritten, nil
}

func exportLocalConfigZip() (zipPath string, includes []string, bytesWritten int64, err error) {
	tmp, err := os.CreateTemp("", "codeswitch-webdav-export-*.zip")
	if err != nil {
		return "", nil, 0, err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	if err := os.Remove(tmpPath); err != nil {
		return "", nil, 0, err
	}
	includes, bytesWritten, err = writeConfigZip(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		return "", nil, 0, err
	}
	return tmpPath, includes, bytesWritten, nil
}

func writeConfigZip(outputPath string) (includes []string, bytesWritten int64, err error) {
	codeDir, err := codeSwitchConfigDir()
	if err != nil {
		return nil, 0, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, 0, err
	}

	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if err != nil {
			return
		}
		info, statErr := os.Stat(outputPath)
		if statErr == nil {
			bytesWritten = info.Size()
		}
	}()
	defer func() {
		if cerr := out.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	zw := zip.NewWriter(out)
	defer func() {
		if cerr := zw.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	files, err := collectConfigFiles(codeDir)
	if err != nil {
		return nil, 0, err
	}

	// 先写 manifest
	manifest := webdavBackupManifest{
		SchemaVersion: webdavBackupZipSchemaVer,
		CreatedAt:     time.Now().Format(time.RFC3339Nano),
		Platform:      runtime.GOOS,
		Includes:      make([]string, 0, len(files)),
	}

	// SQLite 快照：用 VACUUM INTO 保证一致性（避免 WAL/SHM 的坑）
	dbSnapshot, err := exportSQLiteSnapshot()
	if err != nil {
		return nil, 0, err
	}
	if dbSnapshot != "" {
		defer os.Remove(dbSnapshot)
		// 在遍历时跳过真实 app.db，改用快照写入
		for _, f := range files {
			if filepath.Base(f) == "app.db" {
				continue
			}
			rel, err := filepath.Rel(codeDir, f)
			if err != nil {
				return nil, 0, err
			}
			if strings.TrimSpace(rel) == "" || strings.HasPrefix(rel, "..") {
				continue
			}
			zipName := zipPathJoin(webdavZipRootDir, filepath.ToSlash(rel))
			if err := zipAddFile(zw, f, zipName); err != nil {
				return nil, 0, err
			}
			manifest.Includes = append(manifest.Includes, zipName)
		}

		// 写入快照为 code-switch/app.db
		if err := zipAddFile(zw, dbSnapshot, zipPathJoin(webdavZipRootDir, "app.db")); err != nil {
			return nil, 0, err
		}
		manifest.Includes = append(manifest.Includes, zipPathJoin(webdavZipRootDir, "app.db"))
	} else {
		// 没有 DB（或快照失败），退化为普通文件打包
		for _, f := range files {
			rel, err := filepath.Rel(codeDir, f)
			if err != nil {
				return nil, 0, err
			}
			if strings.TrimSpace(rel) == "" || strings.HasPrefix(rel, "..") {
				continue
			}
			zipName := zipPathJoin(webdavZipRootDir, filepath.ToSlash(rel))
			if err := zipAddFile(zw, f, zipName); err != nil {
				return nil, 0, err
			}
			manifest.Includes = append(manifest.Includes, zipName)
		}
	}

	sort.Strings(manifest.Includes)
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	mw, err := zw.Create("manifest.json")
	if err != nil {
		return nil, 0, err
	}
	if _, err := mw.Write(manifestBytes); err != nil {
		return nil, 0, err
	}

	return manifest.Includes, bytesWritten, nil
}

func zipAddFile(zw *zip.Writer, srcPath string, zipName string) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	h, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	h.Name = strings.TrimPrefix(strings.ReplaceAll(zipName, "\\", "/"), "/")
	h.Method = zip.Deflate

	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func zipPathJoin(parts ...string) string {
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ReplaceAll(p, "\\", "/")
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")
		if p == "" {
			continue
		}
		clean = append(clean, p)
	}
	return strings.Join(clean, "/")
}

func collectConfigFiles(codeDir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(codeDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(codeDir, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		relSlash := filepath.ToSlash(rel)
		// 排除更新缓存（体积大，没必要同步）
		if strings.HasPrefix(relSlash, "updates/") || relSlash == "updates" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// 排除 WebDAV 自己的配置（避免把密码打包上去）
		if relSlash == webdavConfigFileName {
			return nil
		}
		// 排除 WAL/SHM（快照会覆盖）
		base := filepath.Base(p)
		if base == "app.db-wal" || base == "app.db-shm" {
			return nil
		}

		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		out = append(out, p)
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func codeSwitchConfigDir() (string, error) {
	home, err := getUserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".code-switch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func exportSQLiteSnapshot() (string, error) {
	db, err := xdb.DB("default")
	if err != nil {
		return "", err
	}

	// 先尝试拿到 main DB 文件路径（有些环境 DSN 带参数）
	dbFile, err := resolveSQLiteMainDBFile(db)
	if err != nil || strings.TrimSpace(dbFile) == "" {
		// 回退到默认路径
		codeDir, err2 := codeSwitchConfigDir()
		if err2 != nil {
			return "", err2
		}
		dbFile = filepath.Join(codeDir, "app.db")
	}

	// 如果主库不存在，就不导出
	if _, err := os.Stat(dbFile); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	tmp, err := os.CreateTemp("", "codeswitch-sqlite-snapshot-*.db")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	if err := os.Remove(tmpPath); err != nil {
		return "", err
	}

	vacuumErr := vacuumSQLiteIntoPath(db, tmpPath)
	if vacuumErr == nil {
		if err := normalizeSQLiteSnapshotFile(tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return "", err
		}
		return tmpPath, nil
	}

	_ = os.Remove(tmpPath)
	fallbackErr := checkpointAndCopySQLiteDBToPath(db, dbFile, tmpPath)
	if fallbackErr == nil {
		if err := normalizeSQLiteSnapshotFile(tmpPath); err != nil {
			_ = os.Remove(tmpPath)
			return "", err
		}
		return tmpPath, nil
	}

	_ = os.Remove(tmpPath)
	return "", fmt.Errorf("导出 SQLite 快照失败（请稍后重试或暂停高频写入）: %w", errors.Join(vacuumErr, fallbackErr))
}

func vacuumSQLiteIntoPath(db *sql.DB, snapshotPath string) error {
	if db == nil {
		return errors.New("nil db")
	}
	escaped := strings.ReplaceAll(snapshotPath, "'", "''")
	if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", escaped)); err != nil {
		return err
	}
	if err := os.Chmod(snapshotPath, 0o600); err != nil {
		return err
	}
	return nil
}

func checkpointAndCopySQLiteDBToPath(db *sql.DB, dbFile string, snapshotPath string) error {
	if db == nil {
		return errors.New("nil db")
	}
	dbFile = strings.TrimSpace(dbFile)
	if dbFile == "" {
		return errors.New("empty sqlite db file")
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
		if err := checkpointAndCopySQLiteDBOnce(db, dbFile, snapshotPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func checkpointAndCopySQLiteDBOnce(db *sql.DB, dbFile string, snapshotPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
	}()

	stats, err := walCheckpointTruncate(ctx, conn)
	if err != nil {
		return err
	}
	if stats.Busy != 0 {
		return fmt.Errorf("wal checkpoint busy: busy=%d log=%d checkpointed=%d", stats.Busy, stats.Log, stats.Checkpointed)
	}

	if err := copyFileWithSync(dbFile, snapshotPath, 0o600); err != nil {
		return err
	}

	commitCtx, commitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer commitCancel()
	if _, err := conn.ExecContext(commitCtx, "COMMIT"); err != nil {
		return err
	}
	committed = true
	return nil
}

type walCheckpointStats struct {
	Busy         int64
	Log          int64
	Checkpointed int64
}

func walCheckpointTruncate(ctx context.Context, conn *sql.Conn) (walCheckpointStats, error) {
	rows, err := conn.QueryContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)")
	if err != nil {
		return walCheckpointStats{}, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return walCheckpointStats{}, err
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return walCheckpointStats{}, err
		}
		return walCheckpointStats{}, errors.New("wal_checkpoint returned no rows")
	}

	values := make([]sql.NullInt64, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range values {
		scanArgs[i] = &values[i]
	}
	if err := rows.Scan(scanArgs...); err != nil {
		return walCheckpointStats{}, err
	}
	if err := rows.Err(); err != nil {
		return walCheckpointStats{}, err
	}

	out := walCheckpointStats{}
	foundBusy := false
	foundLog := false
	foundCheckpointed := false
	for i, col := range cols {
		v := values[i]
		if !v.Valid {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(col)) {
		case "busy":
			out.Busy = v.Int64
			foundBusy = true
		case "log":
			out.Log = v.Int64
			foundLog = true
		case "checkpointed":
			out.Checkpointed = v.Int64
			foundCheckpointed = true
		}
	}

	if !foundBusy && len(values) >= 1 && values[0].Valid {
		out.Busy = values[0].Int64
	}
	if !foundLog && len(values) >= 2 && values[1].Valid {
		out.Log = values[1].Int64
	}
	if !foundCheckpointed && len(values) >= 3 && values[2].Valid {
		out.Checkpointed = values[2].Int64
	}

	return out, nil
}

func copyFileWithSync(srcPath string, dstPath string, perm os.FileMode) (err error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := dst.Close(); err == nil && cerr != nil {
			err = cerr
		}
	}()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	return nil
}

func normalizeSQLiteSnapshotFile(snapshotPath string) error {
	snapDB, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		return err
	}

	if _, err := snapDB.Exec("PRAGMA busy_timeout = 30000"); err != nil {
		_ = snapDB.Close()
		return err
	}

	var mode string
	if err := snapDB.QueryRow("PRAGMA journal_mode = DELETE").Scan(&mode); err != nil {
		_ = snapDB.Close()
		return err
	}
	_, _ = snapDB.Exec("PRAGMA optimize")

	if err := snapDB.Close(); err != nil {
		return err
	}

	// 尽量清理可能残留的 WAL/SHM（不同平台语义不同，忽略错误）
	_ = os.Remove(snapshotPath + "-wal")
	_ = os.Remove(snapshotPath + "-shm")
	return nil
}

func importConfigZip(zipPath string) error {
	codeDir, err := codeSwitchConfigDir()
	if err != nil {
		return err
	}

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	// 先找 DB 文件（如果有），提取到临时文件，后续用 ATTACH 导入
	var dbTemp string
	for _, f := range r.File {
		name := strings.TrimSpace(strings.ReplaceAll(f.Name, "\\", "/"))
		if name == zipPathJoin(webdavZipRootDir, "app.db") {
			dbTemp, err = extractZipFileToTemp(f, "codeswitch-import-db-*.db")
			if err != nil {
				return err
			}
			defer os.Remove(dbTemp)
			break
		}
	}

	// 写入普通文件
	for _, f := range r.File {
		name := strings.TrimSpace(strings.ReplaceAll(f.Name, "\\", "/"))
		if name == "" || name == "manifest.json" {
			continue
		}
		if strings.HasSuffix(name, "/") {
			continue
		}
		if !strings.HasPrefix(name, webdavZipRootDir+"/") {
			continue
		}

		rel := strings.TrimPrefix(name, webdavZipRootDir+"/")
		rel = strings.TrimSpace(rel)
		if rel == "" || rel == "." {
			continue
		}
		if strings.Contains(rel, "..") {
			return fmt.Errorf("zip 路径不安全: %s", name)
		}

		// DB 走专门导入
		if rel == "app.db" {
			continue
		}
		if rel == webdavConfigFileName {
			// 备份不包含，但保险起见导入时也跳过
			continue
		}

		dst := filepath.Join(codeDir, filepath.FromSlash(rel))
		if err := writeZipEntryAtomic(f, dst); err != nil {
			return err
		}
	}

	// 导入 DB（可选）
	if dbTemp != "" {
		if err := importSQLiteFromSnapshot(dbTemp); err != nil {
			return err
		}
	}

	return nil
}

func extractZipFileToTemp(f *zip.File, pattern string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, rc); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}
	return tmpPath, nil
}

func writeZipEntryAtomic(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	return atomicWriteFile(dst, data, 0o644)
}

func importSQLiteFromSnapshot(snapshotPath string) error {
	db, err := xdb.DB("default")
	if err != nil {
		return err
	}

	// 避免 request_log 的插入触发器导致统计重复
	_, _ = db.Exec(`DROP TRIGGER IF EXISTS request_log_stats_hourly_ai`)
	_, _ = db.Exec(`DROP TRIGGER IF EXISTS request_log_stats_daily_ai`)
	defer func() {
		// 无论导入成功与否，都尽量把触发器补回来，避免“导入失败=统计永久断电”
		_ = ensureRequestLogTableWithDB(db)
	}()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	escaped := strings.ReplaceAll(snapshotPath, "'", "''")
	if _, err = tx.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS remote", escaped)); err != nil {
		return err
	}
	attached := true
	defer func() {
		if attached {
			_, _ = tx.Exec("DETACH DATABASE remote")
		}
	}()

	tables, err := listSQLiteTables(tx, "remote")
	if err != nil {
		return err
	}

	remoteHasHourly := false
	remoteHasDaily := false
	for _, table := range tables {
		if strings.HasPrefix(table, "sqlite_") {
			continue
		}
		if table == requestLogStatsHourlyTable {
			remoteHasHourly = true
		}
		if table == requestLogStatsDailyTable {
			remoteHasDaily = true
		}
		exists, err := sqliteTableExists(tx, "main", table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		mainCols, err := listSQLiteColumns(tx, "main", table)
		if err != nil {
			return err
		}
		remoteCols, err := listSQLiteColumns(tx, "remote", table)
		if err != nil {
			return err
		}

		colList := intersectColumns(mainCols, remoteCols)
		if len(colList) == 0 {
			continue
		}

		if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM "%s"`, escapeSQLiteIdent(table))); err != nil {
			return err
		}

		colsCSV := joinIdentList(colList)
		insertSQL := fmt.Sprintf(`INSERT INTO "%s" (%s) SELECT %s FROM remote."%s"`,
			escapeSQLiteIdent(table),
			colsCSV,
			colsCSV,
			escapeSQLiteIdent(table),
		)
		if _, err := tx.Exec(insertSQL); err != nil {
			return err
		}
	}

	if attached {
		if _, err := tx.Exec("DETACH DATABASE remote"); err != nil {
			return err
		}
		attached = false
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// 处理统计 backfill：避免“无 key 时二次回填导致翻倍”
	// - 远端有 hourly+daily：认为统计已完整，直接标记迁移已应用，跳过 backfill
	// - 远端缺失统计：清空本地统计并删除迁移标记，让 ensure 走一次回填
	if err := ensureSchemaMigrationsTable(db); err == nil {
		if remoteHasHourly && remoteHasDaily {
			_ = markSchemaMigrationApplied(db, requestLogStatsMigrationKey, time.Now().Format(timeLayout))
		} else {
			_, _ = db.Exec("DELETE FROM schema_migrations WHERE key = ?", requestLogStatsMigrationKey)
			_, _ = db.Exec(fmt.Sprintf("DELETE FROM %s", requestLogStatsHourlyTable))
			_, _ = db.Exec(fmt.Sprintf("DELETE FROM %s", requestLogStatsDailyTable))
		}
	}

	// 表结构/索引/触发器兜底（上面的 defer 也会做一次，保持幂等）
	if err := ensureRequestLogTableWithDB(db); err != nil {
		return err
	}

	_, _ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	return nil
}

type sqliteQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

func listSQLiteTables(db sqliteQueryer, schema string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT name FROM %s.sqlite_master WHERE type='table'", escapeSQLiteIdent(schema)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name != "" {
			out = append(out, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func sqliteTableExists(db sqliteQueryer, schema string, table string) (bool, error) {
	row := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s.sqlite_master WHERE type='table' AND name=?", escapeSQLiteIdent(schema)), table)
	var n int
	if err := row.Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

func listSQLiteColumns(db sqliteQueryer, schema string, table string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA %s.table_info('%s')", escapeSQLiteIdent(schema), strings.ReplaceAll(table, "'", "''")))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		// cid, name, type, notnull, dflt_value, pk
		var cid int
		var name string
		var typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		name = strings.TrimSpace(name)
		if name != "" {
			out = append(out, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func intersectColumns(mainCols, remoteCols []string) []string {
	set := make(map[string]bool, len(remoteCols))
	for _, c := range remoteCols {
		set[c] = true
	}
	out := make([]string, 0, len(mainCols))
	for _, c := range mainCols {
		if set[c] {
			out = append(out, c)
		}
	}
	return out
}

func joinIdentList(cols []string) string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, fmt.Sprintf(`"%s"`, escapeSQLiteIdent(c)))
	}
	return strings.Join(out, ", ")
}

func escapeSQLiteIdent(s string) string {
	return strings.ReplaceAll(s, `"`, `""`)
}

func randomHex(nBytes int) string {
	if nBytes <= 0 {
		nBytes = 6
	}
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
