/*
@name: 供应商工具兼容测试
@Descripttion: 验证工具过滤、历史配对、协议转换与供应商隔离。
@version: 1.0.0
@Author: sm
@Date: 2026-10-24 23:20:14
@LastEditTime: 2026-10-24 23:20:14
@FilePath: services/provider_tool_compatibility_test.go
*/
package services

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestProviderToolCompatibilityAdvisor(t *testing.T) {
	body := []byte(`{"model":"k3.1","thinking":{"type":"adaptive"},"output_config":{"effort":"max"},"tools":[{"type":"advisor_20260301","name":"advisor","model":"claude-opus-5-5","defer_loading":true},{"name":"Read","input_schema":{"type":"object"},"defer_loading":true}],"messages":[{"role":"user","content":"hi"},{"role":"system","content":[{"type":"text","text":"tools available"},{"type":"tool_addition","tool":{"type":"tool_reference","name":"advisor"}}]}]}`)
	original := append([]byte(nil), body...)
	provider := Provider{ToolCompatibility: &ProviderToolCompatibility{DisabledTypes: []string{"advisor_20260301"}, NormalizeDynamicTools: true}}
	out, report, err := applyProviderToolCompatibility(provider, "/v1/messages", body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, original) {
		t.Fatal("input request was mutated")
	}
	if strings.Contains(string(out), "advisor") || strings.Contains(string(out), "tool_addition") || strings.Contains(string(out), "defer_loading") {
		t.Fatalf("unsupported tool survived: %s", out)
	}
	if gjson.GetBytes(out, "tools.0.name").String() != "Read" || gjson.GetBytes(out, "messages.1.role").String() != "user" || gjson.GetBytes(out, "thinking.type").String() != "adaptive" || gjson.GetBytes(out, "output_config.effort").String() != "max" {
		t.Fatalf("normal content lost: %s", out)
	}
	if strings.Join(report, ",") != "advisor,dynamic-tools" {
		t.Fatal(report)
	}
	for _, endpoint := range []string{"/responses", "/v1/chat/completions"} {
		untouched, _, err := applyProviderToolCompatibility(provider, endpoint, body)
		if err != nil || !bytes.Equal(untouched, body) {
			t.Fatalf("changed unrelated protocol: %s", endpoint)
		}
	}
	untouched, _, _ := applyProviderToolCompatibility(Provider{}, "/v1/messages", body)
	if !bytes.Equal(untouched, body) {
		t.Fatal("unconfigured provider changed")
	}
}

func TestProviderToolCompatibilityHistoryAndChoice(t *testing.T) {
	for _, callType := range []string{"tool_use", "server_tool_use"} {
		t.Run(callType, func(t *testing.T) {
			body := []byte(`{"tools":[{"name":"advisor","type":"advisor_20260301"},{"name":"Read","input_schema":{"type":"object"}}],"tool_choice":{"type":"tool","name":"advisor"},"messages":[{"role":"assistant","content":[{"type":"` + callType + `","id":"blocked","name":"advisor","input":{"question":"keep this context"}},{"type":"tool_use","id":"allowed","name":"Read","input":{}}]},{"role":"user","content":[{"type":"advisor_tool_result","tool_use_id":"blocked","content":"useful answer"},{"type":"tool_result","tool_use_id":"allowed","content":"file data"}]}]}`)
			out, _, err := applyProviderToolCompatibility(Provider{ToolCompatibility: &ProviderToolCompatibility{DisabledNames: []string{" advisor ", "advisor"}}}, "/v1/messages", body)
			if err != nil {
				t.Fatal(err)
			}
			if gjson.GetBytes(out, "tool_choice").Exists() {
				t.Fatal("forced disabled tool retained")
			}
			for _, path := range []string{"messages.0.content.0", "messages.1.content.0"} {
				if gjson.GetBytes(out, path+".type").String() != "text" {
					t.Fatalf("historical call/result not converted: %s", out)
				}
			}
			if !strings.Contains(gjson.GetBytes(out, "messages.0.content.0.text").String(), "keep this context") || !strings.Contains(gjson.GetBytes(out, "messages.1.content.0.text").String(), "useful answer") {
				t.Fatal("history content lost")
			}
			if gjson.GetBytes(out, "messages.0.content.1.id").String() != "allowed" || gjson.GetBytes(out, "messages.1.content.1.tool_use_id").String() != "allowed" {
				t.Fatal("normal tool pair damaged")
			}
		})
	}
}

