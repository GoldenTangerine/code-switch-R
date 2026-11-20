// @name: 应用原生技能状态
// @Descripttion: 使用 OpenCode 和 Grok Build 的单技能规则控制从其他目录读取的技能。
// @version: 1.0.0
// @Author: sm
// @Date: 2026-11-20 16:11:00
// @LastEditTime: 2026-11-20 16:11:00
// @FilePath: services/skillnative.go

package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/pelletier/go-toml/v2/unstable"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var skillNativeApps = []string{skillPlatformOpenCode, skillPlatformGrokBuild}

type skillNativeBlock struct {
	Name     string `json:"name"`
	Format   string `json:"format"`
	Previous string `json:"previous,omitempty"`
}

type skillNativeConfig struct {
	path   string
	data   []byte
	json   []byte
	format string
	err    error
	grok   grokSkillSettings
}

type grokSkillSettings struct {
	Skills struct {
		Disabled []string `toml:"disabled"`
	} `toml:"skills"`
	Compat struct {
		Claude struct {
			Skills *bool `toml:"skills"`
		} `toml:"claude"`
	} `toml:"compat"`
}

func (ss *SkillService) readSkillNativeConfigs() map[string]skillNativeConfig {
	configs := make(map[string]skillNativeConfig)
	for _, app := range skillNativeApps {
		configs[app] = ss.readSkillNativeConfig(app)
	}
	return configs
}

func (ss *SkillService) readSkillNativeConfig(app string) skillNativeConfig {
	root, err := ss.getInstallPath(app, skillLocationUser)
	if err != nil {
		return skillNativeConfig{err: err}
	}
	config := skillNativeConfig{format: "grok"}
	dir := filepath.Dir(root)
	if app == skillPlatformOpenCode {
		config.format = "permission"
		config.path = filepath.Join(dir, "opencode.json")
		if candidate := config.path + "c"; skillPathExists(candidate) {
			config.path = candidate
		}
	} else {
		config.path = filepath.Join(dir, "config.toml")
	}
	if resolved, err := filepath.EvalSymlinks(config.path); err == nil {
		config.path = resolved
	} else if skillPathExists(config.path) {
		config.err = err
		return config
	}
	config.data, err = os.ReadFile(config.path)
	if os.IsNotExist(err) {
		err = nil
	}
	if err != nil {
		config.err = err
		return config
	}
	if config.format == "grok" {
		config.err = toml.Unmarshal(config.data, &config.grok)
		if config.err != nil {
			config.err = fmt.Errorf("无法解析 %s: %w", config.path, config.err)
		}
		return config
	}
	data := config.data
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	config.json = maskSkillJSONC(data)
	if !json.Valid(config.json) || !gjson.ParseBytes(config.json).IsObject() {
		config.err = fmt.Errorf("无法解析 %s", config.path)
		return config
	}
	if gjson.GetBytes(config.json, "policies").Exists() || gjson.GetBytes(config.json, "permissions").Exists() ||
		gjson.GetBytes(config.json, "skills").IsArray() {
		config.format = "policies"
	}
	return config
}

// Keep byte offsets aligned with the original JSONC so edits preserve surrounding comments.
func maskSkillJSONC(input []byte) []byte {
	data := bytes.Clone(input)
	for index := 0; index < len(data); index++ {
		switch data[index] {
		case '"':
			for index++; index < len(data) && data[index] != '"'; index++ {
				if data[index] == '\\' {
					index++
				}
			}
		case '/':
			if index+1 >= len(data) {
				continue
			}
			start := index
			switch data[index+1] {
			case '/':
				for index < len(data) && data[index] != '\n' {
					index++
				}
			case '*':
				end := bytes.Index(data[index+2:], []byte("*/"))
				if end < 0 {
					return input
				}
				index += end + 4
			default:
				continue
			}
			for offset := start; offset < index; offset++ {
				if data[offset] != '\n' && data[offset] != '\r' {
					data[offset] = ' '
				}
			}
			index--
		}
	}
	inString, escaped := false, false
	for index, char := range data {
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
		} else if char == '"' {
			inString = true
		} else if char == ',' {
			next := bytes.TrimLeft(data[index+1:], " \t\r\n")
			if len(next) > 0 && (next[0] == ']' || next[0] == '}') {
				data[index] = ' '
			}
		}
	}
	return data
}

func skillJSONPath(name string) string {
	return strings.NewReplacer("\\", "\\\\", ".", "\\.", "*", "\\*", "?", "\\?", ":", "\\:").Replace(name)
}

