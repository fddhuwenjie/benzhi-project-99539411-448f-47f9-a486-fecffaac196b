package workflow

import (
	"fmt"
	"strings"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

const MaxCorrectionItems = 50

func (s *Service) CorrectFinding(batchID string, cmd CorrectFindingCommand) (Result, error) {
	bulk := CorrectFindingsCommand{CommandMeta: cmd.CommandMeta, Items: []CorrectionItem{{FindingID: cmd.FindingID, Reason: cmd.Reason, Replacement: cmd.Replacement}}}
	return s.correctFindings(batchID, "CORRECT_FINDING", bulk)
}

func (s *Service) CorrectFindings(batchID string, cmd CorrectFindingsCommand) (Result, error) {
	return s.correctFindings(batchID, "CORRECT_FINDINGS", cmd)
}

func (s *Service) correctFindings(batchID, action string, cmd CorrectFindingsCommand) (Result, error) {
	digest, err := commandDigest(action, cmd)
	if err != nil {
		return Result{}, err
	}
	return s.mutate(batchID, action, cmd.CommandMeta, digest, 200, func(batch *domain.DendroBatch, _ *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := requireOperator(batch, cmd.ActorID); err != nil {
			return nil, "", "", err
		}
		if err := domain.EnsureState(batch, domain.StateCorrectionRequired); err != nil {
			return nil, "", "", err
		}
		if len(cmd.Items) == 0 || len(cmd.Items) > MaxCorrectionItems {
			return nil, "", "", domain.NewError(domain.ErrValidation, "items", "批量整改条目数必须为 1 至 %d", MaxCorrectionItems)
		}
		findingIndex := make(map[string]int, len(batch.Findings))
		for i := range batch.Findings {
			findingIndex[batch.Findings[i].FindingID] = i
		}
		seen := map[string]bool{}
		observationAlias := map[string]int{}
		touchedObservations := map[int]int{}
		results := make([]CorrectionItemResult, 0, len(cmd.Items))
		for itemIndex, item := range cmd.Items {
			prefix := fmt.Sprintf("items[%d]", itemIndex)
			if seen[item.FindingID] {
				return nil, "", "", domain.NewError(domain.ErrValidation, prefix+".finding_id", "同一批量请求不能重复整改异常 %s", item.FindingID)
			}
			seen[item.FindingID] = true
			fi, ok := findingIndex[item.FindingID]
			if !ok || batch.Findings[fi].BatchID != batch.BatchID {
				return nil, "", "", domain.NewError(domain.ErrNotFound, prefix+".finding_id", "异常不属于当前批次")
			}
			finding := &batch.Findings[fi]
			if finding.Status != domain.FindingOpen {
				return nil, "", "", domain.NewError(domain.ErrValidation, prefix+".finding_id", "异常 %s 已关闭，不能重复整改", item.FindingID)
			}
			if len(strings.TrimSpace(item.Reason)) < 4 {
				return nil, "", "", domain.NewError(domain.ErrValidation, prefix+".reason", "整改理由至少 4 个字符")
			}
			if finding.ObservationID == "" {
				if finding.RuleCode != "REVIEW_RETURN" {
					return nil, "", "", domain.NewError(domain.ErrValidation, prefix+".replacement", "该批次级异常不能通过替换观测解决")
				}
				finding.Status = domain.FindingResolved
				finding.AfterValue = map[string]any{"acknowledged": true}
				finding.ResolutionReason = strings.TrimSpace(item.Reason)
				finding.ResolvedAt = &now
				results = append(results, CorrectionItemResult{FindingID: item.FindingID, Status: "RESOLVED"})
				continue
			}
			if replacementEmpty(item.Replacement) {
				return nil, "", "", domain.NewError(domain.ErrValidation, prefix+".replacement", "观测异常至少需要一个替换字段")
			}
			oi, ok := observationAlias[finding.ObservationID]
			if !ok {
				oi = observationIndex(batch.Observations, finding.ObservationID)
			}
			if oi < 0 {
				return nil, "", "", domain.NewError(domain.ErrIntegrity, prefix+".finding_id", "异常引用的观测不存在")
			}
			old := batch.Observations[oi]
			updated := applyReplacement(old, item.Replacement)
			updated.SupersedesID = old.ObservationID
			updated.ObservationID = domain.StableID(old.ObservationID, cmd.RequestID, item.FindingID)
			updated.RecordedAt = now
			batch.Observations[oi] = updated
			observationAlias[finding.ObservationID] = oi
			touchedObservations[oi] = itemIndex
			finding.Status = domain.FindingResolved
			finding.BeforeValue = old
			finding.AfterValue = updated
			finding.ResolutionReason = strings.TrimSpace(item.Reason)
			finding.ResolvedAt = &now
			results = append(results, CorrectionItemResult{FindingID: item.FindingID, ObservationID: updated.ObservationID, SupersedesID: updated.SupersedesID, Status: "RESOLVED"})
		}
		for observationIndex, itemIndex := range touchedObservations {
			if err := domain.ValidateReplacementObservation(batch.Observations[observationIndex]); err != nil {
				if de, ok := err.(*domain.DomainError); ok {
					de.Field = fmt.Sprintf("items[%d].replacement.%s", itemIndex, de.Field)
				}
				return nil, "", "", err
			}
		}
		result := domain.RunQualityRules(batch, now)
		batch.Findings = append(resolvedFindings(batch.Findings), result.Findings...)
		batch.LastRuleRunAt = &now
		if result.Passed {
			batch.State = domain.StateReviewReady
		} else {
			batch.State = domain.StateCorrectionRequired
		}
		return &CommandResponse{RuleResult: &result, Corrections: results}, fmt.Sprintf("批量整改 %d 项异常并统一运行一次质量规则", len(results)), "", nil
	})
}

func observationIndex(observations []domain.RingObservation, id string) int {
	for i := range observations {
		if observations[i].ObservationID == id {
			return i
		}
	}
	return -1
}

func replacementEmpty(r Replacement) bool {
	return r.WidthMicrons == nil && r.CalendarYear == nil && r.BoundaryPosition == nil && r.MarkerKind == nil && r.MarkerNote == nil && r.AnchorGroup == nil
}

func applyReplacement(old domain.RingObservation, replacement Replacement) domain.RingObservation {
	updated := old
	if replacement.WidthMicrons != nil {
		updated.WidthMicrons = *replacement.WidthMicrons
	}
	if replacement.CalendarYear != nil {
		updated.CalendarYear = *replacement.CalendarYear
	}
	if replacement.BoundaryPosition != nil {
		updated.BoundaryPosition = *replacement.BoundaryPosition
	}
	if replacement.MarkerKind != nil {
		updated.MarkerKind = *replacement.MarkerKind
	}
	if replacement.MarkerNote != nil {
		updated.MarkerNote = *replacement.MarkerNote
	}
	if replacement.AnchorGroup != nil {
		updated.AnchorGroup = *replacement.AnchorGroup
	}
	return updated
}
