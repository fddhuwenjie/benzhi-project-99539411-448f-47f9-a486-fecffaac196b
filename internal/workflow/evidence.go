package workflow

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

func (s *Service) RegisterImages(batchID string, cmd RegisterImagesCommand) (Result, error) {
	digest, err := commandDigest("REGISTER_IMAGES", cmd)
	if err != nil {
		return Result{}, err
	}
	return s.mutate(batchID, "REGISTER_IMAGES", cmd.CommandMeta, digest, 200, func(batch *domain.DendroBatch, _ *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := requireOperator(batch, cmd.ActorID); err != nil {
			return nil, "", "", err
		}
		if err := domain.EnsureState(batch, domain.StateBaselined); err != nil {
			return nil, "", "", err
		}
		if len(cmd.Images) != len(batch.Cores) {
			return nil, "", "", domain.NewError(domain.ErrValidation, "images", "必须为每根基线样芯一次性登记影像")
		}
		inputs := map[string]ImageInput{}
		for _, in := range cmd.Images {
			if _, ok := inputs[in.CoreID]; ok {
				return nil, "", "", domain.NewError(domain.ErrValidation, "images", "样芯 %s 的影像重复", in.CoreID)
			}
			inputs[in.CoreID] = in
		}
		for i := range batch.Cores {
			in, ok := inputs[batch.Cores[i].CoreID]
			if !ok {
				return nil, "", "", domain.NewError(domain.ErrValidation, "images", "样芯 %s 缺少影像", batch.Cores[i].CoreID)
			}
			captured := in.CapturedAt
			candidate := batch.Cores[i]
			candidate.PreparationMethod = strings.TrimSpace(in.PreparationMethod)
			candidate.ImageDigest = strings.ToLower(in.ImageDigest)
			candidate.MicronsPerPixel = in.MicronsPerPixel
			candidate.CapturedAt = &captured
			if err := domain.ValidateImage(candidate); err != nil {
				return nil, "", "", err
			}
			batch.Cores[i] = candidate
		}
		batch.State = domain.StateImaged
		return &CommandResponse{}, "全部基线样芯影像校验通过", "", nil
	})
}

func (s *Service) SubmitObservations(batchID string, cmd SubmitObservationsCommand) (Result, error) {
	digest, err := commandDigest("SUBMIT_OBSERVATIONS", cmd)
	if err != nil {
		return Result{}, err
	}
	return s.mutate(batchID, "SUBMIT_OBSERVATIONS", cmd.CommandMeta, digest, 200, func(batch *domain.DendroBatch, _ *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := requireOperator(batch, cmd.ActorID); err != nil {
			return nil, "", "", err
		}
		if err := domain.EnsureState(batch, domain.StateImaged); err != nil {
			return nil, "", "", err
		}
		if len(cmd.Observations) == 0 {
			return nil, "", "", domain.NewError(domain.ErrValidation, "observations", "观测列表不能为空")
		}
		observations := append([]domain.RingObservation(nil), cmd.Observations...)
		seenID := map[string]bool{}
		perCore := map[string]int{}
		for i := range observations {
			o := &observations[i]
			if !domain.HasCore(batch, o.CoreID) {
				return nil, "", "", domain.NewError(domain.ErrValidation, "core_id", "观测引用未知样芯 %s", o.CoreID)
			}
			if seenID[o.ObservationID] {
				return nil, "", "", domain.NewError(domain.ErrValidation, "observation_id", "观测编号 %s 重复", o.ObservationID)
			}
			if o.MarkerKind == "" {
				o.MarkerKind = domain.MarkerNone
			}
			if err := domain.ValidateObservation(*o); err != nil {
				return nil, "", "", err
			}
			seenID[o.ObservationID] = true
			perCore[o.CoreID]++
			o.RecordedAt = now
		}
		for _, c := range batch.Cores {
			if perCore[c.CoreID] == 0 {
				return nil, "", "", domain.NewError(domain.ErrValidation, "observations", "样芯 %s 没有观测", c.CoreID)
			}
		}
		sort.Slice(observations, func(i, j int) bool {
			if observations[i].CoreID == observations[j].CoreID {
				return observations[i].RingIndex < observations[j].RingIndex
			}
			return observations[i].CoreID < observations[j].CoreID
		})
		batch.Observations = append([]domain.RingObservation(nil), observations...)
		batch.Findings = nil
		batch.State = domain.StateAnalyzed
		return &CommandResponse{}, fmt.Sprintf("提交 %d 条年轮观测", len(observations)), "", nil
	})
}

func (s *Service) Validate(batchID string, cmd ValidateCommand) (Result, error) {
	digest, err := commandDigest("RUN_QUALITY_RULES", cmd)
	if err != nil {
		return Result{}, err
	}
	return s.mutate(batchID, "RUN_QUALITY_RULES", cmd.CommandMeta, digest, 200, func(batch *domain.DendroBatch, _ *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error) {
		if err := requireOperator(batch, cmd.ActorID); err != nil {
			return nil, "", "", err
		}
		if err := domain.EnsureState(batch, domain.StateAnalyzed, domain.StateCorrectionRequired); err != nil {
			return nil, "", "", err
		}
		result := domain.RunQualityRules(batch, now)
		history := resolvedFindings(batch.Findings)
		batch.Findings = append(history, result.Findings...)
		batch.LastRuleRunAt = &now
		if result.Passed {
			batch.State = domain.StateReviewReady
		} else {
			batch.State = domain.StateCorrectionRequired
		}
		return &CommandResponse{RuleResult: &result}, fmt.Sprintf("运行质量规则，发现 %d 项异常", len(result.Findings)), "", nil
	})
}

func resolvedFindings(all []domain.QualityFinding) []domain.QualityFinding {
	out := make([]domain.QualityFinding, 0, len(all))
	for _, f := range all {
		if f.Status == domain.FindingResolved {
			out = append(out, f)
		}
	}
	return out
}
