/**
 * @name: SQLite 双写队列测试与基准
 * @Descripttion: 验证 DBWriteQueue 的顺序、事务、批量、取消和关闭语义，并记录端到端性能基线
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-31 15:24:55
 * @LastEditTime: 2026-08-31 15:24:55
 * @FilePath: services/dbqueue_test.go
 */

package services

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const dbWriteQueueTestTimeout = 5 * time.Second

func newDBWriteQueueTestFixture(tb testing.TB, queueSize int, enableBatch bool) (*sql.DB, *DBWriteQueue) {
	tb.Helper()

	db, err := sql.Open("sqlite", filepath.Join(tb.TempDir(), "dbqueue.db"))
	if err != nil {
		tb.Fatalf("打开临时 SQLite 失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	statements := []string{
		"PRAGMA busy_timeout = 30000",
		"PRAGMA journal_mode = WAL",
		"CREATE TABLE dbqueue_items (sequence INTEGER PRIMARY KEY AUTOINCREMENT, value TEXT NOT NULL)",
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			_ = db.Close()
			tb.Fatalf("初始化临时 SQLite 失败: %v", err)
		}
	}

	queue := NewDBWriteQueue(db, queueSize, enableBatch)
	tb.Cleanup(func() {
		if !queue.closed.Load() {
			if err := queue.Shutdown(dbWriteQueueTestTimeout); err != nil {
				tb.Errorf("关闭测试写入队列失败: %v", err)
			}
		}
		if err := db.Close(); err != nil {
			tb.Errorf("关闭临时 SQLite 失败: %v", err)
		}
	})

	return db, queue
}

func waitForDBWriteQueueCondition(t *testing.T, condition func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(dbWriteQueueTestTimeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("等待%s超时", description)
		}
		runtime.Gosched()
	}
}

func receiveDBWriteQueueResult(t *testing.T, results <-chan error, description string) error {
	t.Helper()

	select {
	case err := <-results:
		return err
	case <-time.After(dbWriteQueueTestTimeout):
		t.Fatalf("等待%s超时", description)
		return nil
	}
}

func readDBWriteQueueValues(t *testing.T, db *sql.DB) []string {
	t.Helper()

	rows, err := db.Query("SELECT value FROM dbqueue_items ORDER BY sequence")
	if err != nil {
		t.Fatalf("读取队列写入结果失败: %v", err)
	}
	defer rows.Close()

	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatalf("扫描队列写入结果失败: %v", err)
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历队列写入结果失败: %v", err)
	}
	return values
}

func TestDBWriteQueueSingleWritesInOrder(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 16, false)
	want := []string{"first", "second", "third"}

	for _, value := range want {
		if err := queue.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", value); err != nil {
			t.Fatalf("单次写入 %q 失败: %v", value, err)
		}
	}

	if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, want) {
		t.Fatalf("写入顺序=%v，期望=%v", got, want)
	}
	stats := queue.GetStats()
	if stats.TotalWrites != 3 || stats.SuccessWrites != 3 || stats.FailedWrites != 0 {
		t.Fatalf("单次队列统计异常: %#v", stats)
	}
}

func TestDBWriteQueueTxGroupRollsBackAtomically(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 16, false)
	if err := queue.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", "existing"); err != nil {
		t.Fatalf("写入前置数据失败: %v", err)
	}

	err := queue.ExecTxGroup([]WriteTask{
		{SQL: "DELETE FROM dbqueue_items"},
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"temporary"}},
		{SQL: "INSERT INTO no_such_table VALUES (1)"},
	})
	if err == nil || !strings.Contains(err.Error(), "事务组执行失败") {
		t.Fatalf("事务组非法语句错误=%v", err)
	}
	if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, []string{"existing"}) {
		t.Fatalf("事务回滚后数据=%v，期望保留 existing", got)
	}

	if err := queue.ExecTxGroup([]WriteTask{
		{SQL: "DELETE FROM dbqueue_items"},
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"replacement"}},
	}); err != nil {
		t.Fatalf("合法事务组写入失败: %v", err)
	}
	if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, []string{"replacement"}) {
		t.Fatalf("事务提交后数据=%v，期望 replacement", got)
	}
}

func TestDBWriteQueueTxGroupRunsTransactionFunctionAtomically(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 16, false)

	if err := queue.ExecTxGroup([]WriteTask{
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"sql-task"}},
		{TxFunc: func(tx *sql.Tx) error {
			_, err := tx.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", "tx-function")
			return err
		}},
	}); err != nil {
		t.Fatalf("事务函数提交失败: %v", err)
	}
	if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, []string{"sql-task", "tx-function"}) {
		t.Fatalf("事务函数提交结果=%v", got)
	}

	err := queue.ExecTxGroup([]WriteTask{
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"rolled-back-sql"}},
		{TxFunc: func(tx *sql.Tx) error {
			if _, err := tx.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", "rolled-back-function"); err != nil {
				return err
			}
			return errors.New("forced transaction function failure")
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "forced transaction function failure") {
		t.Fatalf("事务函数失败错误=%v", err)
	}
	if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, []string{"sql-task", "tx-function"}) {
		t.Fatalf("事务函数失败后未整体回滚: %v", got)
	}
}

