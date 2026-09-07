/*
@name: 请求计费上下文
@Descripttion: 采集必要计费条件并编解码已应用的价格快照。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:25:11
@LastEditTime: 2026-09-07 11:25:11
@FilePath: services/requestlog_pricing_context.go
*/
package services

import (
	"encoding/json"
	"net/http"
	"strings"

	modelpricing "codeswitch/resources/model-pricing"
	"github.com/tidwall/gjson"
)

func captureRequestLogPricingContext(log *ReqeustLog, body []byte, headers http.Header, endpoint string) {
	if log == nil {
		return
	}
	context := &modelpricing.PricingContext{ConditionsKnown: true}
	context.ServiceTier = strings.TrimSpace(gjson.GetBytes(body, "service_tier").String())
	if beta := headers.Get("anthropic-beta"); beta != "" {
		context.Headers = map[string]string{"anthropic-beta": beta}
	}
	switch {
	case strings.Contains(endpoint, "/responses"):
		context.Operation = "responses.create"
		context.OutputTokenMode = modelpricing.OutputIncludesReasoning
	case strings.Contains(endpoint, "/messages"):
		context.Operation = "messages.create"
	case strings.Contains(endpoint, "/chat/completions"):
		context.Operation = "chat.completions.create"
		context.OutputTokenMode = modelpricing.OutputIncludesReasoning
	case strings.Contains(endpoint, "streamGenerateContent"):
		context.Operation = "models.streamGenerateContent"
		context.OutputTokenMode = modelpricing.OutputExcludesReasoning
	case strings.Contains(endpoint, "generateContent"):
		context.Operation = "models.generateContent"
		context.OutputTokenMode = modelpricing.OutputExcludesReasoning
	}
	log.PricingContext = context
	log.PricingSnapshot = nil
}

func captureResponsePricingContext(data string, log *ReqeustLog) {
	if log == nil || log.PricingContext == nil {
		return
	}
	for _, path := range []string{"service_tier", "response.service_tier", "message.service_tier", "usage.service_tier", "response.usage.service_tier", "message.usage.service_tier"} {
		value := gjson.Get(data, path)
		if value.Type == gjson.String && strings.TrimSpace(value.String()) != "" {
			log.PricingContext.ServiceTier = strings.TrimSpace(value.String())
			return
		}
	}
}

func requestLogPricingResponseHook(log *ReqeustLog) func([]byte) (bool, []byte) {
	var remainder, rawJSON strings.Builder
	shadow := &ReqeustLog{}
	return func(data []byte) (bool, []byte) {
		if log != nil {
			shadow.IsStream = log.IsStream
			shadow.PricingContext = log.PricingContext
			parseTokenUsageChunk(data, shadow, captureResponsePricingContext, &remainder, &rawJSON, "")
		}
		return true, data
	}
}

func encodeRequestLogPricingSnapshot(snapshot *modelpricing.PricingSnapshot) string {
	if snapshot == nil {
		return ""
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	return string(data)
}

func decodeRequestLogPricingSnapshot(data string) *modelpricing.PricingSnapshot {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	var snapshot *modelpricing.PricingSnapshot
	if json.Unmarshal([]byte(data), &snapshot) != nil {
		return nil
	}
	return snapshot
}
