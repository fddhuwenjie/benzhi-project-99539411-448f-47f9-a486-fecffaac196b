package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dendro-chronology-workbench/internal/domain"
)

func (s *Store) VerifyAll() error {
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

func NewSnapshot(batch *domain.DendroBatch) *Snapshot {
	return &Snapshot{Version: 1, Batch: batch, Idempotency: map[string]IdempotencyRecord{}}
}
