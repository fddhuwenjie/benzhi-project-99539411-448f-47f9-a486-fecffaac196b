package web

import (
	"net/http"

	"dendro-chronology-workbench/internal/domain"
	"dendro-chronology-workbench/internal/workflow"
)

func emitResult(w http.ResponseWriter, result workflow.Result, err error) {
	if err != nil {
		handleError(w, err)
		return
	}
	writeRaw(w, result.StatusCode, result.Body, result.Replayed)
}

func (s *Server) HandleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.CreateBatchCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.Create(cmd)
	emitResult(w, result, err)
}
func (s *Server) HandleImages(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.RegisterImagesCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.RegisterImagesContext(r.Context(), r.PathValue("batch_id"), cmd)
	emitResult(w, result, err)
}
func (s *Server) HandleObservations(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.SubmitObservationsCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.SubmitObservations(r.PathValue("batch_id"), cmd)
	emitResult(w, result, err)
}
func (s *Server) HandleValidate(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ValidateCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.Validate(r.PathValue("batch_id"), cmd)
	emitResult(w, result, err)
}
func (s *Server) HandleCorrection(w http.ResponseWriter, r *http.Request) {
	type correctionRequest struct {
		workflow.CommandMeta
		FindingID   string                    `json:"finding_id,omitempty"`
		Reason      string                    `json:"reason,omitempty"`
		Replacement workflow.Replacement      `json:"replacement,omitempty"`
		Items       []workflow.CorrectionItem `json:"items,omitempty"`
	}
	var input correctionRequest
	if err := decodeJSONLimit(w, r, &input, 256<<10); err != nil {
		handleError(w, err)
		return
	}
	var result workflow.Result
	var err error
	if len(input.Items) > 0 {
		if input.FindingID != "" || input.Reason != "" {
			handleError(w, domain.NewError(domain.ErrValidation, "items", "批量 items 不能与单项整改字段同时提交"))
			return
		}
		result, err = s.workflow.CorrectFindings(r.PathValue("batch_id"), workflow.CorrectFindingsCommand{CommandMeta: input.CommandMeta, Items: input.Items})
	} else {
		result, err = s.workflow.CorrectFinding(r.PathValue("batch_id"), workflow.CorrectFindingCommand{CommandMeta: input.CommandMeta, FindingID: input.FindingID, Reason: input.Reason, Replacement: input.Replacement})
	}
	emitResult(w, result, err)
}
func (s *Server) HandleReview(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.ReviewCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	if cmd.InspectedRevision == nil {
		handleError(w, domain.NewError(domain.ErrValidation, "inspected_revision", "复核提交必须引用预检取得的 inspected_revision"))
		return
	}
	result, err := s.workflow.Review(r.PathValue("batch_id"), cmd)
	emitResult(w, result, err)
}
func (s *Server) HandleSeal(w http.ResponseWriter, r *http.Request) {
	var cmd workflow.SealCommand
	if err := decodeJSON(w, r, &cmd); err != nil {
		handleError(w, err)
		return
	}
	result, err := s.workflow.Seal(r.PathValue("batch_id"), cmd)
	emitResult(w, result, err)
}
