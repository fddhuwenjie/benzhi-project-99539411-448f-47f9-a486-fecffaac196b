package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dendro-chronology-workbench/internal/repository"
	"dendro-chronology-workbench/internal/workflow"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := repository.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return New(workflow.New(store), nil).Handler()
}

func TestWorkbenchAndSecurityHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<body>") {
		t.Fatal("missing full HTML body")
	}
	if rec.Header().Get("Content-Security-Policy") == "" {
		t.Fatal("missing CSP")
	}
}

func TestUnknownJSONFieldRejected(t *testing.T) {
	body := `{"request_id":"req-web","expected_revision":0,"actor_id":"operator-web","batch_id":"batch-web","site_code":"S","species":"P","sampled_at":"2025-01-01T00:00:00Z","operator_id":"operator-web","cores":[{"core_id":"core-web","tree_code":"T","radius_code":"A"}],"surprise":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
		t.Fatal("missing stable error code")
	}
}

func TestBatchFilterValidationAndEmptyStatistics(t *testing.T) {
	handler := testHandler(t)
	for _, target := range []string{"/api/batches?status=UNKNOWN", "/api/batches?site_code=" + strings.Repeat("长", 65), "/api/batches?status=SEALED&status=BASELINED"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "VALIDATION_ERROR") {
			t.Fatalf("%s expected stable validation error, got %d: %s", target, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/api/batches?site_code=no-match", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("empty filter failed: %d %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Batches    []any `json:"batches"`
		Statistics struct {
			Total        int            `json:"total"`
			StatusCounts map[string]int `json:"status_counts"`
		} `json:"statistics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Batches == nil || len(response.Batches) != 0 || response.Statistics.Total != 0 || len(response.Statistics.StatusCounts) == 0 {
		t.Fatalf("empty result contract is incomplete: %+v", response)
	}
}
