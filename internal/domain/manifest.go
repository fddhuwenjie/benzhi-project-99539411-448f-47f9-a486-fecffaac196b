package domain

import (
	"sort"
	"time"
)

func BuildManifest(batch *DendroBatch, eventDigest string, now time.Time) (Manifest, error) {
	if batch.State != StateVerified {
		return Manifest{}, NewError(ErrState, "state", "仅 VERIFIED 批次可以封存")
	}
	if batch.Review == nil || batch.Review.Decision != "APPROVE" {
		return Manifest{}, NewError(ErrState, "review", "缺少有效的独立复核签署")
	}
	byCore := ObservationsByCore(batch.Observations)
	cores := append([]CoreSample(nil), batch.Cores...)
	sort.Slice(cores, func(i, j int) bool { return cores[i].CoreID < cores[j].CoreID })
	m := Manifest{SchemaVersion: "dendro-manifest-v1", BatchID: batch.BatchID, SiteCode: batch.SiteCode, Species: batch.Species, SampledAt: batch.SampledAt, OperatorID: batch.OperatorID, ReviewerID: batch.ReviewerID, SealedRevision: batch.Revision + 1, SealedAt: now, EventChainDigest: eventDigest}
	for _, c := range cores {
		mc := ManifestCore{CoreID: c.CoreID, TreeCode: c.TreeCode, RadiusCode: c.RadiusCode, ImageDigest: c.ImageDigest, MicronsPerPixel: c.MicronsPerPixel}
		for _, o := range byCore[c.CoreID] {
			mc.Sequence = append(mc.Sequence, ManifestObservation{RingIndex: o.RingIndex, CalendarYear: o.CalendarYear, WidthMicrons: o.WidthMicrons, BoundaryPosition: o.BoundaryPosition, MarkerKind: o.MarkerKind, AnchorGroup: o.AnchorGroup})
		}
		m.Cores = append(m.Cores, mc)
	}
	evidence := struct {
		Cores    []ManifestCore   `json:"cores"`
		Review   *ReviewSeal      `json:"review"`
		Findings []QualityFinding `json:"findings"`
	}{m.Cores, batch.Review, batch.Findings}
	d, err := Digest(evidence)
	if err != nil {
		return Manifest{}, err
	}
	m.EvidenceDigest = d
	copy := m
	copy.ManifestDigest = ""
	d, err = Digest(copy)
	if err != nil {
		return Manifest{}, err
	}
	m.ManifestDigest = d
	return m, nil
}

func VerifyManifest(m Manifest) bool {
	expected := m.ManifestDigest
	m.ManifestDigest = ""
	d, err := Digest(m)
	return err == nil && d == expected
}
