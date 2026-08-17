/**
 * @name: services 包测试数据库基建
 * @Descripttion: 包级 TestMain 统一初始化测试用 SQLite 与双写队列，使供应商统一存储类测试无需各自初始化
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 03:40:00
 * @LastEditTime: 2026-08-17 03:40:00
 * @FilePath: services/testdb_main_test.go
 */

package services

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/daodao97/xgo/xdb"
)

// testMainHomeDir TestMain 初始化的全局测试库所在 HOME（供恢复 helper 使用）
var testMainHomeDir string

// TestMain 测试进程统一初始化数据库连接与写入队列
// - DB 指向独立 temp HOME 下的 app.db，避免污染真实 ~/.code-switch
// - 复用生产 InitDatabase / InitGlobalDBQueue，保证建表与迁移逻辑一致
// - 各测试内 t.Setenv("HOME", ...) 仅影响文件类路径（~/.claude 等），DB 路径已绑定不受影响
func TestMain(m *testing.M) {
	testHome, err := os.MkdirTemp("", "codeswitch-services-test-*")
	if err != nil {
		panic(err)
	}
	testMainHomeDir = testHome
	oldHome, hadHome := os.LookupEnv("HOME")
	_ = os.Setenv("HOME", testHome)

	if err := InitDatabase(); err != nil {
		_ = os.RemoveAll(testHome)
		panic(err)
	}
	if err := InitGlobalDBQueue(); err != nil {
		_ = os.RemoveAll(testHome)
		panic(err)
	}

	code := m.Run()

	_ = ShutdownGlobalDBQueue(3 * time.Second)
	_ = os.RemoveAll(testHome)
	if hadHome {
		_ = os.Setenv("HOME", oldHome)
	} else {
		_ = os.Unsetenv("HOME")
	}
	os.Exit(code)
}

// restoreGlobalTestDatabase 将全局 default 连接与双写队列恢复为 TestMain 的测试库
// 供那些"重绑临时库 + teardown 拆除全局"的测试在 Cleanup 末尾调用，避免污染后续测试
func restoreGlobalTestDatabase(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(testMainHomeDir, ".code-switch", "app.db?cache=shared&mode=rwc")
	if err := xdb.Inits([]xdb.Config{{Name: "default", Driver: "sqlite", DSN: dsn}}); err != nil {
		t.Fatalf("恢复测试全局数据库失败: %v", err)
	}
	if GlobalDBQueue == nil {
		if err := InitGlobalDBQueue(); err != nil {
			t.Fatalf("恢复测试全局写入队列失败: %v", err)
		}
	}
}

// migrateFixtureProvidersToStore 触发 JSON→SQLite 迁移器
// fixture 测试写好旧 JSON 后调用，等价于生产启动时的自动迁移链路
func migrateFixtureProvidersToStore(t *testing.T) {
	t.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateProviderJSONFilesToStore(db); err != nil {
		t.Fatalf("迁移 fixture JSON 到统一存储失败: %v", err)
	}
}

// resetProviderStoreForTest 清空统一存储全部行（测试前后各执行一次，隔离共享 DB 中的供应商数据）
func resetProviderStoreForTest(t *testing.T) {
	t.Helper()
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM providers_store"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM providers_store")
	})
}

// useIsolatedProviderTestDB 为单个测试提供独立数据库：
// 关闭全局队列 → 重绑 default 到 homeDir/.code-switch/app.db（含全部业务表）→ 重建队列
// Cleanup 恢复 TestMain 全局库与队列。库路径与 InitDatabase 语义一致，
// 测试内后续再调 InitDatabase() 也是同路径幂等重绑，不会造成写读分离。
func useIsolatedProviderTestDB(t *testing.T, homeDir string) *sql.DB {
	t.Helper()
	if GlobalDBQueue != nil || GlobalDBQueueLogs != nil {
		_ = ShutdownGlobalDBQueue(5 * time.Second)
		GlobalDBQueue = nil
		GlobalDBQueueLogs = nil
	}
	if err := os.MkdirAll(filepath.Join(homeDir, ".code-switch"), 0o755); err != nil {
		t.Fatalf("创建独立库目录失败: %v", err)
	}
	dbPath := filepath.Join(homeDir, ".code-switch", "app.db?cache=shared&mode=rwc")
	if err := xdb.Inits([]xdb.Config{{Name: "default", Driver: "sqlite", DSN: dbPath}}); err != nil {
		t.Fatalf("重绑独立测试数据库失败: %v", err)
	}
	db, err := xdb.DB("default")
	if err != nil {
		t.Fatalf("获取独立测试数据库失败: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 30000"); err != nil {
		t.Fatalf("设置独立库 busy_timeout 失败: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		t.Fatalf("设置独立库 WAL 失败: %v", err)
	}
	if err := ensureRequestLogTableWithDB(db); err != nil {
		t.Fatalf("初始化独立库 request_log 失败: %v", err)
	}
	if err := ensureBlacklistTables(); err != nil {
		t.Fatalf("初始化独立库黑名单表失败: %v", err)
	}
	if err := ensureProvidersStoreTable(); err != nil {
		t.Fatalf("初始化独立库供应商存储表失败: %v", err)
	}
	if err := InitGlobalDBQueue(); err != nil {
		t.Fatalf("初始化独立库写入队列失败: %v", err)
	}
	t.Cleanup(func() {
		_ = ShutdownGlobalDBQueue(5 * time.Second)
		GlobalDBQueue = nil
		GlobalDBQueueLogs = nil
		restoreGlobalTestDatabase(t)
	})
	return db
}
