package anchorworkspacecontamination_test

import (
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
)

func batchWithAnchor(batchID string, firstYear, secondAnchorYear int) *domain.DendroBatch {
	cores := []domain.CoreSample{
		{CoreID: batchID + "-core-a", BatchID: batchID, TreeCode: "tree-a", RadiusCode: "A"},
		{CoreID: batchID + "-core-b", BatchID: batchID, TreeCode: "tree-b", RadiusCode: "A"},
	}
	observations := make([]domain.RingObservation, 0, 6)
	for coreIndex, core := range cores {
		for ringIndex, width := range []float64{100, 200, 300} {
			year := firstYear + ringIndex
			if coreIndex == 1 && ringIndex == 1 {
				year = secondAnchorYear
			}
			anchor := ""
			if ringIndex == 1 {
				anchor = "shared-anchor"
			}
			observations = append(observations, domain.RingObservation{
				ObservationID: core.CoreID + "-obs-" + string(rune('1'+ringIndex)),
				CoreID:        core.CoreID, RingIndex: ringIndex + 1, CalendarYear: year,
				WidthMicrons: width, BoundaryPosition: float64((ringIndex + 1) * 100),
				MarkerKind: domain.MarkerNone, AnchorGroup: anchor,
			})
		}
	}
	return &domain.DendroBatch{BatchID: batchID, Cores: cores, Observations: observations}
}

func TestAnchorWorkspaceDoesNotContaminateLaterBatch(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	contaminating := batchWithAnchor("contaminating", 2000, 2002)
	first := domain.RunQualityRules(contaminating, now)
	if first.Passed {
		t.Fatal("前置批次应产生锚点年份冲突")
	}

	healthy := batchWithAnchor("healthy-batch", 2010, 2011)
	second := domain.RunQualityRules(healthy, now)
	for _, finding := range second.Findings {
		if finding.RuleCode == "ANCHOR_CONFLICT" {
			t.Fatalf("健康批次复用锚点名时继承了前一批次状态：%s", finding.Message)
		}
	}
	if !second.Passed {
		t.Fatalf("健康批次应独立通过规则，实际异常：%+v", second.Findings)
	}
}
