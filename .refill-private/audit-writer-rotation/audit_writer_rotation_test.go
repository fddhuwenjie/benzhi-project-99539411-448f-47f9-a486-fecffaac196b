package auditwriterrotation_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

func TestAuditWriterReopensAfterLogRotation(t *testing.T) {
	root := t.TempDir()
	store, err := repository.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	batch := &domain.DendroBatch{
		BatchID: "batch-writer-rotation", SiteCode: "SITE-R", Species: "Pinus",
		SampledAt: now.Add(-time.Hour), OperatorID: "operator-r", State: domain.StateBaselined,
		Revision: 1, CreatedAt: now, UpdatedAt: now,
		Cores: []domain.CoreSample{{CoreID: "core-r", BatchID: "batch-writer-rotation", TreeCode: "tree-r", RadiusCode: "A"}},
	}
	first := domain.AuditEvent{
		EventID: "event-rotation-1", BatchID: batch.BatchID, Revision: 1,
		RequestID: "request-rotation-1", Action: "CREATE_BATCH", ActorID: batch.OperatorID,
		OccurredAt: now, Payload: map[string]any{"state": batch.State},
	}
	if err := domain.FinalizeEvent(&first); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(repository.NewSnapshot(batch), first); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(root, "audit", batch.BatchID+".jsonl")
	rotatedPath := auditPath + ".1"
	committedAudit, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(auditPath, rotatedPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(auditPath, committedAudit, 0o640); err != nil {
		t.Fatal(err)
	}

	snap, err := store.Load(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	snap.Batch.Revision = 2
	snap.Batch.State = domain.StateImaged
	snap.Batch.UpdatedAt = now.Add(time.Minute)
	second := domain.AuditEvent{
		EventID: "event-rotation-2", BatchID: batch.BatchID, Revision: 2,
		RequestID: "request-rotation-2", Action: "REGISTER_IMAGES", ActorID: batch.OperatorID,
		OccurredAt: now.Add(time.Minute), PreviousDigest: snap.LastEvent,
		Payload: map[string]any{"state": snap.Batch.State},
	}
	if err := domain.FinalizeEvent(&second); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(snap, second); err != nil {
		t.Fatal(err)
	}

	if err := store.VerifyBatch(batch.BatchID); err != nil {
		t.Fatalf("审计日志轮转后提交必须写入当前路径并保持快照一致: %v", err)
	}
}