func (config skillNativeConfig) blocks(name string) bool {
	if config.err != nil || config.path == "" {
		return false
	}
	switch config.format {
	case "grok":
		return slices.Contains(config.grok.Skills.Disabled, name)
	case "policies", "permission":
		if openCodeSkillEffect(config.json, "policies", "skill.use", name) == "deny" {
			return true
		}
		if effect := openCodeSkillEffect(config.json, "permissions", "skill", name); effect != "" {
			return effect == "deny"
		}
		permission := gjson.GetBytes(config.json, "permission.skill")
		if permission.Type == gjson.String {
			return permission.String() == "deny"
		}
		effect := ""
		permission.ForEach(func(key, value gjson.Result) bool {
			if matchesSkillPattern(key.String(), name) {
				effect = value.String()
			}
			return true
		})
		return effect == "deny"
	default:
		return false
	}
}

func matchesSkillPattern(pattern, name string) bool {
	matches, _ := path.Match(pattern, name)
	return matches || pattern == name
}

func openCodeSkillEffect(data []byte, field, action, name string) string {
	effect := ""
	for _, rule := range gjson.GetBytes(data, field).Array() {
		if rule.Get("action").String() == action && matchesSkillPattern(rule.Get("resource").String(), name) {
			effect = rule.Get("effect").String()
		}
	}
	return effect
}

func (config skillNativeConfig) openCodeRuleIndex(name string) int {
	rules := gjson.GetBytes(config.json, "policies").Array()
	for index := len(rules) - 1; index >= 0; index-- {
		rule := rules[index]
		if rule.Get("action").String() == "skill.use" && rule.Get("resource").String() == name &&
			rule.Get("effect").String() == "deny" {
			return index
		}
	}
	return -1
}

func (config skillNativeConfig) change(name string, block bool, receipt skillNativeBlock) ([]byte, skillNativeBlock, error) {
	if config.err != nil {
		return nil, receipt, config.err
	}
	if !block && receipt.Name == "" {
		return nil, receipt, nil
	}
	if block && config.blocks(name) {
		return nil, receipt, nil
	}
	if config.format == "grok" {
		disabled := slices.Clone(config.grok.Skills.Disabled)
		if block {
			disabled = append(disabled, name)
		} else if !slices.Contains(disabled, receipt.Name) {
			return nil, skillNativeBlock{}, nil
		} else {
			disabled = slices.DeleteFunc(disabled, func(value string) bool { return value == receipt.Name })
		}
		data, err := patchGrokSkillDisabled(config.data, disabled)
		if err == nil {
			var updated grokSkillSettings
			if err = toml.Unmarshal(data, &updated); err == nil && !slices.Equal(disabled, updated.Skills.Disabled) {
				err = errors.New("无法确认 Grok 技能禁用列表的更新")
			}
		}
		if block {
			return data, skillNativeBlock{Name: name, Format: config.format}, err
		}
		return data, skillNativeBlock{}, err
	}
	format := config.format
	if !block {
		format = receipt.Format
		name = receipt.Name
	}
	if strings.ContainsAny(name, "*?") {
		return nil, receipt, errors.New("技能名含通配符，无法设置 OpenCode 单技能规则")
	}
	var data []byte
	var err error
	switch format {
	case "policies":
		if block {
			if value := gjson.GetBytes(config.json, "policies"); value.Exists() && !value.IsArray() {
				return nil, receipt, errors.New("OpenCode policies 必须是列表")
			}
			data, err = sjson.SetBytes(config.json, "policies.-1", map[string]string{
				"action": "skill.use", "resource": name, "effect": "deny",
			})
		} else if index := config.openCodeRuleIndex(name); index >= 0 {
			data, err = sjson.DeleteBytes(config.json, fmt.Sprintf("policies.%d", index))
		}
	case "permission":
		path := "permission.skill." + skillJSONPath(name)
		if block {
			receipt.Previous = gjson.GetBytes(config.json, path).Raw
			data, err = config.withOpenCodeSkillDeny(path)
		} else if gjson.GetBytes(config.json, path).String() == "deny" {
			if receipt.Previous == "" {
				data, err = sjson.DeleteBytes(config.json, path)
			} else {
				data, err = sjson.SetRawBytes(config.json, path, []byte(receipt.Previous))
			}
		}
	}
	if err != nil {
		return nil, receipt, err
	}
	if block {
		receipt.Name, receipt.Format = name, format
	} else {
		receipt = skillNativeBlock{}
	}
	if data == nil {
		return nil, receipt, nil
	}
	merged, err := config.mergeOpenCodeJSON(data)
	return merged, receipt, err
}

func (config skillNativeConfig) withOpenCodeSkillDeny(path string) ([]byte, error) {
	permission := gjson.GetBytes(config.json, "permission")
	if permission.Type == gjson.String {
		name := strings.TrimPrefix(path, "permission.skill.")
		base, err := sjson.SetBytes(config.json, "permission", map[string]string{"*": permission.String()})
		if err != nil {
			return nil, err
		}
		return sjson.SetBytes(base, "permission.skill."+name, "deny")
	}
	skill := permission.Get("skill")
	if skill.Type == gjson.String {
		base, err := sjson.SetBytes(config.json, "permission.skill", map[string]string{"*": skill.String()})
		if err != nil {
			return nil, err
		}
		return sjson.SetBytes(base, path, "deny")
	}
	return sjson.SetBytes(config.json, path, "deny")
}

