package web

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/workflow"
)

func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) HandleListBatches(w http.ResponseWriter, r *http.Request) {
	filter, err := parseBatchFilter(r)
	if err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.SearchBatches(filter)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func parseBatchFilter(r *http.Request) (workflow.BatchFilter, error) {
	allowed := map[string]bool{"status": true, "site_code": true, "species": true, "operator_id": true, "batch_id": true}
	for key, values := range r.URL.Query() {
		if !allowed[key] {
			return workflow.BatchFilter{}, domain.NewError(domain.ErrValidation, key, "未知批次筛选字段 %s", key)
		}
		if len(values) != 1 {
			return workflow.BatchFilter{}, domain.NewError(domain.ErrValidation, key, "筛选字段 %s 不能提供互相矛盾的多个值", key)
		}
	}
	value := func(key string) string { return strings.TrimSpace(r.URL.Query().Get(key)) }
	status := domain.State(value("status"))
	if status != "" && !validState(status) {
		return workflow.BatchFilter{}, domain.NewError(domain.ErrValidation, "status", "未知批次状态 %s", status)
	}
	keywords := map[string]string{"site_code": value("site_code"), "species": value("species"), "operator_id": value("operator_id"), "batch_id": value("batch_id")}
	for field, keyword := range keywords {
		if utf8.RuneCountInString(keyword) > 64 {
			return workflow.BatchFilter{}, domain.NewError(domain.ErrValidation, field, "%s 筛选关键词不能超过 64 个字符", field)
		}
	}
	return workflow.BatchFilter{State: status, SiteCode: keywords["site_code"], Species: keywords["species"], OperatorID: keywords["operator_id"], BatchID: keywords["batch_id"]}, nil
}

func validState(state domain.State) bool {
	switch state {
	case domain.StateDraft, domain.StateBaselined, domain.StateImaged, domain.StateAnalyzed, domain.StateCorrectionRequired, domain.StateReviewReady, domain.StateVerified, domain.StateSealed:
		return true
	default:
		return false
	}
}

func (s *Server) HandleGetBatch(w http.ResponseWriter, r *http.Request) {
	batch, err := s.workflow.Get(r.PathValue("batch_id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batch": batch})
}

func (s *Server) HandleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.workflow.Events(r.PathValue("batch_id"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) HandleReviewInspection(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query) != 1 || len(query["reviewer_id"]) != 1 {
		handleError(w, domain.NewError(domain.ErrValidation, "reviewer_id", "必须提供唯一的 reviewer_id"))
		return
	}
	report, err := s.workflow.ReviewInspection(r.PathValue("batch_id"), strings.TrimSpace(query.Get("reviewer_id")))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"inspection": report})
}

func (s *Server) HandleManifest(w http.ResponseWriter, r *http.Request) {
	manifest, err := s.workflow.Manifest(r.PathValue("batch_id"))
	if err != nil {
		handleError(w, err)
		return
	}
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+manifest.BatchID+"-manifest.json\"")
	}
	writeJSON(w, http.StatusOK, manifest)
}
