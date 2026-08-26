package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
)

type Service struct {
	store           *repository.Store
	locks           *keyedLocks
	clock           func() time.Time
	inspectionCache map[string]EvidenceInspection
}

func New(store *repository.Store) *Service {
	return &Service{
		store:           store,
		locks:           newKeyedLocks(),
		clock:           func() time.Time { return time.Now().UTC() },
		inspectionCache: map[string]EvidenceInspection{},
	}
}

func (s *Service) List() ([]*domain.DendroBatch, error) { return s.store.List() }
func (s *Service) Get(id string) (*domain.DendroBatch, error) {
	snap, err := s.store.Load(id)
	if err != nil {
		return nil, translateRepo(err)
	}
	return snap.Batch, nil
}
func (s *Service) Manifest(id string) (*domain.Manifest, error) {
	m, err := s.store.Manifest(id)
	if err != nil {
		return nil, translateRepo(err)
	}
	return m, nil
}
func (s *Service) Events(id string) ([]domain.AuditEvent, error) {
	e, err := s.store.Events(id)
	if err != nil {
		return nil, translateRepo(err)
	}
	return e, nil
}

func validateMeta(meta CommandMeta) error {
	if err := domain.ValidateID("request_id", meta.RequestID); err != nil {
		return err
	}
	if meta.ExpectedRevision < 0 {
		return domain.NewError(domain.ErrValidation, "expected_revision", "expected_revision 不能为负数")
	}
	if err := domain.ValidateID("actor_id", meta.ActorID); err != nil {
		return err
	}
	return nil
}

type mutation func(batch *domain.DendroBatch, snap *repository.Snapshot, now time.Time) (*CommandResponse, string, string, error)

func (s *Service) mutate(batchID, action string, meta CommandMeta, status int, fn mutation) (Result, error) {
	if err := validateMeta(meta); err != nil {
		return Result{}, err
	}
	unlock := s.locks.lock(batchID)
	defer unlock()
	snap, err := s.store.Load(batchID)
	if err != nil {
		return Result{}, translateRepo(err)
	}
	if previous, ok := snap.Idempotency[meta.RequestID]; ok {
		if previous.Action != action {
			return Result{}, domain.NewError(domain.ErrConflict, "request_id", "request_id 已用于其他操作")
		}
		return Result{StatusCode: previous.StatusCode, Body: append(json.RawMessage(nil), previous.Response...), Replayed: true}, nil
	}
	if snap.Batch.Revision != meta.ExpectedRevision {
		return Result{}, domain.NewError(domain.ErrConflict, "expected_revision", "期望 revision %d，当前为 %d", meta.ExpectedRevision, snap.Batch.Revision)
	}
	batch, err := domain.CloneBatch(snap.Batch)
	if err != nil {
		return Result{}, err
	}
	now := s.clock()
	response, reason, actor, err := fn(batch, snap, now)
	if err != nil {
		return Result{}, err
	}
	if actor == "" {
		actor = meta.ActorID
	}
	batch.Revision++
	batch.UpdatedAt = now
	response.Batch = batch
	body, err := json.Marshal(response)
	if err != nil {
		return Result{}, err
	}
	payload := map[string]any{"state": batch.State}
	if len(response.Corrections) > 0 {
		payload["processed_count"] = len(response.Corrections)
	}
	event := domain.AuditEvent{EventID: domain.StableID(batchID, fmt.Sprint(batch.Revision), meta.RequestID), BatchID: batchID, Revision: batch.Revision, RequestID: meta.RequestID, Action: action, ActorID: actor, Reason: reason, OccurredAt: now, PreviousDigest: snap.LastEvent, Payload: payload}
	if err := domain.FinalizeEvent(&event); err != nil {
		return Result{}, err
	}
	snap.Batch = batch
	snap.Idempotency[meta.RequestID] = repository.IdempotencyRecord{RequestID: meta.RequestID, Action: action, StatusCode: status, Response: body, Revision: batch.Revision}
	if response.Manifest != nil {
		m := *response.Manifest
		snap.Manifest = &m
	}
	if err := s.store.Commit(snap, event); err != nil {
		return Result{}, err
	}
	return Result{StatusCode: status, Body: body}, nil
}

func (s *Service) Create(cmd CreateBatchCommand) (Result, error) {
	if cmd.ActorID == "" {
		cmd.ActorID = cmd.OperatorID
	}
	if cmd.ActorID != cmd.OperatorID {
		return Result{}, domain.NewError(domain.ErrForbidden, "actor_id", "建档人必须与测量员一致")
	}
	if err := validateMeta(cmd.CommandMeta); err != nil {
		return Result{}, err
	}
	if cmd.ExpectedRevision != 0 {
		return Result{}, domain.NewError(domain.ErrConflict, "expected_revision", "创建批次时 expected_revision 必须为 0")
	}
	unlock := s.locks.lock(cmd.BatchID)
	defer unlock()
	if existing, err := s.store.Load(cmd.BatchID); err == nil {
		if rec, ok := existing.Idempotency[cmd.RequestID]; ok && rec.Action == "CREATE_BATCH" {
			return Result{StatusCode: rec.StatusCode, Body: rec.Response, Replayed: true}, nil
		}
		return Result{}, domain.NewError(domain.ErrConflict, "batch_id", "批次 %s 已存在", cmd.BatchID)
	} else if !errors.Is(err, repository.ErrNotFound) {
		return Result{}, err
	}
	now := s.clock()
	batch := &domain.DendroBatch{BatchID: cmd.BatchID, SiteCode: strings.TrimSpace(cmd.SiteCode), Species: strings.TrimSpace(cmd.Species), SampledAt: cmd.SampledAt, OperatorID: cmd.OperatorID, State: domain.StateBaselined, Revision: 1, CreatedAt: now, UpdatedAt: now, Cores: cmd.Cores}
	if err := domain.ValidateNewBatch(batch, now); err != nil {
		return Result{}, err
	}
	response := CommandResponse{Batch: batch}
	body, err := json.Marshal(response)
	if err != nil {
		return Result{}, err
	}
	event := domain.AuditEvent{EventID: domain.StableID(cmd.BatchID, "1", cmd.RequestID), BatchID: cmd.BatchID, Revision: 1, RequestID: cmd.RequestID, Action: "CREATE_BATCH", ActorID: cmd.OperatorID, Reason: "冻结采样基线", OccurredAt: now, Payload: map[string]any{"state": batch.State, "core_count": len(batch.Cores)}}
	if err := domain.FinalizeEvent(&event); err != nil {
		return Result{}, err
	}
	snap := repository.NewSnapshot(batch)
	snap.Idempotency[cmd.RequestID] = repository.IdempotencyRecord{RequestID: cmd.RequestID, Action: "CREATE_BATCH", StatusCode: 201, Response: body, Revision: 1}
	if err := s.store.Commit(snap, event); err != nil {
		return Result{}, err
	}
	return Result{StatusCode: 201, Body: body}, nil
}

func translateRepo(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return domain.NewError(domain.ErrNotFound, "batch_id", "批次不存在")
	}
	return err
}
