package repository

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"dendro-chronology-workbench/internal/domain"
)

// repairAuditTail removes only a fully valid event-chain suffix that was
// fsynced before a process stopped but whose corresponding snapshot rename did
// not complete. It never repairs a broken digest, revision gap, or divergent
// event, because those conditions represent evidence corruption rather than a
// recoverable interrupted commit.
func (s *Store) repairAuditTail(batchID string) error {
	snap, err := s.loadUnlocked(batchID)
	if err != nil {
		return err
	}
	events, err := s.eventsUnlocked(batchID)
	if err != nil {
		return err
	}
	committed := snap.Batch.Revision
	if int64(len(events)) <= committed {
		return nil
	}
	if committed < 1 || committed > int64(len(events)) {
		return domain.NewError(domain.ErrIntegrity, "audit", "无法定位已提交审计边界")
	}
	if events[committed-1].Digest != snap.LastEvent {
		return domain.NewError(domain.ErrIntegrity, "audit", "审计尾部与快照分叉，拒绝自动修复")
	}
	cut, err := auditOffsetAfter(s.auditPath(batchID), committed)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.auditPath(batchID), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := f.Truncate(cut); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return syncDirectory(s.audit)
}

func auditOffsetAfter(path string, lines int64) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	reader := bufio.NewReader(f)
	var offset int64
	for line := int64(0); line < lines; line++ {
		chunk, readErr := reader.ReadBytes('\n')
		offset += int64(len(chunk))
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return 0, fmt.Errorf("审计文件在 revision %d 前意外结束", lines)
			}
			return 0, readErr
		}
	}
	return offset, nil
}
