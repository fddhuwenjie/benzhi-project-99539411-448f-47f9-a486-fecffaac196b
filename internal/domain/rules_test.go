package domain

import (
	"testing"
	"time"
)

func testBatch() *DendroBatch {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return &DendroBatch{BatchID: "batch-test", SiteCode: "SITE", Species: "Pinus", SampledAt: now.Add(-time.Hour), OperatorID: "operator-test", State: StateAnalyzed, Revision: 3, CreatedAt: now, Cores: []CoreSample{{CoreID: "core-a", BatchID: "batch-test", TreeCode: "tree-a", RadiusCode: "A"}, {CoreID: "core-b", BatchID: "batch-test", TreeCode: "tree-b", RadiusCode: "A"}}, Observations: []RingObservation{
		{ObservationID: "obs-a-1", CoreID: "core-a", RingIndex: 1, CalendarYear: 2000, WidthMicrons: 100, BoundaryPosition: 100, MarkerKind: MarkerNone},
		{ObservationID: "obs-a-2", CoreID: "core-a", RingIndex: 2, CalendarYear: 2001, WidthMicrons: 200, BoundaryPosition: 300, MarkerKind: MarkerNone, AnchorGroup: "anchor-x"},
		{ObservationID: "obs-a-3", CoreID: "core-a", RingIndex: 3, CalendarYear: 2002, WidthMicrons: 300, BoundaryPosition: 600, MarkerKind: MarkerNone},
		{ObservationID: "obs-b-1", CoreID: "core-b", RingIndex: 1, CalendarYear: 2000, WidthMicrons: 110, BoundaryPosition: 110, MarkerKind: MarkerNone},
		{ObservationID: "obs-b-2", CoreID: "core-b", RingIndex: 2, CalendarYear: 2001, WidthMicrons: 210, BoundaryPosition: 320, MarkerKind: MarkerNone, AnchorGroup: "anchor-x"},
		{ObservationID: "obs-b-3", CoreID: "core-b", RingIndex: 3, CalendarYear: 2002, WidthMicrons: 310, BoundaryPosition: 630, MarkerKind: MarkerNone},
	}}
}

func TestRunQualityRulesDeterministic(t *testing.T) {
	b := testBatch()
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	first := RunQualityRules(b, now)
	second := RunQualityRules(b, now)
	if !first.Passed || len(first.Findings) != 0 {
		t.Fatalf("valid batch did not pass: %+v", first.Findings)
	}
	d1, _ := Digest(first)
	d2, _ := Digest(second)
	if d1 != d2 {
		t.Fatalf("rule output not deterministic")
	}
	b.Observations[4].WidthMicrons = -1
	b.Observations[4].MarkerKind = MarkerMissing
	result := RunQualityRules(b, now)
	if result.Passed {
		t.Fatal("invalid batch unexpectedly passed")
	}
	codes := map[string]bool{}
	for _, f := range result.Findings {
		codes[f.RuleCode] = true
	}
	if !codes["WIDTH_RANGE"] || !codes["UNEXPLAINED_MARKER"] {
		t.Fatalf("missing expected findings: %v", codes)
	}
}

func TestBuildManifestStableAndVerifiable(t *testing.T) {
	b := testBatch()
	b.State = StateVerified
	b.ReviewerID = "reviewer-test"
	b.Review = &ReviewSeal{SealID: "seal-test", BatchID: b.BatchID, ReviewerID: b.ReviewerID, Decision: "APPROVE", VerifiedRevision: 4, SignedAt: b.CreatedAt}
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	m1, err := BuildManifest(b, "event-digest", now)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := BuildManifest(b, "event-digest", now)
	if err != nil {
		t.Fatal(err)
	}
	if m1.ManifestDigest != m2.ManifestDigest || !VerifyManifest(m1) {
		t.Fatalf("manifest is not stable or verifiable")
	}
	m1.Species = "changed"
	if VerifyManifest(m1) {
		t.Fatal("tampered manifest passed verification")
	}
}
