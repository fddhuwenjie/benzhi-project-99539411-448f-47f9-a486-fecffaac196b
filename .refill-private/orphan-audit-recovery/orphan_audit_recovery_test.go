package bugtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func TestOrphanAuditRecoveryBeforeFirstSnapshot(t *testing.T) {
	root := t.TempDir()
	if _, err := repository.Open(root); err != nil {
		t.Fatal(err)
	}

	batchID := "batch-orphan-audit"
	event := domain.AuditEvent{
		EventID:    "event-orphan-audit",
		BatchID:    batchID,
		Revision:   1,
		RequestID:  "request-interrupted",
		Action:     "CREATE_BATCH",
		ActorID:    "operator-orphan",
		OccurredAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := domain.FinalizeEvent(&event); err != nil {
		t.Fatal(err)
	}
	line, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	path := filepath.Join(root, "audit", batchID+".jsonl")
	if err := os.WriteFile(path, line, 0o640); err != nil {
		t.Fatal(err)
	}

	store, err := repository.Open(root)
	if err != nil {
		t.Fatalf("startup should recover the interrupted first commit: %v", err)
	}
	service := workflow.New(store)
	_, err = service.Create(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "request-retry", ActorID: "operator-orphan"},
		BatchID:     batchID,
		SiteCode:    "SITE-O",
		Species:     "Pinus",
		SampledAt:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		OperatorID:  "operator-orphan",
		Cores:       []domain.CoreSample{{CoreID: "core-orphan", TreeCode: "tree-o", RadiusCode: "A"}},
	})
	if err != nil {
		t.Fatalf("retry after recovery should create the batch: %v", err)
	}
	if _, err := store.Events(batchID); err != nil {
		t.Fatalf("recovered store accepted a batch with an unusable audit chain: %v", err)
	}
}