func TestDBWriteQueueTxGroupCanReuseTaskSliceAfterReturn(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 16, false)
	tasks := []WriteTask{
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"tx-a"}},
		{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"tx-b"}},
	}
	const repeats = 1000

	for i := 0; i < repeats; i++ {
		if err := queue.ExecTxGroup(tasks); err != nil {
			t.Fatalf("第 %d 次复用事务组失败: %v", i+1, err)
		}
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM dbqueue_items").Scan(&count); err != nil {
		t.Fatalf("统计事务组复用结果失败: %v", err)
	}
	if count != repeats*len(tasks) {
		t.Fatalf("事务组复用写入条数=%d，期望=%d", count, repeats*len(tasks))
	}
}

func TestDBWriteQueueBatchCommitsConcurrentWrites(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 64, true)
	const writes = 50
	start := make(chan struct{})
	results := make(chan error, writes)

	for i := 0; i < writes; i++ {
		go func() {
			<-start
			results <- queue.ExecBatch("INSERT INTO dbqueue_items (value) VALUES (?)", "batch")
		}()
	}
	close(start)

	for i := 0; i < writes; i++ {
		if err := receiveDBWriteQueueResult(t, results, "批量写入结果"); err != nil {
			t.Fatalf("批量写入失败: %v", err)
		}
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM dbqueue_items").Scan(&count); err != nil {
		t.Fatalf("统计批量写入结果失败: %v", err)
	}
	if count != writes {
		t.Fatalf("批量写入条数=%d，期望=%d", count, writes)
	}
	waitForDBWriteQueueCondition(t, func() bool {
		return queue.GetStats().TotalWrites == writes
	}, "批量队列统计更新")
	stats := queue.GetStats()
	if stats.TotalWrites != writes || stats.SuccessWrites != writes || stats.FailedWrites != 0 || stats.BatchCommits < 1 {
		t.Fatalf("批量队列统计异常: %#v", stats)
	}
}

func TestDBWriteQueueContextCancellationPreservesQueuedWrite(t *testing.T) {
	tests := []struct {
		name        string
		enableBatch bool
	}{
		{name: "single", enableBatch: false},
		{name: "batch", enableBatch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, queue := newDBWriteQueueTestFixture(t, 16, tt.enableBatch)
			connection, err := db.Conn(context.Background())
			if err != nil {
				t.Fatalf("占用 SQLite 连接失败: %v", err)
			}
			t.Cleanup(func() { _ = connection.Close() })

			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() {
				if tt.enableBatch {
					result <- queue.ExecBatchCtx(ctx, "INSERT INTO dbqueue_items (value) VALUES (?)", tt.name)
					return
				}
				result <- queue.ExecCtx(ctx, "INSERT INTO dbqueue_items (value) VALUES (?)", tt.name)
			}()

			waitForDBWriteQueueCondition(t, func() bool {
				return db.Stats().WaitCount > 0
			}, "队列任务等待数据库连接")
			cancel()
			if err := receiveDBWriteQueueResult(t, result, "取消结果"); !errors.Is(err, context.Canceled) {
				t.Fatalf("取消错误=%v，期望 context.Canceled", err)
			}

			if err := connection.Close(); err != nil {
				t.Fatalf("释放 SQLite 连接失败: %v", err)
			}
			waitForDBWriteQueueCondition(t, func() bool {
				return queue.GetStats().TotalWrites == 1
			}, "已入队任务完成")
			if got := readDBWriteQueueValues(t, db); !reflect.DeepEqual(got, []string{tt.name}) {
				t.Fatalf("取消后已入队任务结果=%v，期望仍完成写入", got)
			}
		})
	}
}

