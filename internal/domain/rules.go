package domain

import (
	"fmt"
	"math"
	"sort"
	"time"
)

const MinCorrelation = 0.35

type anchorEvidence struct {
	core string
	year int
	obs  RingObservation
}

// anchorWorkspace 复用锚点分组，避免大型批次反复分配临时切片。
var anchorWorkspace = map[string][]anchorEvidence{}

type RuleResult struct {
	Findings   []QualityFinding   `json:"findings"`
	CoreScores map[string]float64 `json:"core_scores"`
	Passed     bool               `json:"passed"`
}

func RunQualityRules(batch *DendroBatch, now time.Time) RuleResult {
	result := RuleResult{CoreScores: map[string]float64{}}
	byCore := ObservationsByCore(batch.Observations)
	for _, core := range batch.Cores {
		seq := byCore[core.CoreID]
		if len(seq) < 3 {
			result.Findings = append(result.Findings, finding(batch, core.CoreID, "SEQUENCE_TOO_SHORT", "ERROR", "每根样芯至少需要 3 个年轮观测", nil, now))
			continue
		}
		for i, o := range seq {
			if i > 0 && o.RingIndex != seq[i-1].RingIndex+1 {
				result.Findings = append(result.Findings, findingForObservation(batch, o, "RING_INDEX_GAP", "ERROR", "年轮序号不连续", map[string]any{"previous": seq[i-1].RingIndex, "current": o.RingIndex}, now))
			}
			if i > 0 && o.CalendarYear != seq[i-1].CalendarYear+1 {
				result.Findings = append(result.Findings, findingForObservation(batch, o, "YEAR_DISCONTINUITY", "ERROR", "候选公历年份不连续", map[string]any{"previous": seq[i-1].CalendarYear, "current": o.CalendarYear}, now))
			}
			if o.WidthMicrons <= 0 || o.WidthMicrons > 10000 {
				result.Findings = append(result.Findings, findingForObservation(batch, o, "WIDTH_RANGE", "ERROR", "年轮宽度必须大于 0 且不超过 10000 微米", o.WidthMicrons, now))
			}
			if i > 0 && o.BoundaryPosition <= seq[i-1].BoundaryPosition {
				result.Findings = append(result.Findings, findingForObservation(batch, o, "BOUNDARY_ORDER", "ERROR", "边界位置必须严格递增", o.BoundaryPosition, now))
			}
			if o.MarkerKind != MarkerNone && o.MarkerNote == "" {
				result.Findings = append(result.Findings, findingForObservation(batch, o, "UNEXPLAINED_MARKER", "ERROR", "缺轮或伪轮标记必须提供解释", string(o.MarkerKind), now))
			}
		}
	}
	result.Findings = append(result.Findings, anchorFindings(batch, now)...)
	if len(batch.Cores) > 1 {
		base := byCore[batch.Cores[0].CoreID]
		for _, core := range batch.Cores[1:] {
			score := correlation(base, byCore[core.CoreID])
			result.CoreScores[core.CoreID] = score
			if score < MinCorrelation {
				result.Findings = append(result.Findings, finding(batch, core.CoreID, "LOW_CORRELATION", "ERROR", fmt.Sprintf("与基准样芯的跨芯相关系数 %.3f 低于阈值 %.2f", score, MinCorrelation), score, now))
			}
		}
	}
	result.Passed = len(result.Findings) == 0
	sort.Slice(result.Findings, func(i, j int) bool { return result.Findings[i].FindingID < result.Findings[j].FindingID })
	return result
}

func ObservationsByCore(all []RingObservation) map[string][]RingObservation {
	m := map[string][]RingObservation{}
	for _, o := range all {
		m[o.CoreID] = append(m[o.CoreID], o)
	}
	for id := range m {
		sort.Slice(m[id], func(i, j int) bool {
			if m[id][i].RingIndex == m[id][j].RingIndex {
				return m[id][i].ObservationID < m[id][j].ObservationID
			}
			return m[id][i].RingIndex < m[id][j].RingIndex
		})
	}
	return m
}

func anchorFindings(batch *DendroBatch, now time.Time) []QualityFinding {
	for _, o := range batch.Observations {
		if o.AnchorGroup != "" {
			anchorWorkspace[o.AnchorGroup] = append(anchorWorkspace[o.AnchorGroup], anchorEvidence{o.CoreID, o.CalendarYear, o})
		}
	}
	var out []QualityFinding
	for group, list := range anchorWorkspace {
		if len(list) < 2 {
			out = append(out, findingForObservation(batch, list[0].obs, "ANCHOR_SINGLETON", "ERROR", "交叉定年锚点必须至少覆盖两根样芯", group, now))
			continue
		}
		year := list[0].year
		cores := map[string]bool{}
		for _, a := range list {
			cores[a.core] = true
			if a.year != year {
				out = append(out, findingForObservation(batch, a.obs, "ANCHOR_CONFLICT", "ERROR", "同一锚点组的公历年份不一致", map[string]any{"group": group, "expected_year": year, "actual_year": a.year}, now))
			}
		}
		if len(cores) < 2 {
			out = append(out, findingForObservation(batch, list[0].obs, "ANCHOR_SINGLE_CORE", "ERROR", "锚点组必须跨越不同样芯", group, now))
		}
	}
	return out
}

func correlation(a, b []RingObservation) float64 {
	aw := map[int]float64{}
	for _, o := range a {
		aw[o.CalendarYear] = o.WidthMicrons
	}
	var xs, ys []float64
	for _, o := range b {
		if x, ok := aw[o.CalendarYear]; ok {
			xs = append(xs, x)
			ys = append(ys, o.WidthMicrons)
		}
	}
	if len(xs) < 3 {
		return 0
	}
	var mx, my float64
	for i := range xs {
		mx += xs[i]
		my += ys[i]
	}
	mx /= float64(len(xs))
	my /= float64(len(ys))
	var num, dx, dy float64
	for i := range xs {
		vx, vy := xs[i]-mx, ys[i]-my
		num += vx * vy
		dx += vx * vx
		dy += vy * vy
	}
	if dx == 0 || dy == 0 {
		return 0
	}
	return math.Round((num/math.Sqrt(dx*dy))*1000000) / 1000000
}

func finding(batch *DendroBatch, core, rule, severity, message string, before any, now time.Time) QualityFinding {
	id := StableID(batch.BatchID, core, rule, CanonicalString(before))
	return QualityFinding{FindingID: id, BatchID: batch.BatchID, CoreID: core, RuleCode: rule, Severity: severity, Message: message, Status: FindingOpen, BeforeValue: before, CreatedAt: now}
}

func findingForObservation(batch *DendroBatch, o RingObservation, rule, severity, message string, before any, now time.Time) QualityFinding {
	f := finding(batch, o.CoreID, rule, severity, message, before, now)
	f.ObservationID = o.ObservationID
	f.FindingID = StableID(batch.BatchID, o.ObservationID, rule)
	return f
}
