package auditeventcachealias_test

import (
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func TestAuditEventCacheDoesNotLeakPayloadAliases(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatalf("打开仓储: %v", err)
	}
	service := workflow.New(store)
	now := time.Now().UTC()
	_, err = service.Create(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{
			RequestID:        "request-create-cache-alias",
			ExpectedRevision: 0,
			ActorID:          "operator-cache",
		},
		BatchID:    "batch-cache-alias",
		SiteCode:   "SITE-CACHE",
		Species:    "Pinus",
		SampledAt:  now.Add(-time.Hour),
		OperatorID: "operator-cache",
		Cores: []domain.CoreSample{{
			CoreID:     "core-cache",
			TreeCode:   "TREE-CACHE",
			RadiusCode: "R1",
		}},
	})
	if err != nil {
		t.Fatalf("创建测试批次: %v", err)
	}

	first, err := service.Events("batch-cache-alias")
	if err != nil {
		t.Fatalf("首次读取审计事件: %v", err)
	}
	payload, ok := first[0].Payload.(map[string]any)
	if !ok {
		t.Fatalf("审计 payload 类型为 %T", first[0].Payload)
	}
	payload["state"] = "POISONED_BY_CALLER"

	second, err := service.Events("batch-cache-alias")
	if err != nil {
		t.Fatalf("再次读取审计事件: %v", err)
	}
	secondPayload, payloadOK := second[0].Payload.(map[string]any)
	if payloadOK && secondPayload["state"] == string(domain.StateBaselined) && domain.VerifyEvent(second[0]) {
		return
	}
	durable, durableErr := store.Events("batch-cache-alias")
	if durableErr != nil || len(durable) != 1 || !domain.VerifyEvent(durable[0]) {
		t.Fatalf("磁盘审计证据也发生异常，无法隔离缓存别名: %v", durableErr)
	}
	t.Fatalf("调用方修改首次读取的 Payload 后，缓存复用返回了摘要失效的审计事件")
}