func (config skillNativeConfig) mergeOpenCodeJSON(next []byte) ([]byte, error) {
	if len(bytes.TrimSpace(config.data)) == 0 {
		return append(next, '\n'), nil
	}
	start, end, nextEnd := 0, len(config.json), len(next)
	for start < end && start < nextEnd && config.json[start] == next[start] {
		start++
	}
	for end > start && nextEnd > start && config.json[end-1] == next[nextEnd-1] {
		end--
		nextEnd--
	}
	out := slices.Concat(config.data[:start], next[start:nextEnd], config.data[end:])
	var actual, expected bytes.Buffer
	if json.Compact(&actual, maskSkillJSONC(out)) != nil || json.Compact(&expected, next) != nil ||
		!bytes.Equal(actual.Bytes(), expected.Bytes()) {
		return nil, errors.New("无法保留现有 OpenCode 配置格式并写入技能规则")
	}
	return out, nil
}

func patchGrokSkillDisabled(data []byte, disabled []string) ([]byte, error) {
	values := make([]string, 0, len(disabled))
	for _, name := range disabled {
		quoted, _ := json.Marshal(name)
		values = append(values, string(quoted))
	}
	value := "[" + strings.Join(values, ", ") + "]"
	parser := unstable.Parser{}
	parser.Reset(data)
	inSkills, hasDottedSkills := false, false
	insert, afterRoot := -1, len(data)
	for parser.NextExpression() {
		node := parser.Expression()
		key := node.Key()
		var parts []string
		var first unstable.Range
		for key.Next() {
			if len(parts) == 0 {
				first = key.Node().Raw
			}
			parts = append(parts, string(key.Node().Data))
		}
		switch node.Kind {
		case unstable.Table, unstable.ArrayTable:
			line := bytes.LastIndexByte(data[:first.Offset], '\n') + 1
			afterRoot = min(afterRoot, line)
			inSkills = node.Kind == unstable.Table && slices.Equal(parts, []string{"skills"})
			if inSkills {
				next := bytes.IndexByte(data[first.Offset:], '\n')
				insert = len(data)
				if next >= 0 {
					insert = int(first.Offset) + next + 1
				}
			}
		case unstable.KeyValue:
			if inSkills && slices.Equal(parts, []string{"disabled"}) ||
				afterRoot == len(data) && slices.Equal(parts, []string{"skills", "disabled"}) {
				start := int(first.Offset)
				start += bytes.IndexByte(data[start:], '=')
				start += bytes.IndexByte(data[start:], '[')
				end := start + 1
				for child := node.Value().Child(); child != nil; child = child.Next() {
					end = max(end, int(child.Raw.Offset+child.Raw.Length))
				}
				closing := bytes.IndexByte(data[end:], ']')
				if closing < 0 {
					return nil, errors.New("无法定位 Grok 的技能禁用列表")
				}
				return slices.Concat(data[:start], []byte(value), data[end+closing+1:]), nil
			}
			if afterRoot == len(data) && len(parts) > 0 && parts[0] == "skills" {
				if len(parts) == 1 {
					return nil, errors.New("请将 Grok 的 skills 内联表展开为 [skills] 表后再切换")
				}
				hasDottedSkills = true
			}
		}
	}
	if err := parser.Error(); err != nil {
		return nil, err
	}
	if insert >= 0 {
		return slices.Concat(data[:insert], []byte("\ndisabled = "+value+"\n"), data[insert:]), nil
	}
	if hasDottedSkills {
		return slices.Concat(data[:afterRoot], []byte("\nskills.disabled = "+value+"\n"), data[afterRoot:]), nil
	}
	return append(bytes.Clone(data), []byte("\n[skills]\ndisabled = "+value+"\n")...), nil
}

func (ss *SkillService) setSkillNativeBlock(changes *skillFileChanges, state *managedSkillState, app, name string, block bool) error {
	if state.NativeBlocks == nil {
		state.NativeBlocks = make(map[string]skillNativeBlock)
	}
	previous := state.NativeBlocks[app]
	config := ss.readSkillNativeConfig(app)
	if block && previous.Name != "" && previous.Name != name {
		return fmt.Errorf("%s 的技能名已变更，请先移除旧规则", app)
	}
	data, next, err := config.change(name, block, previous)
	if err != nil {
		return fmt.Errorf("%s: %w", app, err)
	}
	if data != nil && !bytes.Equal(config.data, data) {
		if err := changes.writeConfig(config.path, config.data, data); err != nil {
			return err
		}
		if block && !ss.readSkillNativeConfig(app).blocks(name) {
			return fmt.Errorf("%s 的其他规则覆盖了对 %s 的停用", app, name)
		}
	}
	if next.Name == "" {
		delete(state.NativeBlocks, app)
	} else {
		state.NativeBlocks[app] = next
	}
	return nil
}
