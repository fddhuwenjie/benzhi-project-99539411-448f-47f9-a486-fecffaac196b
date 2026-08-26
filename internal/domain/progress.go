package domain

import (
	"fmt"
	"sort"
	"time"
)

// BatchProgress 是从已验证批次快照派生的只读进度，不参与持久化摘要。
type BatchProgress struct {
	ImageCovered       int        `json:"image_covered"`
	ImageTotal         int        `json:"image_total"`
	MeasurementCovered int        `json:"measurement_covered"`
	MeasurementTotal   int        `json:"measurement_total"`
	OpenFindings       int        `json:"open_findings"`
	LastRuleRunAt      *time.Time `json:"last_rule_run_at,omitempty"`
	ReadyForNextStage  bool       `json:"ready_for_next_stage"`
	BlockingReasons    []string   `json:"blocking_reasons"`
}

type BatchStatistics struct {
	Total             int           `json:"total"`
	StatusCounts      map[State]int `json:"status_counts"`
	OpenFindings      int           `json:"open_findings"`
	ReadyForNextStage int           `json:"ready_for_next_stage"`
}

func Progress(batch *DendroBatch) BatchProgress {
	p := BatchProgress{ImageTotal: len(batch.Cores), MeasurementTotal: len(batch.Cores), BlockingReasons: []string{}}
	if batch.LastRuleRunAt != nil {
		runAt := *batch.LastRuleRunAt
		p.LastRuleRunAt = &runAt
	}
	byCore := ObservationsByCore(batch.Observations)
	for _, core := range batch.Cores {
		if core.ImageDigest != "" && core.PreparationMethod != "" && core.MicronsPerPixel > 0 && core.CapturedAt != nil {
			p.ImageCovered++
		}
		if len(byCore[core.CoreID]) > 0 {
			p.MeasurementCovered++
		}
	}
	for _, finding := range batch.Findings {
		if finding.Status == FindingOpen {
			p.OpenFindings++
		}
	}
	if p.ImageCovered < p.ImageTotal {
		p.BlockingReasons = append(p.BlockingReasons, fmt.Sprintf("影像覆盖不足（%d/%d）", p.ImageCovered, p.ImageTotal))
	}
	if batch.State != StateBaselined && p.MeasurementCovered < p.MeasurementTotal {
		p.BlockingReasons = append(p.BlockingReasons, fmt.Sprintf("样芯测量覆盖不足（%d/%d）", p.MeasurementCovered, p.MeasurementTotal))
	}
	if p.OpenFindings > 0 {
		p.BlockingReasons = append(p.BlockingReasons, fmt.Sprintf("尚有 %d 项异常未关闭", p.OpenFindings))
	}
	if (batch.State == StateAnalyzed || batch.State == StateCorrectionRequired || batch.State == StateReviewReady) && batch.LastRuleRunAt == nil {
		p.BlockingReasons = append(p.BlockingReasons, "尚未保存质量规则运行结果")
	}
	if batch.State == StateSealed {
		p.BlockingReasons = append(p.BlockingReasons, "批次已封存")
	}
	p.ReadyForNextStage = len(p.BlockingReasons) == 0
	return p
}

func SummarizeBatches(batches []*DendroBatch) BatchStatistics {
	stats := BatchStatistics{StatusCounts: map[State]int{
		StateDraft: 0, StateBaselined: 0, StateImaged: 0, StateAnalyzed: 0,
		StateCorrectionRequired: 0, StateReviewReady: 0, StateVerified: 0, StateSealed: 0,
	}}
	for _, batch := range batches {
		progress := Progress(batch)
		stats.Total++
		stats.StatusCounts[batch.State]++
		stats.OpenFindings += progress.OpenFindings
		if progress.ReadyForNextStage {
			stats.ReadyForNextStage++
		}
	}
	return stats
}

func SortBatchesForOverview(batches []*DendroBatch) {
	sort.SliceStable(batches, func(i, j int) bool {
		if batches[i].UpdatedAt.Equal(batches[j].UpdatedAt) {
			return batches[i].BatchID < batches[j].BatchID
		}
		return batches[i].UpdatedAt.After(batches[j].UpdatedAt)
	})
}
