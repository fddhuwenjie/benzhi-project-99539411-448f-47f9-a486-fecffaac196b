package repository

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
)

func commitTestEvent(t *testing.T, store *Store, snap *Snapshot, request string) {
	t.Helper()
	revision := snap.Batch.Revision
	event := domain.AuditEvent{EventID: domain.StableID(request), BatchID: snap.Batch.BatchID, Revision: revision, RequestID: request, Action: "TEST", ActorID: "actor-test", OccurredAt: time.Now().UTC(), PreviousDigest: snap.LastEvent}
	if err := domain.FinalizeEvent(&event); err != nil {
		t.Fatal(err)
	}
	if err := store.Commit(snap, event); err != nil {
		t.Fatal(err)
	}
}

func TestStoreCommitReopenAndVerify(t *testing.T) {
	root := t.TempDir()
	store, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	batch := &domain.DendroBatch{BatchID: "batch-store", SiteCode: "S", Species: "P", OperatorID: "operator-x", State: domain.StateBaselined, Revision: 1, CreatedAt: time.Now().UTC()}
	snap := NewSnapshot(batch)
	snap.Idempotency["req-one"] = IdempotencyRecord{RequestID: "req-one", Action: "TEST", StatusCode: 200, Response: json.RawMessage(`{"ok":true}`), Revision: 1}
	commitTestEvent(t, store, snap, "req-one")
	loaded, err := store.Load(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Batch.Revision = 2
	commitTestEvent(t, store, loaded, "req-two")
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	final, err := reopened.Load(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Batch.Revision != 2 || final.LastEvent == "" {
		t.Fatalf("unexpected recovered snapshot: %+v", final)
	}
	events, err := reopened.Events(batch.BatchID)
	if err != nil || len(events) != 2 {
		t.Fatalf("unexpected events: %d %v", len(events), err)
	}
}

func TestOpenRepairsOnlyValidUncommittedTail(t *testing.T) {
	root := t.TempDir()
	store, _ := Open(root)
	batch := &domain.DendroBatch{BatchID: "batch-tail", SiteCode: "S", Species: "P", OperatorID: "operator-x", State: domain.StateBaselined, Revision: 1, CreatedAt: time.Now().UTC()}
	snap := NewSnapshot(batch)
	commitTestEvent(t, store, snap, "req-one")
	loaded, _ := store.Load(batch.BatchID)
	extra := domain.AuditEvent{EventID: "extra-event", BatchID: batch.BatchID, Revision: 2, RequestID: "req-extra", Action: "TEST", ActorID: "actor-test", OccurredAt: time.Now().UTC(), PreviousDigest: loaded.LastEvent}
	if err := domain.FinalizeEvent(&extra); err != nil {
		t.Fatal(err)
	}
	line, _ := json.Marshal(extra)
	line = append(line, '\n')
	f, err := os.OpenFile(store.auditPath(batch.BatchID), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(line); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	reopened, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	events, err := reopened.Events(batch.BatchID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected repaired single event, got %d", len(events))
	}
}
