/**
 * @name: OpenClaw JSON5 归一化
 * @Descripttion: 读取 ~/.openclaw/openclaw.json（JSON5）为通用 map，并以标准 JSON 原子写回
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 12:00:00
 * @LastEditTime: 2026-08-17 12:00:00
 * @FilePath: services/openclaw_json5.go
 */

package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// OpenClaw 配置目录名（~/.openclaw）
	openClawDirName = ".openclaw"
	// OpenClaw 主配置文件名（用户侧为 JSON5，本应用写回标准 JSON 亦兼容）
	openClawConfigFileName = "openclaw.json"
)

// getOpenClawDir OpenClaw 配置目录（~/.openclaw）
func getOpenClawDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", openClawDirName)
	}
	return filepath.Join(home, openClawDirName)
}

// getOpenClawConfigPath OpenClaw 主配置文件路径（~/.openclaw/openclaw.json）
func getOpenClawConfigPath() string {
	return filepath.Join(getOpenClawDir(), openClawConfigFileName)
}

// readOpenClawLiveMap 读取 live 配置为通用 map（保留 env/tools/agents 等全部顶层键）
// 文件不存在或为空视为空配置；JSON5 语法（注释/单引号/裸键/尾逗号/十六进制）自动归一化
func readOpenClawLiveMap() (map[string]any, error) {
	data, err := os.ReadFile(getOpenClawConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return map[string]any{}, nil
	}
	config, err := unmarshalOpenClawLiveConfig(data)
	if err != nil {
		return nil, fmt.Errorf("解析 OpenClaw 配置失败 (%s): %w", getOpenClawConfigPath(), err)
	}
	return config, nil
}

// unmarshalOpenClawLiveConfig JSON5 归一化后反序列化为 map（顶层非对象时返回错误）
func unmarshalOpenClawLiveConfig(data []byte) (map[string]any, error) {
	jsonData, err := normalizeJSON5ToJSON(data)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, err
	}
	if config == nil {
		config = map[string]any{}
	}
	return config, nil
}

// writeOpenClawLiveMap 原子写回 live 配置（标准 JSON 两空格缩进；用户注释丢失可接受，OpenClaw 兼容标准 JSON）
// 统一走 atomicWriteFile（临时文件 + fsync + rename），崩溃不会留下半写入文件
func writeOpenClawLiveMap(config map[string]any) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(getOpenClawConfigPath(), data, 0o644)
}

// readOpenClawLiveConfigBytes 快照 live 配置原始字节（事务回滚用）
func readOpenClawLiveConfigBytes() ([]byte, bool, error) {
	path := getOpenClawConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return data, true, nil
}

// restoreOpenClawLiveConfigBytes 恢复 live 配置快照（原不存在则删除）
func restoreOpenClawLiveConfigBytes(data []byte, exists bool) error {
	path := getOpenClawConfigPath()
	if !exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ========== JSON5 → JSON 归一化 ==========
// 支持：块/行注释、单引号字符串、裸键（标识符作为对象键）、尾逗号、十六进制数字、前导 + 号、前/后导小数点
// 不支持：多行字符串、逗号换行省略、Infinity/NaN 等（OpenClaw 配置不使用这些深层特性）

// normalizeJSON5ToJSON 将 JSON5 文本归一化为标准 JSON
func normalizeJSON5ToJSON(data []byte) ([]byte, error) {
	data = bytes.TrimPrefix(data, []byte("\ufeff"))
	var out bytes.Buffer
	i, n := 0, len(data)
	for i < n {
		c := data[i]
		switch {
		case c == '/' && i+1 < n && data[i+1] == '/':
			// 行注释：整体替换为空格（保留行尾换行符）
			i += 2
			for i < n && data[i] != '\n' {
				i++
			}
			out.WriteByte(' ')
		case c == '/' && i+1 < n && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return nil, fmt.Errorf("JSON5 块注释未闭合")
			}
			for _, b := range data[i : i+2+end+2] {
				if b == '\n' {
					out.WriteByte('\n')
				}
			}
			out.WriteByte(' ')
			i += 2 + end + 2
		case c == '"' || c == '\'':
			token, next, err := normalizeJSON5String(data, i)
			if err != nil {
				return nil, err
			}
			out.WriteString(token)
			i = next
		case c == ',':
			// 尾逗号：前瞻（跳过空白与注释）若下一个有效字符是 } 或 ] 则丢弃
			if effective := nextJSON5Byte(data, i+1); effective == '}' || effective == ']' {
				i++
			} else {
				out.WriteByte(c)
				i++
			}
		case json5StartsNumber(data, i):
			token, next, err := normalizeJSON5Number(data, i)
			if err != nil {
				return nil, err
			}
			out.WriteString(token)
			i = next
		case json5IsIdentStart(c) || (c == '-' && i+1 < n && json5IsIdentStart(data[i+1])):
			// 标识符：true/false/null 关键字透传，其余视为裸键补双引号
			j := i + 1
			for j < n && json5IsIdentPart(data[j]) {
				j++
			}
			word := string(data[i:j])
			if word == "true" || word == "false" || word == "null" {
				out.WriteString(word)
			} else {
				out.WriteString(strconv.Quote(word))
			}
			i = j
		default:
			out.WriteByte(c)
			i++
		}
	}
	return out.Bytes(), nil
}

