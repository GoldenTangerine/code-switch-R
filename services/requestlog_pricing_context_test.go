/*
@name: 请求计费上下文测试
@Descripttion: 验证条件采集、响应覆盖、重试隔离与计费快照持久化。
@version: 1.0.0
@Author: sm
@Date: 2026-09-07 11:25:11
@LastEditTime: 2026-09-07 11:25:11
@FilePath: services/requestlog_pricing_context_test.go
*/
package services

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	modelpricing "codeswitch/resources/model-pricing"
)

func TestRequestLogPricingContextCaptureAndRetry(t *testing.T) {
	log := &ReqeustLog{CapturePayload: false}
	captureRequestLogPricingContext(log, []byte(`{"service_tier":"priority"}`), http.Header{"Anthropic-Beta": {"context-1m-2026-09-07"}, "Authorization": {"placeholder"}}, "/v1/responses")
	if !log.PricingContext.ConditionsKnown || log.PricingContext.ServiceTier != "priority" || log.PricingContext.Operation != "responses.create" || len(log.PricingContext.Headers) != 1 {
		t.Fatalf("wrong context: %+v", log.PricingContext)
	}
	CodexParseTokenUsageFromResponse(`{"response":{"service_tier":"flex"}}`, log)
	if log.PricingContext.ServiceTier != "flex" {
		t.Fatal("actual tier did not override request")
	}
	log.PricingSnapshot = &modelpricing.PricingSnapshot{TrackLabel: "old"}
	captureRequestLogPricingContext(log, []byte(`{}`), nil, "/v1/messages")
	if log.PricingContext.ServiceTier != "" || len(log.PricingContext.Headers) != 0 || log.PricingSnapshot != nil {
		t.Fatal("retry leaked pricing data")
	}
	resetGeminiRequestLogAttempt(log, time.Now())
	if log.PricingContext != nil || log.PricingSnapshot != nil {
		t.Fatal("attempt reset retained pricing data")
	}
}

func TestResponsePricingContextParsers(t *testing.T) {
	for _, parser := range []func(string, *ReqeustLog){ClaudeCodeParseTokenUsageFromResponse, CodexParseTokenUsageFromResponse, GeminiParseTokenUsageFromResponse} {
		for _, payload := range []string{`{"service_tier":"flex"}`, `{"response":{"service_tier":"flex"}}`, `{"message":{"service_tier":"flex"}}`, `{"usage":{"service_tier":"flex"}}`} {
			log := &ReqeustLog{PricingContext: &modelpricing.PricingContext{ConditionsKnown: true, ServiceTier: "priority"}}
			parser(payload, log)
			if log.PricingContext.ServiceTier != "flex" {
				t.Fatalf("tier not captured: %s", payload)
			}
		}
	}
	log := &ReqeustLog{IsStream: true, PricingContext: &modelpricing.PricingContext{ConditionsKnown: true}}
	hook := requestLogPricingResponseHook(log)
	hook([]byte("data: {\"response\":{\"service_tier\":\"pri"))
	hook([]byte("ority\"}}\n\n"))
	if log.PricingContext.ServiceTier != "priority" {
		t.Fatal("fragmented upstream response lost tier")
	}
	parseGeminiUsageMetadata([]byte(`{"service_tier":"flex"}`), log)
	if log.PricingContext.ServiceTier != "flex" {
		t.Fatal("nonstream Gemini lost tier")
	}
}

func TestPricingSnapshotPreservesRecordedZero(t *testing.T) {
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	for _, complete := range []bool{true, false} {
		snapshot := &modelpricing.PricingSnapshot{UnitPrices: map[string]float64{"prompt": 0}, FieldSources: map[string]string{"prompt": "provider_api"}, Complete: complete}
		log := &ReqeustLog{Model: "claude-sonnet-4-5", RequestedModel: "claude-sonnet-4-5", InputTokens: 1000000, PriceSource: "mixed", PricingSnapshot: decodeRequestLogPricingSnapshot(encodeRequestLogPricingSnapshot(snapshot))}
		before := *log
		applyLogPricing(pricing, log)
		if !reflect.DeepEqual(before, *log) {
			t.Fatalf("recorded zero changed: %+v", log)
		}
	}
	if normalizeRequestLogPriceSource("mixed", 0) != "mixed" {
		t.Fatal("mixed source lost")
	}
}

func TestPricingSnapshotGeminiPersistence(t *testing.T) {
	db, queue := newDBWriteQueueTestFixture(t, 16, true)
	if err := ensureRequestLogTableWithDB(db); err != nil {
		t.Fatal(err)
	}
	previous := GlobalDBQueueLogs
	GlobalDBQueueLogs = queue
	t.Cleanup(func() { GlobalDBQueueLogs = previous })
	pricing, err := modelpricing.NewService()
	if err != nil {
		t.Fatal(err)
	}
	pricing.ApplyOverrides(map[string]modelpricing.PricingEntry{"snapshot-model": {CloudPricing: &modelpricing.CloudPricingRules{Charges: map[string]float64{"prompt": 0}, Tracks: []modelpricing.PricingTrack{{Label: "free", Factor: 1}}}}}, nil)
	log := &ReqeustLog{Platform: "gemini", Model: "snapshot-model", RequestedModel: "snapshot-model", InputTokens: 10, HttpCode: 200, PricingContext: &modelpricing.PricingContext{ConditionsKnown: true}}
	prs := &ProviderRelayService{}
	prs.persistGeminiRequestLog(log, time.Now(), pricing)
	if err := queue.Shutdown(dbWriteQueueTestTimeout); err != nil {
		t.Fatal(err)
	}
	var encoded string
	if err := db.QueryRow("SELECT pricing_snapshot FROM request_log WHERE model = ?", "snapshot-model").Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	snapshot := decodeRequestLogPricingSnapshot(encoded)
	if snapshot == nil || snapshot.TrackLabel != "free" || !snapshot.Complete {
		t.Fatalf("snapshot missing after persistence: %s", encoded)
	}
}
