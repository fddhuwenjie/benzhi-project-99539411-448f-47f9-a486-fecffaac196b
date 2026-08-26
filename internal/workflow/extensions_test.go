package workflow

import (
	"encoding/json"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

func correctionReadyBatch(t *testing.T) (*Service, *repository.Store, *domain.DendroBatch) {
	t.Helper()
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := New(store)
	now := time.Now().UTC()
	result, err := service.Create(CreateBatchCommand{CommandMeta: CommandMeta{RequestID: "ext-create", ActorID: "operator-ext"}, BatchID: "batch-ext", SiteCode: "SITE-X", Species: "Pinus", SampledAt: now.Add(-time.Hour), OperatorID: "operator-ext", Cores: []domain.CoreSample{{CoreID: "core-ext", TreeCode: "tree-ext", RadiusCode: "A"}}})
	batch := resultBatch(t, result, err)
	result, err = service.RegisterImages(batch.BatchID, RegisterImagesCommand{CommandMeta: CommandMeta{RequestID: "ext-images", ExpectedRevision: batch.Revision, ActorID: batch.OperatorID}, Images: []ImageInput{{CoreID: "core-ext", PreparationMethod: "精磨", ImageDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", MicronsPerPixel: 2, CapturedAt: now}}})
	batch = resultBatch(t, result, err)
	observations := []domain.RingObservation{
		{ObservationID: "obs-ext-1", CoreID: "core-ext", RingIndex: 1, CalendarYear: 2000, WidthMicrons: 100, BoundaryPosition: 100, MarkerKind: domain.MarkerNone},
		{ObservationID: "obs-ext-2", CoreID: "core-ext", RingIndex: 2, CalendarYear: 2001, WidthMicrons: -2, BoundaryPosition: 300, MarkerKind: domain.MarkerMissing},
		{ObservationID: "obs-ext-3", CoreID: "core-ext", RingIndex: 3, CalendarYear: 2002, WidthMicrons: 300, BoundaryPosition: 600, MarkerKind: domain.MarkerNone},
	}
	result, err = service.SubmitObservations(batch.BatchID, SubmitObservationsCommand{CommandMeta: CommandMeta{RequestID: "ext-observations", ExpectedRevision: batch.Revision, ActorID: batch.OperatorID}, Observations: observations})
	batch = resultBatch(t, result, err)
	result, err = service.Validate(batch.BatchID, ValidateCommand{CommandMeta: CommandMeta{RequestID: "ext-rules", ExpectedRevision: batch.Revision, ActorID: batch.OperatorID}})
	batch = resultBatch(t, result, err)
	if batch.State != domain.StateCorrectionRequired {
		t.Fatalf("expected correction required, got %s", batch.State)
	}
	return service, store, batch
}

func TestBulkCorrectionAtomicReplayAndInspection(t *testing.T) {
	service, store, batch := correctionReadyBatch(t)
	findings := map[string]string{}
	for _, finding := range batch.Findings {
		findings[finding.RuleCode] = finding.FindingID
	}
	bad := CorrectFindingsCommand{CommandMeta: CommandMeta{RequestID: "ext-bad-bulk", ExpectedRevision: batch.Revision, ActorID: batch.OperatorID}, Items: []CorrectionItem{{FindingID: findings["WIDTH_RANGE"], Reason: "重新测量宽度", Replacement: Replacement{WidthMicrons: floatPointer(220)}}, {FindingID: "not-in-batch", Reason: "无效异常引用", Replacement: Replacement{MarkerNote: stringPointer("现场记录")}}}}
	if _, err := service.CorrectFindings(batch.BatchID, bad); !domain.IsCode(err, domain.ErrNotFound) {
		t.Fatalf("expected target error, got %v", err)
	}
	unchanged, _ := service.Get(batch.BatchID)
	events, _ := store.Events(batch.BatchID)
	if unchanged.Revision != batch.Revision || len(events) != int(batch.Revision) || unchanged.Observations[1].WidthMicrons != -2 {
		t.Fatal("failed bulk correction changed persisted aggregate")
	}

	command := CorrectFindingsCommand{CommandMeta: CommandMeta{RequestID: "ext-good-bulk", ExpectedRevision: batch.Revision, ActorID: batch.OperatorID}, Items: []CorrectionItem{{FindingID: findings["WIDTH_RANGE"], Reason: "重新测量并核对宽度", Replacement: Replacement{WidthMicrons: floatPointer(220)}}, {FindingID: findings["UNEXPLAINED_MARKER"], Reason: "补录现场缺轮解释", Replacement: Replacement{MarkerNote: stringPointer("现场确认缺轮")}}}}
	first, err := service.CorrectFindings(batch.BatchID, command)
	corrected := resultBatch(t, first, err)
	if corrected.Revision != batch.Revision+1 || corrected.State != domain.StateReviewReady {
		t.Fatalf("unexpected corrected batch revision/state: r%d %s", corrected.Revision, corrected.State)
	}
	second, err := service.CorrectFindings(batch.BatchID, command)
	if err != nil || !second.Replayed || string(first.Body) != string(second.Body) {
		t.Fatalf("bulk correction was not replayed: %v", err)
	}
	events, _ = store.Events(batch.BatchID)
	if len(events) != int(corrected.Revision) {
		t.Fatalf("replay appended an audit event: %d", len(events))
	}
	var response CommandResponse
	if err := json.Unmarshal(first.Body, &response); err != nil || len(response.Corrections) != 2 {
		t.Fatalf("missing per-item results: %v", err)
	}
	report, err := service.ReviewInspection(batch.BatchID, "reviewer-ext")
	if err != nil || !report.Signable || report.InspectedRevision != corrected.Revision || len(report.Differences) != 2 {
		t.Fatalf("unexpected review inspection: %+v %v", report, err)
	}
	for _, difference := range report.Differences {
		if difference.AuditRevision != corrected.Revision || difference.SupersedesID == "" {
			t.Fatalf("difference is not traceable: %+v", difference)
		}
	}
}

func floatPointer(value float64) *float64 { return &value }
func stringPointer(value string) *string  { return &value }