// normalizeJSON5String 解析单/双引号字符串字面量并输出标准 JSON 双引号字符串
// 转义序列解码后统一重新转义（\' 与 \v 等 JSON5 专属转义在此转换为合法 JSON）
func normalizeJSON5String(data []byte, start int) (string, int, error) {
	quote := data[start]
	var content bytes.Buffer
	i := start + 1
	n := len(data)
	for i < n {
		c := data[i]
		if c == quote {
			encoded, err := json.Marshal(content.String())
			if err != nil {
				return "", 0, err
			}
			return string(encoded), i + 1, nil
		}
		if c == '\n' || c == '\r' {
			return "", 0, fmt.Errorf("JSON5 字符串包含未转义换行（不支持多行字符串）")
		}
		if c == '\\' {
			i++
			if i >= n {
				return "", 0, fmt.Errorf("JSON5 转义序列未闭合")
			}
			switch e := data[i]; e {
			case 'n':
				content.WriteByte('\n')
			case 't':
				content.WriteByte('\t')
			case 'r':
				content.WriteByte('\r')
			case 'b':
				content.WriteByte('\b')
			case 'f':
				content.WriteByte('\f')
			case 'v':
				content.WriteByte('\v')
			case '0':
				content.WriteByte(0)
			case '\n':
				// 行续接：丢弃换行
			case '\r':
				// CRLF 行续接：丢弃
				if i+1 < n && data[i+1] == '\n' {
					i++
				}
			case 'x':
				if i+2 >= n {
					return "", 0, fmt.Errorf("JSON5 \\x 转义序列不完整")
				}
				value, err := strconv.ParseUint(string(data[i+1:i+3]), 16, 8)
				if err != nil {
					return "", 0, fmt.Errorf("无效的 JSON5 \\x 转义序列")
				}
				content.WriteRune(rune(value))
				i += 2
			case 'u':
				if i+4 >= n {
					return "", 0, fmt.Errorf("JSON5 \\u 转义序列不完整")
				}
				value, err := strconv.ParseUint(string(data[i+1:i+5]), 16, 32)
				if err != nil {
					return "", 0, fmt.Errorf("无效的 JSON5 \\u 转义序列")
				}
				content.WriteRune(rune(value))
				i += 4
			default:
				// \' \" \\ \/ 及其他字符：去掉转义符按原样输出（输出阶段统一重新转义）
				content.WriteByte(e)
			}
			i++
			continue
		}
		content.WriteByte(c)
		i++
	}
	return "", 0, fmt.Errorf("JSON5 字符串未闭合")
}

// normalizeJSON5Number 归一化数字字面量：十六进制转十进制，前导 + 丢弃，前/后导小数点补 0
func normalizeJSON5Number(data []byte, start int) (string, int, error) {
	i := start
	n := len(data)
	sign := ""
	if data[i] == '-' {
		sign = "-"
		i++
	} else if data[i] == '+' {
		i++
	}
	// 十六进制：0x/0X 后跟至少一个十六进制数字
	if i+1 < n && data[i] == '0' && (data[i+1] == 'x' || data[i+1] == 'X') && i+2 < n && json5IsHexDigit(data[i+2]) {
		j := i + 2
		for j < n && json5IsHexDigit(data[j]) {
			j++
		}
		value, err := strconv.ParseInt(string(data[i+2:j]), 16, 64)
		if err != nil {
			return "", 0, fmt.Errorf("无效的十六进制数字: %s", string(data[i:j]))
		}
		return sign + strconv.FormatInt(value, 10), j, nil
	}
	j := i
	for j < n && (json5IsDigit(data[j]) || data[j] == '.') {
		j++
	}
	// 科学计数指数（e/E + 可选符号 + 数字）
	if j < n && (data[j] == 'e' || data[j] == 'E') {
		k := j + 1
		if k < n && (data[k] == '+' || data[k] == '-') {
			k++
		}
		if k < n && json5IsDigit(data[k]) {
			for k < n && json5IsDigit(data[k]) {
				k++
			}
			j = k
		}
	}
	token := string(data[i:j])
	if strings.HasPrefix(token, ".") {
		token = "0" + token
	}
	if strings.HasSuffix(token, ".") {
		token += "0"
	}
	return sign + token, j, nil
}

// nextJSON5Byte 前瞻下一个有效字节（跳过空白与注释；到达末尾返回 0）
func nextJSON5Byte(data []byte, from int) byte {
	i := from
	n := len(data)
	for i < n {
		c := data[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '/' && i+1 < n && data[i+1] == '/':
			for i < n && data[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && data[i+1] == '*':
			end := bytes.Index(data[i+2:], []byte("*/"))
			if end < 0 {
				return 0
			}
			i += 2 + end + 2
		default:
			return c
		}
	}
	return 0
}

// json5StartsNumber 判断当前位置是否开始数字字面量（含符号与前导小数点）
func json5StartsNumber(data []byte, i int) bool {
	if i >= len(data) {
		return false
	}
	c := data[i]
	if json5IsDigit(c) {
		return true
	}
	if c == '-' || c == '+' {
		return i+1 < len(data) && (json5IsDigit(data[i+1]) || data[i+1] == '.')
	}
	if c == '.' {
		return i+1 < len(data) && json5IsDigit(data[i+1])
	}
	return false
}

func json5IsDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func json5IsHexDigit(c byte) bool {
	return json5IsDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func json5IsIdentStart(c byte) bool {
	return c == '$' || c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func json5IsIdentPart(c byte) bool {
	return json5IsIdentStart(c) || json5IsDigit(c) || c == '-'
}
