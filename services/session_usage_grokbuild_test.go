/**
 * @name: Grok Build 会话用量测试
 * @Descripttion: 验证会话记录 ID 的 scope 隔离与按文件清理仅作用于当前 scope
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 07:40:00
 * @LastEditTime: 2026-08-17 07:40:00
 * @FilePath: services/session_usage_grokbuild_test.go
 */

package services

import (
	"testing"
)

// TestBuildGrokSessionRecordIDScopeIsolation 记录 ID 必须携带 scope，避免不同 scope 的同路径记录互相覆盖
func TestBuildGrokSessionRecordIDScopeIsolation(t *testing.T) {
	id := buildGrokSessionRecordID("project", "sessions/a.jsonl", 12, "grok-4")
	if id != "grok_session:project:sessions/a.jsonl:12:grok-4" {
		t.Fatalf("记录 ID 格式错误: %s", id)
	}

	// 相同输入稳定（去重键需要确定性）
	if again := buildGrokSessionRecordID("project", "sessions/a.jsonl", 12, "grok-4"); again != id {
		t.Fatalf("相同输入的记录 ID 不稳定: %s vs %s", id, again)
	}

	// 不同 scope 的同路径同行号：必须互不冲突
	if other := buildGrokSessionRecordID("global", "sessions/a.jsonl", 12, "grok-4"); other == id {
		t.Fatalf("不同 scope 的记录 ID 冲突: %s", other)
	}

	// 同一行不同模型：追加模型名消歧
	if sameLine := buildGrokSessionRecordID("project", "sessions/a.jsonl", 12, "grok-3"); sameLine == id {
		t.Fatalf("同一行不同模型的记录 ID 冲突: %s", sameLine)
	}

	// 空模型省略尾段
	if bare := buildGrokSessionRecordID("project", "sessions/a.jsonl", 12, "  "); bare != "grok_session:project:sessions/a.jsonl:12" {
		t.Fatalf("空模型应省略尾段: %s", bare)
	}
}

// TestEscapeGrokLikePattern LIKE 通配符必须转义（配合 ESCAPE '\' 使用）
func TestEscapeGrokLikePattern(t *testing.T) {
	cases := map[string]string{
		`a_b`:     `a\_b`,
		`a%b`:     `a\%b`,
		`a\b`:     `a\\b`,
		`plain`:   `plain`,
		`a_b%c\d`: `a\_b\%c\\d`,
	}
	for in, want := range cases {
		if got := escapeGrokLikePattern(in); got != want {
			t.Errorf("escapeGrokLikePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestDeleteGrokSessionFileRecordsScopedToScope 按文件清理只删当前 scope 的记录，
// 其余 scope 的同路径记录与其他文件不受影响；路径中的 _ 不再被当作单字符通配符
func TestDeleteGrokSessionFileRecordsScopedToScope(t *testing.T) {
	_, db, scope := prepareSessionUsageTest(t)
	otherScope := scope + ":other"

	insert := func(recordScope string, relPath string, lineNumber int64) {
		t.Helper()
		if _, err := db.Exec(`
			INSERT INTO request_log (
				platform, model, provider_id, provider, http_code, input_tokens, output_tokens,
				cache_read_tokens, data_source, source_record_id, session_id, created_at
			) VALUES ('grokbuild', 'grok-4', '_grok_session', 'Grok 会话', 200, 10, 20, 0, 'grok_session', ?, ?, '2026-08-17 08:00:00')
		`, buildGrokSessionRecordID(recordScope, relPath, lineNumber, "grok-4"), relPath); err != nil {
			t.Fatalf("插入 fixture 失败: %v", err)
		}
	}

	targetPath := "sessions/target.jsonl"
	underscorePath := "sessions/target_v2.jsonl"
	otherPath := "sessions/other.jsonl"
	insert(scope, targetPath, 1)
	insert(scope, targetPath, 2)
	insert(otherScope, targetPath, 1)
	// 路径仅差一个字符（underscore 位置）：转义 _ 后不应被误删
	insert(scope, "sessions/targetXv2.jsonl", 1)
	insert(scope, underscorePath, 1)
	insert(otherScope, underscorePath, 1)
	insert(scope, otherPath, 1)

	if err := deleteGrokSessionFileRecords(scope, targetPath); err != nil {
		t.Fatalf("清理文件记录失败: %v", err)
	}

	// 与生产一致：转义 LIKE 通配符后按前缀计数（计数查询本身也不能把 _ 当通配符）
	count := func(prefix string) int {
		t.Helper()
		var n int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM request_log WHERE source_record_id LIKE ? ESCAPE '\\'",
			escapeGrokLikePattern(prefix)+"%",
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	if n := count("grok_session:" + scope + ":" + targetPath + ":"); n != 0 {
		t.Fatalf("当前 scope 目标文件记录应全部删除，剩余 %d", n)
	}
	if n := count("grok_session:" + otherScope + ":" + targetPath + ":"); n != 1 {
		t.Fatalf("其他 scope 的同路径记录不应被删除，剩余 %d", n)
	}
	if n := count("grok_session:" + scope + ":sessions/targetXv2.jsonl:"); n != 1 {
		t.Fatalf("仅差一个字符的路径不应被 _ 通配符误删，剩余 %d", n)
	}
	if n := count("grok_session:" + scope + ":" + underscorePath + ":"); n != 1 {
		t.Fatalf("其他文件的记录不应被删除，剩余 %d", n)
	}
	if n := count("grok_session:" + otherScope + ":" + underscorePath + ":"); n != 1 {
		t.Fatalf("其他 scope 其他文件的记录不应被删除，剩余 %d", n)
	}
	if n := count("grok_session:" + scope + ":" + otherPath + ":"); n != 1 {
		t.Fatalf("其他文件的记录不应被删除，剩余 %d", n)
	}
}
