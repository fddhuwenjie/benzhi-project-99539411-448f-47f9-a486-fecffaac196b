package queuedcontextcancellation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/web"
	"dendro-chronology-workbench/internal/workflow"
)

type observedContext struct {
	context.Context
	checked chan struct{}
	once    sync.Once
}

func (c *observedContext) Err() error {
	c.once.Do(func() { close(c.checked) })
	return c.Context.Err()
}

func TestQueuedRequestCancellationPreventsCommit(t *testing.T) {
	root := t.TempDir()
	store, err := repository.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	service := workflow.New(store)
	now := time.Now().UTC()
	_, err = service.Create(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "request-create", ActorID: "operator-one"},
		BatchID:     "batch-cancel",
		SiteCode:    "SITE-CANCEL",
		Species:     "Pinus",
		SampledAt:   now.Add(-time.Hour),
		OperatorID:  "operator-one",
		Cores:       []domain.CoreSample{{CoreID: "core-one", TreeCode: "tree-one", RadiusCode: "A"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshotPath := filepath.Join(root, "snapshots", "batch-cancel.json")
	backupPath := filepath.Join(root, "snapshots", "batch-cancel.backup")
	if err := os.Rename(snapshotPath, backupPath); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(snapshotPath, 0o600); err != nil {
		t.Fatal(err)
	}

	images := []workflow.ImageInput{{
		CoreID:            "core-one",
		PreparationMethod: "sanded",
		ImageDigest:       "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		MicronsPerPixel:   2,
		CapturedAt:        now,
	}}
	firstDone := make(chan error, 1)
	go func() {
		_, callErr := service.RegisterImages("batch-cancel", workflow.RegisterImagesCommand{
			CommandMeta: workflow.CommandMeta{RequestID: "request-blocker", ExpectedRevision: 1, ActorID: "operator-one"},
			Images:      images,
		})
		firstDone <- callErr
	}()

	writer, err := os.OpenFile(snapshotPath, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}

	requestBody, err := json.Marshal(workflow.RegisterImagesCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "request-canceled", ExpectedRevision: 1, ActorID: "operator-one"},
		Images:      images,
	})
	if err != nil {
		t.Fatal(err)
	}
	baseContext, cancel := context.WithCancel(context.Background())
	trackedContext := &observedContext{Context: baseContext, checked: make(chan struct{})}
	request := httptest.NewRequest(http.MethodPost, "/api/batches/batch-cancel/images", bytes.NewReader(requestBody)).WithContext(trackedContext)
	recorder := httptest.NewRecorder()
	handlerDone := make(chan struct{})
	handler := web.New(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
	go func() {
		handler.ServeHTTP(recorder, request)
		close(handlerDone)
	}()

	<-trackedContext.checked
	cancel()
	if err := os.Remove(snapshotPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(backupPath, snapshotPath); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("{interrupted")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-firstDone; err == nil {
		t.Fatal("阻塞请求应因无效快照内容失败")
	}
	<-handlerDone

	loaded, err := store.Load("batch-cancel")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Batch.Revision != 1 || loaded.Batch.State != domain.StateBaselined {
		t.Fatalf("已取消的排队请求仍完成持久化：status=%d revision=%d state=%s", recorder.Code, loaded.Batch.Revision, loaded.Batch.State)
	}
}
