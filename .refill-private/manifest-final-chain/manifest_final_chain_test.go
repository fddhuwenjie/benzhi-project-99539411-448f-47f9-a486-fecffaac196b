package bugtest

import (
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func TestManifestReferencesSealedRevisionAuditHead(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	batch := &domain.DendroBatch{
		BatchID:    "batch-manifest-chain",
		SiteCode:   "SITE-M",
		Species:    "Pinus",
		SampledAt:  now.Add(-time.Hour),
		OperatorID: "operator-manifest",
		ReviewerID: "reviewer-manifest",
		State:      domain.StateVerified,
		Revision:   1,
		CreatedAt:  now,
		UpdatedAt:  now,
		Cores:      []domain.CoreSample{{CoreID: "core-manifest", BatchID: "batch-manifest-chain", TreeCode: "tree-m", RadiusCode: "A"}},
		Review: &domain.ReviewSeal{
			SealID: "review-seal", BatchID: "batch-manifest-chain", ReviewerID: "reviewer-manifest",
			Decision: "APPROVE", VerifiedRevision: 1, SignedAt: now,
		},
	}
	event := domain.AuditEvent{
		EventID: "event-review", BatchID: batch.BatchID, Revision: 1, RequestID: "request-review",
		Action: "REVIEW_BATCH", ActorID: batch.ReviewerID, OccurredAt: now,
	}
	if err := domain.FinalizeEvent(&event); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(repository.NewSnapshot(batch), event); err != nil {
		t.Fatal(err)
	}

	service := workflow.New(store)
	result, err := service.Seal(batch.BatchID, workflow.SealCommand{CommandMeta: workflow.CommandMeta{
		RequestID: "request-seal", ExpectedRevision: 1, ActorID: batch.OperatorID,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if result.StatusCode != 200 {
		t.Fatalf("unexpected seal status %d", result.StatusCode)
	}
	manifest, err := service.Manifest(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	events, err := service.Events(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	sealedHead := events[len(events)-1].Digest
	if manifest.SealedRevision != events[len(events)-1].Revision {
		t.Fatalf("manifest revision and audit revision differ: %d vs %d", manifest.SealedRevision, events[len(events)-1].Revision)
	}
	if manifest.EventChainDigest != sealedHead {
		t.Fatalf("manifest points to pre-seal audit head %s, sealed head is %s", manifest.EventChainDigest, sealedHead)
	}
}
