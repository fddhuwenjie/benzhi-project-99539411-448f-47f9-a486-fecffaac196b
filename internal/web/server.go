package web

import (
	"log/slog"
	"net/http"

	"dendro-chronology-workbench/internal/workflow"
)

type Server struct {
	workflow *workflow.Service
	logger   *slog.Logger
	mux      *http.ServeMux
}

func New(service *workflow.Service, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{workflow: service, logger: logger, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return securityHeaders(recoverer(s.logger, s.mux)) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /", s.HandleWorkbench)
	s.mux.HandleFunc("GET /static/app.css", s.HandleCSS)
	s.mux.HandleFunc("GET /static/app.js", s.HandleJS)
	s.mux.HandleFunc("GET /api/health", s.HandleHealth)
	s.mux.HandleFunc("GET /api/batches", s.HandleListBatches)
	s.mux.HandleFunc("POST /api/batches", s.HandleCreateBatch)
	s.mux.HandleFunc("GET /api/batches/{batch_id}", s.HandleGetBatch)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/events", s.HandleEvents)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/review-inspection", s.HandleReviewInspection)
	s.mux.HandleFunc("GET /api/batches/{batch_id}/manifest", s.HandleManifest)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/images", s.HandleImages)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/observations", s.HandleObservations)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/validate", s.HandleValidate)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/corrections", s.HandleCorrection)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/review", s.HandleReview)
	s.mux.HandleFunc("POST /api/batches/{batch_id}/seal", s.HandleSeal)
}
