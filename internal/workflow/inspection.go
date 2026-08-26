package workflow

import (
	"fmt"
	"sort"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

type EvidenceCheck struct {
	Code    string `json:"code"`
	Label   string `json:"label"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

type EvidenceInspection struct {
	InspectedRevision int64                       `json:"inspected_revision"`
	Checks            []EvidenceCheck             `json:"checks"`
	Differences       []domain.RevisionDifference `json:"differences"`
	RecentEvents      []AuditEventSummary         `json:"recent_events"`
	Audit             AuditEvidenceSummary        `json:"audit"`
	Passed            bool                        `json:"passed"`
	Signable          bool                        `json:"signable"`
}

type AuditEvidenceSummary struct {
	LatestRevision     int64  `json:"latest_revision"`
	RevisionContinuous bool   `json:"revision_continuous"`
	SnapshotConsistent bool   `json:"snapshot_consistent"`
	Details            string `json:"details"`
}

type AuditEventSummary struct {
	Revision   int64     `json:"revision"`
	Action     string    `json:"action"`
	ActorID    string    `json:"actor_id"`
	Reason     string    `json:"reason,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	Digest     string    `json:"digest"`
}

func InspectEvidence(batch *domain.DendroBatch, reviewerID string) EvidenceInspection {
	inspection := EvidenceInspection{InspectedRevision: batch.Revision, Differences: []domain.RevisionDifference{}, RecentEvents: []AuditEventSummary{}}
	add := func(code, label string, passed bool, details string) {
		inspection.Checks = append(inspection.Checks, EvidenceCheck{Code: code, Label: label, Passed: passed, Details: details})
	}
	add("BASELINE", "采样基线", batch.BatchID != "" && batch.SiteCode != "" && batch.Species != "" && !batch.SampledAt.IsZero() && len(batch.Cores) > 0, fmt.Sprintf("批次包含 %d 根基线样芯", len(batch.Cores)))
	imageCount := 0
	for _, core := range batch.Cores {
		if core.ImageDigest != "" && core.PreparationMethod != "" && core.MicronsPerPixel > 0 && core.CapturedAt != nil {
			imageCount++
		}
	}
	add("IMAGES", "制备影像证据", imageCount == len(batch.Cores), fmt.Sprintf("%d/%d 根样芯具有完整影像摘要", imageCount, len(batch.Cores)))
	byCore := domain.ObservationsByCore(batch.Observations)
	measured := 0
	for _, core := range batch.Cores {
		if len(byCore[core.CoreID]) > 0 {
			measured++
		}
	}
	add("MEASUREMENTS", "年轮测量覆盖", measured == len(batch.Cores), fmt.Sprintf("%d/%d 根样芯具有有效序列", measured, len(batch.Cores)))
	open, resolved := 0, 0
	for _, finding := range batch.Findings {
		if finding.Status == domain.FindingOpen {
			open++
		} else if finding.Status == domain.FindingResolved {
			resolved++
		}
	}
	add("FINDINGS", "异常闭环", open == 0, fmt.Sprintf("待处理 %d 项，已保留整改轨迹 %d 项", open, resolved))
	add("RULE_RUN", "确定性规则结果", batch.LastRuleRunAt != nil, func() string {
		if batch.LastRuleRunAt == nil {
			return "尚未运行规则"
		}
		return "已保存最近规则运行时间"
	}())
	separated := reviewerID != "" && reviewerID != batch.OperatorID
	add("PERSONNEL_SEPARATION", "人员分离", separated, func() string {
		if separated {
			return "复核员与测量员不同"
		}
		return "复核员不得与测量员相同"
	}())
	sort.SliceStable(inspection.Checks, func(i, j int) bool { return inspection.Checks[i].Code < inspection.Checks[j].Code })
	inspection.Passed = true
	for _, check := range inspection.Checks {
		if !check.Passed {
			inspection.Passed = false
			break
		}
	}
	return inspection
}

func (s *Service) ReviewInspection(batchID, reviewerID string) (EvidenceInspection, error) {
	if err := domain.ValidateID("reviewer_id", reviewerID); err != nil {
		return EvidenceInspection{}, err
	}
	unlock := s.locks.lock(batchID)
	defer unlock()
	snap, audit, err := s.store.ReadEvidence(batchID)
	if err != nil {
		return EvidenceInspection{}, translateRepo(err)
	}
	if err := domain.EnsureState(snap.Batch, domain.StateReviewReady); err != nil {
		return EvidenceInspection{}, err
	}
	cacheKey := fmt.Sprintf("%s:%d", batchID, snap.Batch.Revision)
	if cached, ok := s.inspectionCache[cacheKey]; ok {
		return cached, nil
	}
	inspection := assembleInspection(snap.Batch, reviewerID, audit)
	s.inspectionCache[cacheKey] = inspection
	return inspection, nil
}

func assembleInspection(batch *domain.DendroBatch, reviewerID string, audit repository.AuditInspection) EvidenceInspection {
	inspection := InspectEvidence(batch, reviewerID)
	inspection.Audit = AuditEvidenceSummary{RevisionContinuous: audit.RevisionContinuous, SnapshotConsistent: audit.SnapshotConsistent, Details: audit.Details}
	if len(audit.Events) > 0 {
		inspection.Audit.LatestRevision = audit.Events[len(audit.Events)-1].Revision
	}
	differences, problems := domain.RevisionDifferences(batch)
	for i := range differences {
		for _, event := range audit.Events {
			if event.OccurredAt.Equal(differences[i].ResolvedAt) && (event.Action == "CORRECT_FINDING" || event.Action == "CORRECT_FINDINGS") {
				differences[i].AuditRevision = event.Revision
				break
			}
		}
		if differences[i].AuditRevision == 0 {
			problems = append(problems, fmt.Sprintf("异常 %s 找不到对应整改审计 revision", differences[i].FindingID))
		}
	}
	inspection.Differences = differences
	inspection.Checks = append(inspection.Checks, EvidenceCheck{Code: "AUDIT_CHAIN", Label: "审计事件链", Passed: audit.Valid, Details: audit.Details})
	referencesValid := len(problems) == 0
	details := "全部替换观测、异常前后值与审计 revision 可追溯"
	if !referencesValid {
		sort.Strings(problems)
		details = problems[0]
		if len(problems) > 1 {
			details += fmt.Sprintf("；另有 %d 项引用问题", len(problems)-1)
		}
	}
	inspection.Checks = append(inspection.Checks, EvidenceCheck{Code: "REVISION_REFERENCES", Label: "修订差异引用", Passed: referencesValid, Details: details})
	start := 0
	if len(audit.Events) > 5 {
		start = len(audit.Events) - 5
	}
	for _, event := range audit.Events[start:] {
		inspection.RecentEvents = append(inspection.RecentEvents, AuditEventSummary{Revision: event.Revision, Action: event.Action, ActorID: event.ActorID, Reason: event.Reason, OccurredAt: event.OccurredAt, Digest: event.Digest})
	}
	sort.SliceStable(inspection.Checks, func(i, j int) bool { return inspection.Checks[i].Code < inspection.Checks[j].Code })
	inspection.Passed = true
	for _, check := range inspection.Checks {
		if !check.Passed {
			inspection.Passed = false
			break
		}
	}
	inspection.Signable = inspection.Passed
	return inspection
}

func requireOperator(batch *domain.DendroBatch, actorID string) error {
	if actorID != batch.OperatorID {
		return domain.NewError(domain.ErrForbidden, "actor_id", "仅批次测量员 %s 可以执行此操作", batch.OperatorID)
	}
	return nil
}
