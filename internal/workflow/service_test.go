package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

func resultBatch(t *testing.T, result Result, err error) *domain.DendroBatch {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	var response CommandResponse
	if err := json.Unmarshal(result.Body, &response); err != nil {
		t.Fatal(err)
	}
	return response.Batch
}

func TestServiceWorkflowConflictSeparationAndSeal(t *testing.T) {
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := New(store)
	now := time.Now().UTC()
	result, callErr := service.Create(CreateBatchCommand{CommandMeta: CommandMeta{RequestID: "req-create", ExpectedRevision: 0, ActorID: "operator-1"}, BatchID: "batch-flow", SiteCode: "SITE", Species: "Pinus", SampledAt: now.Add(-time.Hour), OperatorID: "operator-1", Cores: []domain.CoreSample{{CoreID: "core-one", TreeCode: "tree-1", RadiusCode: "A"}, {CoreID: "core-two", TreeCode: "tree-2", RadiusCode: "A"}}})
	batch := resultBatch(t, result, callErr)
	images := []ImageInput{{CoreID: "core-one", PreparationMethod: "sand", ImageDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MicronsPerPixel: 2, CapturedAt: now}, {CoreID: "core-two", PreparationMethod: "sand", ImageDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", MicronsPerPixel: 2, CapturedAt: now}}
	result, callErr = service.RegisterImages(batch.BatchID, RegisterImagesCommand{CommandMeta: CommandMeta{RequestID: "req-images", ExpectedRevision: batch.Revision, ActorID: "operator-1"}, Images: images})
	batch = resultBatch(t, result, callErr)
	var obs []domain.RingObservation
	for ci, core := range []string{"core-one", "core-two"} {
		for i, w := range []float64{100, 200, 300} {
			if ci == 1 {
				w += 10
			}
			obs = append(obs, domain.RingObservation{ObservationID: domain.StableID(core, string(rune('0'+i))), CoreID: core, RingIndex: i + 1, CalendarYear: 2000 + i, WidthMicrons: w, BoundaryPosition: float64((i + 1) * 300), MarkerKind: domain.MarkerNone, AnchorGroup: map[bool]string{true: "anchor-2001"}[i == 1]})
		}
	}
	result, callErr = service.SubmitObservations(batch.BatchID, SubmitObservationsCommand{CommandMeta: CommandMeta{RequestID: "req-obs", ExpectedRevision: batch.Revision, ActorID: "operator-1"}, Observations: obs})
	batch = resultBatch(t, result, callErr)
	result, callErr = service.Validate(batch.BatchID, ValidateCommand{CommandMeta: CommandMeta{RequestID: "req-rules", ExpectedRevision: batch.Revision, ActorID: "operator-1"}})
	batch = resultBatch(t, result, callErr)
	if batch.State != domain.StateReviewReady {
		t.Fatalf("expected review ready, got %s", batch.State)
	}
	_, err = service.Review(batch.BatchID, ReviewCommand{CommandMeta: CommandMeta{RequestID: "req-bad-review", ExpectedRevision: batch.Revision, ActorID: "operator-1"}, ReviewerID: "operator-1", Decision: "APPROVE", Note: "cannot self review"})
	if !domain.IsCode(err, domain.ErrForbidden) {
		t.Fatalf("expected separation error: %v", err)
	}
	result, callErr = service.Review(batch.BatchID, ReviewCommand{CommandMeta: CommandMeta{RequestID: "req-review", ExpectedRevision: batch.Revision, ActorID: "reviewer-2"}, ReviewerID: "reviewer-2", Decision: "APPROVE", Note: "evidence independently checked"})
	batch = resultBatch(t, result, callErr)
	result, callErr = service.Seal(batch.BatchID, SealCommand{CommandMeta: CommandMeta{RequestID: "req-seal", ExpectedRevision: batch.Revision, ActorID: "operator-1"}})
	batch = resultBatch(t, result, callErr)
	if batch.State != domain.StateSealed {
		t.Fatal("batch not sealed")
	}
	manifest, err := service.Manifest(batch.BatchID)
	if err != nil || !domain.VerifyManifest(*manifest) {
		t.Fatalf("invalid manifest: %v", err)
	}
	_, err = service.Validate(batch.BatchID, ValidateCommand{CommandMeta: CommandMeta{RequestID: "req-after-seal", ExpectedRevision: batch.Revision, ActorID: "operator-1"}})
	if !domain.IsCode(err, domain.ErrState) {
		t.Fatalf("expected immutable sealed batch: %v", err)
	}
}

func TestCreateIdempotencyPrecedesRevisionCheck(t *testing.T) {
	store, _ := repository.Open(t.TempDir())
	service := New(store)
	cmd := CreateBatchCommand{CommandMeta: CommandMeta{RequestID: "same-request", ExpectedRevision: 0, ActorID: "operator-z"}, BatchID: "batch-idem", SiteCode: "S", Species: "P", SampledAt: time.Now().Add(-time.Hour), OperatorID: "operator-z", Cores: []domain.CoreSample{{CoreID: "core-z", TreeCode: "tree-z", RadiusCode: "A"}}}
	first, err := service.Create(cmd)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Replayed || string(first.Body) != string(second.Body) {
		t.Fatal("idempotent create did not replay original response")
	}
}
