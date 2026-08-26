package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type RevisionDifference struct {
	CoreID           string          `json:"core_id"`
	FindingID        string          `json:"finding_id"`
	RuleCode         string          `json:"rule_code"`
	ObservationID    string          `json:"observation_id"`
	SupersedesID     string          `json:"supersedes_id"`
	BeforeValue      RingObservation `json:"before_value"`
	AfterValue       RingObservation `json:"after_value"`
	ResolutionReason string          `json:"resolution_reason"`
	ResolvedAt       time.Time       `json:"resolved_at"`
	AuditRevision    int64           `json:"audit_revision,omitempty"`
}

func RevisionDifferences(batch *DendroBatch) ([]RevisionDifference, []string) {
	var differences []RevisionDifference
	var problems []string
	for _, finding := range batch.Findings {
		if finding.Status != FindingResolved || finding.ObservationID == "" {
			continue
		}
		var before, after RingObservation
		if !decodeObservation(finding.BeforeValue, &before) || !decodeObservation(finding.AfterValue, &after) {
			problems = append(problems, fmt.Sprintf("异常 %s 缺少可核对的替换前后观测", finding.FindingID))
			continue
		}
		if before.ObservationID == "" || after.ObservationID == "" || after.SupersedesID != before.ObservationID {
			problems = append(problems, fmt.Sprintf("异常 %s 的 supersedes 引用不连续", finding.FindingID))
		}
		if before.CoreID != after.CoreID || finding.CoreID != after.CoreID {
			problems = append(problems, fmt.Sprintf("异常 %s 的样芯引用不一致", finding.FindingID))
		}
		if finding.ResolutionReason == "" || finding.ResolvedAt == nil {
			problems = append(problems, fmt.Sprintf("异常 %s 缺少整改理由或完成时间", finding.FindingID))
			continue
		}
		differences = append(differences, RevisionDifference{CoreID: after.CoreID, FindingID: finding.FindingID, RuleCode: finding.RuleCode, ObservationID: after.ObservationID, SupersedesID: after.SupersedesID, BeforeValue: before, AfterValue: after, ResolutionReason: finding.ResolutionReason, ResolvedAt: *finding.ResolvedAt})
	}
	sort.SliceStable(differences, func(i, j int) bool {
		if differences[i].CoreID != differences[j].CoreID {
			return differences[i].CoreID < differences[j].CoreID
		}
		if differences[i].FindingID != differences[j].FindingID {
			return differences[i].FindingID < differences[j].FindingID
		}
		if !differences[i].ResolvedAt.Equal(differences[j].ResolvedAt) {
			return differences[i].ResolvedAt.Before(differences[j].ResolvedAt)
		}
		return differences[i].ObservationID < differences[j].ObservationID
	})
	sort.Strings(problems)
	return differences, problems
}

func decodeObservation(value any, target *RingObservation) bool {
	b, err := json.Marshal(value)
	if err != nil {
		return false
	}
	if err := json.Unmarshal(b, target); err != nil {
		return false
	}
	return target.ObservationID != ""
}
