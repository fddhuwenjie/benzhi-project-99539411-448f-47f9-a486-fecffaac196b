package audit_integrity_error_chain_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/web"
	"dendro-chronology-workbench/internal/workflow"
)

func TestAuditIntegrityErrorChainPreserved(t *testing.T) {
	root := t.TempDir()
	store, err := repository.Open(root)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	service := workflow.New(store)
	_, err = service.Create(workflow.CreateBatchCommand{
		CommandMeta: workflow.CommandMeta{RequestID: "req-create", ActorID: "operator-1"},
		BatchID:     "batch-chain",
		SiteCode:    "SITE-CHAIN",
		Species:     "Pinus tabuliformis",
		SampledAt:   time.Now().UTC().Add(-time.Hour),
		OperatorID:  "operator-1",
		Cores: []domain.CoreSample{{
			CoreID: "core-1", TreeCode: "tree-1", RadiusCode: "A",
		}},
	})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	auditPath := filepath.Join(root, "audit", "batch-chain.jsonl")
	if err := os.WriteFile(auditPath, []byte("{\"broken\":true}\n"), 0o640); err != nil {
		t.Fatalf("corrupt audit fixture: %v", err)
	}

	_, serviceErr := service.Events("batch-chain")
	var integrityErr *domain.DomainError
	if !errors.As(serviceErr, &integrityErr) {
		t.Fatalf("wrapped repository error no longer exposes DomainError: %v", serviceErr)
	}
	if !domain.IsCode(serviceErr, domain.ErrIntegrity) {
		t.Errorf("domain.IsCode lost the wrapped integrity classification: %v", serviceErr)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/batches/batch-chain/events", nil)
	web.New(service, nil).Handler().ServeHTTP(recorder, request)
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if recorder.Code != http.StatusInternalServerError || response.Error.Code != string(domain.ErrIntegrity) {
		t.Errorf("wrapped integrity error mapped to status=%d code=%q, want status=500 code=%q", recorder.Code, response.Error.Code, domain.ErrIntegrity)
	}
}