func TestDBWriteQueueShutdownDrainsAndRejectsWrites(t *testing.T) {
	tests := []struct {
		name        string
		enableBatch bool
	}{
		{name: "single", enableBatch: false},
		{name: "batch", enableBatch: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, queue := newDBWriteQueueTestFixture(t, 16, tt.enableBatch)
			connection, err := db.Conn(context.Background())
			if err != nil {
				t.Fatalf("占用 SQLite 连接失败: %v", err)
			}
			t.Cleanup(func() { _ = connection.Close() })

			const writes = 8
			results := make(chan error, writes)
			exec := func(value string) error {
				if tt.enableBatch {
					return queue.ExecBatch("INSERT INTO dbqueue_items (value) VALUES (?)", value)
				}
				return queue.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", value)
			}

			go func() { results <- exec("first") }()
			waitForDBWriteQueueCondition(t, func() bool {
				return db.Stats().WaitCount > 0
			}, "首个任务占用 worker")
			for i := 1; i < writes; i++ {
				go func() { results <- exec("pending") }()
			}
			waitForDBWriteQueueCondition(t, func() bool {
				if tt.enableBatch {
					return len(queue.batchQueue) == writes-1
				}
				return len(queue.queue) == writes-1
			}, "剩余任务全部入队")

			shutdownResult := make(chan error, 1)
			go func() { shutdownResult <- queue.Shutdown(dbWriteQueueTestTimeout) }()
			waitForDBWriteQueueCondition(t, queue.closed.Load, "队列进入关闭状态")
			if err := exec("rejected"); err == nil || !strings.Contains(err.Error(), "写入队列已关闭") {
				t.Fatalf("关闭后写入错误=%v", err)
			}

			if err := connection.Close(); err != nil {
				t.Fatalf("释放 SQLite 连接失败: %v", err)
			}
			for i := 0; i < writes; i++ {
				if err := receiveDBWriteQueueResult(t, results, "关闭排空写入结果"); err != nil {
					t.Fatalf("关闭排空写入失败: %v", err)
				}
			}
			if err := receiveDBWriteQueueResult(t, shutdownResult, "队列关闭结果"); err != nil {
				t.Fatalf("关闭队列失败: %v", err)
			}

			var count int
			if err := db.QueryRow("SELECT COUNT(*) FROM dbqueue_items").Scan(&count); err != nil {
				t.Fatalf("统计关闭排空结果失败: %v", err)
			}
			if count != writes {
				t.Fatalf("关闭排空写入条数=%d，期望=%d", count, writes)
			}
		})
	}
}

func BenchmarkDBWriteQueue(b *testing.B) {
	b.Run("single", func(b *testing.B) {
		_, queue := newDBWriteQueueTestFixture(b, 5000, false)
		b.ReportAllocs()
		b.ResetTimer()
		started := time.Now()
		for i := 0; i < b.N; i++ {
			if err := queue.Exec("INSERT INTO dbqueue_items (value) VALUES (?)", "single"); err != nil {
				b.Fatalf("单次基准写入失败: %v", err)
			}
		}
		reportDBWriteQueueBenchmark(b, queue, time.Since(started))
	})

	b.Run("batch", func(b *testing.B) {
		_, queue := newDBWriteQueueTestFixture(b, 5000, true)
		var firstErr error
		var errorOnce sync.Once
		b.ReportAllocs()
		b.ResetTimer()
		started := time.Now()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if err := queue.ExecBatch("INSERT INTO dbqueue_items (value) VALUES (?)", "batch"); err != nil {
					errorOnce.Do(func() { firstErr = err })
				}
			}
		})
		elapsed := time.Since(started)
		b.StopTimer()
		if firstErr != nil {
			b.Fatalf("批量基准写入失败: %v", firstErr)
		}
		reportDBWriteQueueBenchmark(b, queue, elapsed)
	})

	b.Run("tx_group", func(b *testing.B) {
		_, queue := newDBWriteQueueTestFixture(b, 5000, false)
		b.ReportAllocs()
		b.ResetTimer()
		started := time.Now()
		for i := 0; i < b.N; i++ {
			if err := queue.ExecTxGroup([]WriteTask{
				{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"tx-a"}},
				{SQL: "INSERT INTO dbqueue_items (value) VALUES (?)", Args: []interface{}{"tx-b"}},
			}); err != nil {
				b.Fatalf("事务组基准写入失败: %v", err)
			}
		}
		reportDBWriteQueueBenchmark(b, queue, time.Since(started))
	})
}

func reportDBWriteQueueBenchmark(b *testing.B, queue *DBWriteQueue, elapsed time.Duration) {
	b.Helper()
	b.StopTimer()
	if err := queue.Shutdown(dbWriteQueueTestTimeout); err != nil {
		b.Fatalf("关闭基准写入队列失败: %v", err)
	}
	stats := queue.GetStats()
	b.ReportMetric(float64(elapsed.Nanoseconds())/float64(b.N), "e2e-ns/op")
	b.ReportMetric(float64(b.N)/elapsed.Seconds(), "ops/s")
	b.ReportMetric(stats.AvgLatencyMs, "queue-avg-ms")
	b.ReportMetric(stats.P99LatencyMs, "queue-p99-ms")
	b.ReportMetric(float64(stats.TotalWrites)/float64(b.N), "queue-writes/op")
	if stats.BatchCommits > 0 {
		b.ReportMetric(float64(stats.TotalWrites)/float64(stats.BatchCommits), "tasks/commit")
	}
}
