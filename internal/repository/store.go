package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"dendro-chronology-workbench/internal/domain"
)

var ErrNotFound = errors.New("repository: batch not found")

type IdempotencyRecord struct {
	RequestID      string          `json:"request_id"`
	Action         string          `json:"action"`
	RequestDigest  string          `json:"request_digest"`
	StatusCode     int             `json:"status_code"`
	Response       json.RawMessage `json:"response"`
	Revision       int64           `json:"revision"`
}

type Snapshot struct {
	Version        int                          `json:"version"`
	Batch          *domain.DendroBatch          `json:"batch"`
	Idempotency    map[string]IdempotencyRecord `json:"idempotency"`
	Manifest       *domain.Manifest             `json:"manifest,omitempty"`
	LastEvent      string                       `json:"last_event_digest"`
	SnapshotDigest string                       `json:"snapshot_digest"`
}

type Store struct {
	root      string
	snapshots string
	audit     string
	mu        sync.RWMutex
}

func Open(root string) (*Store, error) {
	if root == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	s := &Store{root: root, snapshots: filepath.Join(root, "snapshots"), audit: filepath.Join(root, "audit")}
	if err := os.MkdirAll(s.snapshots, 0o750); err != nil {
		return nil, fmt.Errorf("创建快照目录: %w", err)
	}
	if err := os.MkdirAll(s.audit, 0o750); err != nil {
		return nil, fmt.Errorf("创建审计目录: %w", err)
	}
	if err := s.VerifyAll(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Load(batchID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadUnlocked(batchID)
}

func (s *Store) loadUnlocked(batchID string) (*Snapshot, error) {
	if err := domain.ValidateID("batch_id", batchID); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.snapshotPath(batchID))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, fmt.Errorf("解析批次 %s 快照: %w", batchID, err)
	}
	if err := verifySnapshot(&snap); err != nil {
		return nil, err
	}
	return cloneSnapshot(&snap)
}

func (s *Store) List() ([]*domain.DendroBatch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.snapshots)
	if err != nil {
		return nil, err
	}
	var out []*domain.DendroBatch
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-len(".json")]
		snap, err := s.loadUnlocked(id)
		if err != nil {
			return nil, err
		}
		batch, err := domain.CloneBatch(snap.Batch)
		if err != nil {
			return nil, err
		}
		out = append(out, batch)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].BatchID < out[j].BatchID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) Manifest(batchID string) (*domain.Manifest, error) {
	snap, err := s.Load(batchID)
	if err != nil {
		return nil, err
	}
	if snap.Manifest == nil {
		return nil, ErrNotFound
	}
	m := *snap.Manifest
	return &m, nil
}

func (s *Store) LastEventDigest(batchID string) (string, error) {
	snap, err := s.Load(batchID)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return snap.LastEvent, nil
}

func (s *Store) snapshotPath(id string) string { return filepath.Join(s.snapshots, id+".json") }

func (s *Store) auditPath(id string) string { return filepath.Join(s.audit, id+".jsonl") }

func cloneSnapshot(in *Snapshot) (*Snapshot, error) {
	b, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var out Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
