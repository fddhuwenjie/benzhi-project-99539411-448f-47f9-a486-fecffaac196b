package repository

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"dendro-chronology-workbench/internal/domain"
)

type AuditInspection struct {
	Events             []domain.AuditEvent `json:"events"`
	Valid              bool                `json:"valid"`
	Details            string              `json:"details"`
	RevisionContinuous bool                `json:"revision_continuous"`
	SnapshotConsistent bool                `json:"snapshot_consistent"`
}

// ReadEvidence 在同一个仓储读锁内取得快照与审计证据，供只读预检核对。
// 审计链损坏会成为不可签署的检查结果，而不是丢失具体原因。
func (s *Store) ReadEvidence(batchID string) (*Snapshot, AuditInspection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, err := s.loadUnlocked(batchID)
	if err != nil {
		return nil, AuditInspection{}, err
	}
	inspection := AuditInspection{}
	events, err := s.eventsUnlocked(batchID)
	if err != nil {
		inspection.Details = err.Error()
		return snap, inspection, nil
	}
	inspection.Events = events
	inspection.RevisionContinuous = len(events) > 0 && int64(len(events)) == snap.Batch.Revision
	inspection.SnapshotConsistent = inspection.RevisionContinuous && events[len(events)-1].Digest == snap.LastEvent
	if inspection.SnapshotConsistent {
		if payload, ok := events[len(events)-1].Payload.(map[string]any); ok {
			if state, ok := payload["state"].(string); ok && state != string(snap.Batch.State) {
				inspection.SnapshotConsistent = false
			}
		}
	}
	inspection.Valid = inspection.RevisionContinuous && inspection.SnapshotConsistent
	if inspection.Valid {
		inspection.Details = fmt.Sprintf("%d 个审计 revision 连续，末端摘要与当前快照一致", len(events))
	} else if !inspection.RevisionContinuous {
		inspection.Details = fmt.Sprintf("审计事件数 %d 与当前 revision %d 不一致", len(events), snap.Batch.Revision)
	} else {
		inspection.Details = "审计末端摘要或状态与当前快照不一致"
	}
	return snap, inspection, nil
}

func (s *Store) Events(batchID string) ([]domain.AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.eventsUnlocked(batchID)
}

func (s *Store) eventsUnlocked(batchID string) ([]domain.AuditEvent, error) {
	f, err := os.Open(s.auditPath(batchID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	var out []domain.AuditEvent
	previous := ""
	for scanner.Scan() {
		var event domain.AuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return nil, fmt.Errorf("审计事件 JSON 无效: %w", err)
		}
		if event.BatchID != batchID || event.Revision != int64(len(out)+1) {
			return nil, domain.NewError(domain.ErrIntegrity, "audit", "批次 %s 审计 revision 不连续", batchID)
		}
		if event.PreviousDigest != previous || !domain.VerifyEvent(event) {
			return nil, domain.NewError(domain.ErrIntegrity, "audit", "批次 %s 审计哈希链无效", batchID)
		}
		previous = event.Digest
		out = append(out, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) VerifyBatch(batchID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap, err := s.loadUnlocked(batchID)
	if err != nil {
		return err
	}
	events, err := s.eventsUnlocked(batchID)
	if err != nil {
		return err
	}
	if len(events) == 0 || int64(len(events)) != snap.Batch.Revision {
		return domain.NewError(domain.ErrIntegrity, "audit", "审计事件数与快照 revision 不一致")
	}
	if events[len(events)-1].Digest != snap.LastEvent {
		return domain.NewError(domain.ErrIntegrity, "audit", "快照未指向最后审计事件")
	}
	if snap.Manifest != nil && !domain.VerifyManifest(*snap.Manifest) {
		return domain.NewError(domain.ErrIntegrity, "manifest", "封存清单摘要无效")
	}
	return nil
}
