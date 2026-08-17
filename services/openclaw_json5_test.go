/**
 * @name: OpenClaw JSON5 归一化测试
 * @Descripttion: 验证 JSON5 语料（注释/单引号/裸键/尾逗号/十六进制）归一化与 live 读写往返
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 12:20:00
 * @LastEditTime: 2026-08-17 12:20:00
 * @FilePath: services/openclaw_json5_test.go
 */

package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNormalizeJSON5MixedCorpus 混合语料：注释/尾逗号/单双引号/裸键/十六进制往返
func TestNormalizeJSON5MixedCorpus(t *testing.T) {
	source := `{
  // OpenClaw 允许行注释
  key: 'value',
  arr: [1, 2, 3,],
  "quoted": true,
  /* 块注释也允许 */
  nested: { a: 'x', },
  hex: 0x1F,
  $schema: 'https://example.com/schema.json',
  flag: false,
  nothing: null,
}`
	normalized, err := normalizeJSON5ToJSON([]byte(source))
	if err != nil {
		t.Fatalf("归一化 JSON5 失败: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("归一化结果不是合法 JSON: %v\n%s", err, normalized)
	}

	if parsed["key"] != "value" {
		t.Fatalf("key = %#v, want \"value\"", parsed["key"])
	}
	arr, ok := parsed["arr"].([]any)
	if !ok || len(arr) != 3 || arr[0] != float64(1) || arr[2] != float64(3) {
		t.Fatalf("arr = %#v, want [1 2 3]", parsed["arr"])
	}
	if parsed["quoted"] != true {
		t.Fatalf("quoted = %#v, want true", parsed["quoted"])
	}
	nested, ok := parsed["nested"].(map[string]any)
	if !ok || nested["a"] != "x" {
		t.Fatalf("nested = %#v, want {a: \"x\"}", parsed["nested"])
	}
	if parsed["hex"] != float64(31) {
		t.Fatalf("hex = %#v, want 31", parsed["hex"])
	}
	if parsed["$schema"] != "https://example.com/schema.json" {
		t.Fatalf("$schema = %#v", parsed["$schema"])
	}
	if parsed["flag"] != false || parsed["nothing"] != nil {
		t.Fatalf("关键字字面量处理错误: flag=%#v nothing=%#v", parsed["flag"], parsed["nothing"])
	}
}

// TestNormalizeJSON5StringEscapes 单引号字符串的 JSON5 专属转义（\' \v \x \u）转标准 JSON
func TestNormalizeJSON5StringEscapes(t *testing.T) {
	source := `{ s: 'it\'s "double" \n\x41é', t: "pre\"served", }`
	normalized, err := normalizeJSON5ToJSON([]byte(source))
	if err != nil {
		t.Fatalf("归一化 JSON5 失败: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("归一化结果不是合法 JSON: %v\n%s", err, normalized)
	}
	want := "it's \"double\" \nAé"
	if parsed["s"] != want {
		t.Fatalf("s = %#v, want %#v", parsed["s"], want)
	}
	if parsed["t"] != `pre"served` {
		t.Fatalf("t = %#v", parsed["t"])
	}
}

// TestNormalizeJSON5TrailingCommaWithComments 尾逗号与注释交错时的丢弃判定
func TestNormalizeJSON5TrailingCommaWithComments(t *testing.T) {
	source := `[
  1,
  2, // 行注释后接闭合括号
  /* 块注释也跳过 */
]`
	normalized, err := normalizeJSON5ToJSON([]byte(source))
	if err != nil {
		t.Fatalf("归一化 JSON5 失败: %v", err)
	}
	var parsed []any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("归一化结果不是合法 JSON: %v\n%s", err, normalized)
	}
	if len(parsed) != 2 || parsed[0] != float64(1) || parsed[1] != float64(2) {
		t.Fatalf("parsed = %#v, want [1 2]", parsed)
	}
}

// TestNormalizeJSON5Numbers 十六进制（含负数）、前导 + / 前后导小数点
func TestNormalizeJSON5Numbers(t *testing.T) {
	source := `{"a": -0x10, "b": +5, "c": .5, "d": 2., "e": 1e+3, "f": 0xFF,}`
	normalized, err := normalizeJSON5ToJSON([]byte(source))
	if err != nil {
		t.Fatalf("归一化 JSON5 失败: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("归一化结果不是合法 JSON: %v\n%s", err, normalized)
	}
	if parsed["a"] != float64(-16) {
		t.Fatalf("a = %#v, want -16", parsed["a"])
	}
	if parsed["b"] != float64(5) {
		t.Fatalf("b = %#v, want 5", parsed["b"])
	}
	if parsed["c"] != float64(0.5) {
		t.Fatalf("c = %#v, want 0.5", parsed["c"])
	}
	if parsed["d"] != float64(2) {
		t.Fatalf("d = %#v, want 2", parsed["d"])
	}
	if parsed["e"] != float64(1000) {
		t.Fatalf("e = %#v, want 1000", parsed["e"])
	}
	if parsed["f"] != float64(255) {
		t.Fatalf("f = %#v, want 255", parsed["f"])
	}
}

// TestNormalizeJSON5RejectsInvalidInput 未闭合注释/字符串/多行字符串报错
func TestNormalizeJSON5RejectsInvalidInput(t *testing.T) {
	cases := []string{
		`{"a": 1} /* 未闭合`,
		`{"a": 'unclosed}`,
		"{\"a\": 'multi\nline'}",
	}
	for _, source := range cases {
		if _, err := normalizeJSON5ToJSON([]byte(source)); err == nil {
			t.Fatalf("期望 %q 返回错误", source)
		}
	}
}

// TestReadOpenClawLiveMapMissingFile 配置文件不存在时返回空 map
func TestReadOpenClawLiveMapMissingFile(t *testing.T) {
	useIsolatedHomeDir(t)
	config, err := readOpenClawLiveMap()
	if err != nil {
		t.Fatalf("读取缺失的 OpenClaw 配置失败: %v", err)
	}
	if len(config) != 0 {
		t.Fatalf("期望空配置, 实际 %#v", config)
	}
}

// TestOpenClawLiveMapRoundTrip JSON5 读入 → 标准 JSON 写回 → 再次读入的往返保持
func TestOpenClawLiveMapRoundTrip(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	configPath := filepath.Join(homeDir, openClawDirName, openClawConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("创建 OpenClaw 配置目录失败: %v", err)
	}
	json5Source := `{
  // 用户注释
  env: { vars: { FOO: 'bar', }, },
  tools: { profile: 'coding', allow: ['git', 'ls',], },
  agents: { defaults: { timeoutSeconds: 120, }, },
  models: { providers: { manual: { baseUrl: 'https://manual.example', apiKey: 'sk-keep', }, }, },
}`
	if err := os.WriteFile(configPath, []byte(json5Source), 0o644); err != nil {
		t.Fatalf("写入 OpenClaw live 配置失败: %v", err)
	}

	config, err := readOpenClawLiveMap()
	if err != nil {
		t.Fatalf("读取 OpenClaw live 配置失败: %v", err)
	}
	if err := writeOpenClawLiveMap(config); err != nil {
		t.Fatalf("写回 OpenClaw live 配置失败: %v", err)
	}

	// 写回后必须是标准 JSON（无需归一化即可解析）
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("读取写回后的配置失败: %v", err)
	}
	if strings.Contains(string(raw), "// 用户注释") {
		t.Fatalf("写回后仍包含 JSON5 注释: %s", raw)
	}
	var standard map[string]any
	if err := json.Unmarshal(raw, &standard); err != nil {
		t.Fatalf("写回内容不是标准 JSON: %v\n%s", err, raw)
	}

	reread, err := readOpenClawLiveMap()
	if err != nil {
		t.Fatalf("二次读取 OpenClaw live 配置失败: %v", err)
	}
	env := openClawChildReadOnly(reread, "env")
	if openClawStringMap(env["vars"])["FOO"] != "bar" {
		t.Fatalf("env.vars.FOO 丢失: %#v", reread)
	}
	tools := openClawChildReadOnly(reread, "tools")
	if tools["profile"] != "coding" {
		t.Fatalf("tools.profile 丢失: %#v", reread)
	}
	agents := openClawChildReadOnly(reread, "agents")
	defaults := openClawChildReadOnly(agents, "defaults")
	if defaults["timeoutSeconds"] != float64(120) {
		t.Fatalf("agents.defaults.timeoutSeconds 丢失: %#v", reread)
	}
	providers, err := readOpenClawLiveProviders()
	if err != nil {
		t.Fatalf("读取 OpenClaw live providers 失败: %v", err)
	}
	if providers["manual"]["baseUrl"] != "https://manual.example" || providers["manual"]["apiKey"] != "sk-keep" {
		t.Fatalf("models.providers.manual 丢失: %#v", providers)
	}
}