func TestProviderToolCompatibilityDynamicInlineAndNested(t *testing.T) {
	body := []byte(`{"messages":[{"role":"user","content":"hi"},{"role":"system","content":[{"type":"tool_addition","tool":{"name":"Read","input_schema":{"type":"object"},"defer_loading":true}},{"type":"tool_removal","tool":"Other"}]},{"role":"assistant","content":[{"type":"tool_use","id":"search","name":"ToolSearch","input":{}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"search","content":[{"type":"tool_reference","name":"Read"}]}]}]}`)
	out, _, err := applyProviderToolCompatibility(Provider{ToolCompatibility: &ProviderToolCompatibility{NormalizeDynamicTools: true}}, "/v1/messages", body)
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(out, "tools.0.name").String() != "Read" || gjson.GetBytes(out, "messages.#").Int() != 3 {
		t.Fatalf("inline tools not flattened: %s", out)
	}
	if gjson.GetBytes(out, "messages.2.content.0.content.0.type").String() != "text" {
		t.Fatal("nested reference not adapted")
	}
}

func TestProviderToolCompatibilityPlanAndClone(t *testing.T) {
	provider := Provider{ToolCompatibility: &ProviderToolCompatibility{DisabledNames: []string{"advisor"}, DisabledTypes: []string{"advisor_20260301"}}, RequestBodyOverrides: map[string]interface{}{"tools": []interface{}{map[string]interface{}{"name": "advisor", "type": "advisor_20260301"}}}}
	cloned := cloneProvider(provider)
	cloned.ToolCompatibility.DisabledNames[0] = "other"
	cloned.ToolCompatibility.DisabledTypes[0] = "other"
	if provider.ToolCompatibility.DisabledNames[0] != "advisor" || provider.ToolCompatibility.DisabledTypes[0] != "advisor_20260301" {
		t.Fatal("snapshot shares mutable rules")
	}
	for _, format := range []string{"anthropic", "openai_chat", "openai_responses"} {
		provider.APIFormat = format
		plan, err := buildProviderRequestPlan(provider, []byte(`{"model":"test","max_tokens":32,"messages":[{"role":"user","content":"hi"}]}`), "/v1/messages", "test")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(plan.BodyBytes), "advisor") {
			t.Fatalf("overrides or conversion reintroduced disabled tool: %s", plan.BodyBytes)
		}
	}
	encoded, err := json.Marshal(provider)
	if err != nil {
		t.Fatal(err)
	}
	var restored Provider
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ToolCompatibility.DisabledNames[0] != "advisor" {
		t.Fatal("rules lost on persistence round trip")
	}
}

func TestProviderToolCompatibilityNoMatchAndHeaders(t *testing.T) {
	body := []byte(`{ "model": "test", "tools": [{"name":"Read"}], "messages": [] }`)
	out, _, err := applyProviderToolCompatibility(Provider{ToolCompatibility: &ProviderToolCompatibility{DisabledNames: []string{"read"}}}, "/v1/messages", body)
	if err != nil || !bytes.Equal(out, body) {
		t.Fatal("exact matching or no-op preservation failed")
	}
	headers := map[string]string{"Anthropic-Beta": "context-1m-2025-08-07,inline-tools-2026-09-15,mid-conversation-tool-changes-2026-07-01,other"}
	provider := Provider{ToolCompatibility: &ProviderToolCompatibility{NormalizeDynamicTools: true}}
	applyProviderToolCompatibilityHeaders(provider, "/v1/messages", headers)
	if headers["Anthropic-Beta"] != "context-1m-2025-08-07,other" {
		t.Fatal(headers)
	}
}
