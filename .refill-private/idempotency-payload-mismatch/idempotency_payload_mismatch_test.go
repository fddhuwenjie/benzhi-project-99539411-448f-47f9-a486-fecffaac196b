package bugtest

import (
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func TestIdempotencyPayloadMismatchIsRejected(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	base := workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "request-same", ActorID: "operator-one"},
		BatchID:     "batch-idempotency-payload",
		SiteCode:    "SITE-ORIGINAL",
		Species:     "Pinus",
		SampledAt:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		OperatorID:  "operator-one",
		Cores:       []domain.CoreSample{{CoreID: "core-one", TreeCode: "tree-one", RadiusCode: "A"}},
	}
	if _, err := service.Create(base); err != nil {
		t.Fatal(err)
	}

	changed := base
	changed.SiteCode = "SITE-DIFFERENT"
	changed.Species = "Quercus"
	result, err := service.Create(changed)
	if err == nil || result.Replayed {
		t.Fatalf("a changed request body reused cached success: replayed=%v err=%v", result.Replayed, err)
	}
	if !domain.IsCode(err, domain.ErrConflict) {
		t.Fatalf("changed request_id reuse should return a conflict, got %v", err)
	}
}
