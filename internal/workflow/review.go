package workflow

import (
	"strings"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

func (s *Service) Review(batchID string, cmd ReviewCommand) (Result, error) {
	if cmd.ActorID == "" {
		cmd.ActorID = cmd.ReviewerID
	}
	return s.mutate(batchID, "REVIEW_BATCH", cmd.CommandMeta, 200, func(batch *domain.DendroBatch, _ *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := domain.EnsureState(batch, domain.StateReviewReady); err != nil {
			return nil, "", "", err
		}
		if err := domain.ValidateID("reviewer_id", cmd.ReviewerID); err != nil {
			return nil, "", "", err
		}
		if cmd.ReviewerID == batch.OperatorID {
			return nil, "", "", domain.NewError(domain.ErrForbidden, "reviewer_id", "复核员必须不同于测量员")
		}
		inspectedRevision := cmd.ExpectedRevision
		if cmd.InspectedRevision != nil {
			inspectedRevision = *cmd.InspectedRevision
		}
		if inspectedRevision != batch.Revision {
			return nil, "", "", domain.NewError(domain.ErrConflict, "inspected_revision", "预检 revision %d 已陈旧，当前为 %d", inspectedRevision, batch.Revision)
		}
		if len(strings.TrimSpace(cmd.Note)) < 4 {
			return nil, "", "", domain.NewError(domain.ErrValidation, "note", "复核说明至少 4 个字符")
		}
		decision := strings.ToUpper(cmd.Decision)
		if decision != "APPROVE" && decision != "RETURN" {
			return nil, "", "", domain.NewError(domain.ErrValidation, "decision", "复核决定只能为 APPROVE 或 RETURN")
		}
		_, audit, err := s.store.ReadEvidence(batch.BatchID)
		if err != nil {
			return nil, "", "", translateRepo(err)
		}
		inspection := assembleInspection(batch, cmd.ReviewerID, audit)
		if decision == "APPROVE" && !inspection.Signable {
			return nil, "", "", domain.NewError(domain.ErrValidation, "evidence", "复核证据预检未全部通过，禁止签署")
		}
		if !audit.Valid {
			return nil, "", "", domain.NewError(domain.ErrIntegrity, "audit", audit.Details)
		}
		batch.ReviewerID = cmd.ReviewerID
		batch.Review = &domain.ReviewSeal{SealID: domain.StableID(batch.BatchID, cmd.RequestID, "review"), BatchID: batch.BatchID, ReviewerID: cmd.ReviewerID, Decision: decision, ReviewNote: strings.TrimSpace(cmd.Note), VerifiedRevision: batch.Revision + 1, SignedAt: now}
		if decision == "APPROVE" {
			batch.State = domain.StateVerified
		} else {
			batch.State = domain.StateCorrectionRequired
			batch.Findings = append(batch.Findings, domain.QualityFinding{FindingID: domain.StableID(batch.BatchID, cmd.RequestID, "return"), BatchID: batch.BatchID, RuleCode: "REVIEW_RETURN", Severity: "ERROR", Message: "独立复核退回：" + strings.TrimSpace(cmd.Note), Status: domain.FindingOpen, CreatedAt: now})
		}
		return &CommandResponse{Inspection: &inspection}, "独立复核决定：" + decision, cmd.ReviewerID, nil
	})
}

func (s *Service) Seal(batchID string, cmd SealCommand) (Result, error) {
	return s.mutate(batchID, "SEAL_BATCH", cmd.CommandMeta, 200, func(batch *domain.DendroBatch, snap *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := requireOperator(batch, cmd.ActorID); err != nil {
			return nil, "", "", err
		}
		if err := domain.EnsureState(batch, domain.StateVerified); err != nil {
			return nil, "", "", err
		}
		manifest, err := domain.BuildManifest(batch, snap.LastEvent, now)
		if err != nil {
			return nil, "", "", err
		}
		batch.State = domain.StateSealed
		batch.SealedAt = &now
		batch.Review.ManifestDigest = manifest.ManifestDigest
		batch.Review.EventChainDigest = manifest.EventChainDigest
		return &CommandResponse{Manifest: &manifest}, "生成并校验确定性封存清单", "", nil
		// Note: mutate() finalizes the manifest EventChainDigest with the
		// seal event digest and refreshes ManifestDigest and review seal.
	})
}
