package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dendro-chronology-workbench/internal/domain"
)

func (s *Store) VerifyAll() error {
	if err := s.repairOrphanedAudits(); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.snapshots)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if err := s.repairAuditTail(id); err != nil {
			return fmt.Errorf("恢复批次 %s 的未提交审计尾部失败: %w", id, err)
		}
		if err := s.VerifyBatch(id); err != nil {
			return fmt.Errorf("恢复批次 %s 失败: %w", id, err)
		}
	}
	return nil
}

// repairOrphanedAudits scans the audit directory for JSONL files that have no
// corresponding snapshot. Such a file represents a commit whose audit event was
// fsynced before the process stopped but whose snapshot rename never completed.
// When the orphaned chain contains exactly one valid revision-1 event, the
// commit never landed and the orphan is removed so the batch can be created
// cleanly again. Any other orphaned audit content is treated as evidence
// corruption rather than a recoverable interrupted commit.
func (s *Store) repairOrphanedAudits() error {
	entries, err := os.ReadDir(s.audit)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".jsonl" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		if _, statErr := os.Stat(s.snapshotPath(id)); statErr == nil {
			continue
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := s.repairOrphanedAudit(id); err != nil {
			return fmt.Errorf("恢复孤儿审计 %s 失败: %w", id, err)
		}
	}
	return nil
}

func (s *Store) repairOrphanedAudit(batchID string) error {
	events, err := s.eventsUnlocked(batchID)
	if err != nil {
		return fmt.Errorf("孤儿审计链无效: %w", err)
	}
	if len(events) == 0 {
		return os.Remove(s.auditPath(batchID))
	}
	if len(events) == 1 && events[0].Revision == 1 && events[0].PreviousDigest == "" && domain.VerifyEvent(events[0]) {
		return os.Remove(s.auditPath(batchID))
	}
	return domain.NewError(domain.ErrIntegrity, "audit", "批次 %s 存在无快照的非首事件孤儿审计", batchID)
}

func NewSnapshot(batch *domain.DendroBatch) *Snapshot {
	return &Snapshot{Version: 1, Batch: batch, Idempotency: map[string]IdempotencyRecord{}}
}
