package repository

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"dendro-chronology-workbench/internal/domain"
)

func (s *Store) Commit(snap *Snapshot, event domain.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if snap == nil || snap.Batch == nil {
		return fmt.Errorf("空快照")
	}
	current, err := s.loadUnlocked(snap.Batch.BatchID)
	if err != nil && err != ErrNotFound {
		return err
	}
	if current == nil {
		if snap.Batch.Revision != 1 || event.Revision != 1 || event.PreviousDigest != "" {
			return domain.NewError(domain.ErrConflict, "revision", "首个提交的 revision 必须为 1")
		}
	} else {
		if snap.Batch.Revision != current.Batch.Revision+1 {
			return domain.NewError(domain.ErrConflict, "revision", "保存 revision 不连续")
		}
		if event.Revision != snap.Batch.Revision || event.PreviousDigest != current.LastEvent {
			return domain.NewError(domain.ErrIntegrity, "event", "审计事件链前序摘要不匹配")
		}
	}
	if !domain.VerifyEvent(event) {
		return domain.NewError(domain.ErrIntegrity, "event", "审计事件摘要无效")
	}
	snap.Version = 1
	snap.LastEvent = event.Digest
	if snap.Idempotency == nil {
		snap.Idempotency = map[string]IdempotencyRecord{}
	}
	snap.SnapshotDigest = ""
	d, err := snapshotDigest(snap)
	if err != nil {
		return err
	}
	snap.SnapshotDigest = d
	line, err := json.Marshal(event)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	auditFile, err := s.auditWriter(snap.Batch.BatchID)
	if err != nil {
		return err
	}
	pos, err := auditFile.Seek(0, 2)
	if err != nil {
		return err
	}
	rollback := func() {
		_ = auditFile.Truncate(pos)
		_ = auditFile.Sync()
		s.releaseAuditWriter(snap.Batch.BatchID)
	}
	if _, err = auditFile.Write(line); err != nil {
		rollback()
		return err
	}
	if err = auditFile.Sync(); err != nil {
		rollback()
		return err
	}
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		rollback()
		return err
	}
	if err = atomicWrite(s.snapshotPath(snap.Batch.BatchID), b, 0o640); err != nil {
		rollback()
		return err
	}
	return syncDirectory(s.root)
}

// auditWriter 复用批次的追加句柄，避免每个 revision 都重新打开审计文件。
func (s *Store) auditWriter(batchID string) (*os.File, error) {
	if writer := s.auditWriters[batchID]; writer != nil {
		return writer, nil
	}
	writer, err := os.OpenFile(s.auditPath(batchID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, err
	}
	s.auditWriters[batchID] = writer
	return writer, nil
}

func (s *Store) releaseAuditWriter(batchID string) {
	writer := s.auditWriters[batchID]
	delete(s.auditWriters, batchID)
	if writer != nil {
		_ = writer.Close()
	}
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".snapshot-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func verifySnapshot(snap *Snapshot) error {
	if snap.Version != 1 || snap.Batch == nil {
		return domain.NewError(domain.ErrIntegrity, "snapshot", "快照版本或聚合无效")
	}
	expected := snap.SnapshotDigest
	actual, err := snapshotDigest(snap)
	if err != nil || actual != expected {
		return domain.NewError(domain.ErrIntegrity, "snapshot", "快照摘要校验失败")
	}
	return nil
}

func snapshotDigest(snap *Snapshot) (string, error) {
	b, err := json.Marshal(snap)
	if err != nil {
		return "", err
	}
	var persisted Snapshot
	if err := json.Unmarshal(b, &persisted); err != nil {
		return "", err
	}
	persisted.SnapshotDigest = ""
	return domain.Digest(&persisted)
}
