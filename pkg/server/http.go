package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/spectra-ai/spectra/pkg/index"
	"github.com/spectra-ai/spectra/pkg/memindex"
	"github.com/spectra-ai/spectra/pkg/meta"
	"github.com/spectra-ai/spectra/pkg/model"
	"github.com/spectra-ai/spectra/pkg/query"
	"github.com/spectra-ai/spectra/pkg/segment"
	"github.com/spectra-ai/spectra/pkg/wal"
)

// HTTPServer serves the Spectra REST API.
type HTTPServer struct {
	walWriter *wal.Writer
	memIdx    *memindex.MemIndex
	planner   *query.Planner
	logger    *zap.Logger
	server    *http.Server
}

// NewHTTPServer creates a new HTTP server.
func NewHTTPServer(addr string, walWriter *wal.Writer, memIdx *memindex.MemIndex, walReader *wal.Reader, metaStore meta.MetaStore, segReader *segment.Reader, indexer *index.Indexer, logger *zap.Logger) *HTTPServer {
	planner := query.NewPlanner(memIdx, walReader, metaStore, segReader, indexer)

	s := &HTTPServer{
		walWriter: walWriter,
		memIdx:    memIdx,
		planner:   planner,
		logger:    logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/spans", s.handleIngest)
	mux.HandleFunc("GET /v1/traces/{traceID}", s.handleGetTrace)
	mux.HandleFunc("POST /v1/search", s.handleSearch)
	mux.HandleFunc("POST /v1/query", s.handleQuery)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s
}

// Start starts the HTTP server.
func (s *HTTPServer) Start() error {
	s.logger.Info("starting HTTP server", zap.String("addr", s.server.Addr))
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *HTTPServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func (s *HTTPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type ingestRequest struct {
	Spans []model.Span `json:"spans"`
}

type ingestResponse struct {
	Ingested int `json:"ingested"`
}

func (s *HTTPServer) handleIngest(w http.ResponseWriter, r *http.Request) {
	var req ingestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	ingested := 0
	for i := range req.Spans {
		span := &req.Spans[i]
		if err := span.Validate(); err != nil {
			s.logger.Warn("invalid span", zap.String("span_id", span.SpanID), zap.Error(err))
			continue
		}

		entry, err := s.walWriter.Append(ctx, span)
		if err != nil {
			s.logger.Error("failed to append span", zap.Error(err))
			continue
		}

		// Update memindex for immediate reads
		s.memIdx.Add(span.TraceID, memindex.Location{
			WALKey:    "", // will be set after flush
			Timestamp: time.Now(),
		})
		_ = entry
		ingested++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ingestResponse{Ingested: ingested})
}

func (s *HTTPServer) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("traceID")
	if traceID == "" {
		http.Error(w, `{"error":"trace_id is required"}`, http.StatusBadRequest)
		return
	}

	q := model.QueryRequest{
		Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: traceID}},
		Limit:   1000,
	}

	plan := s.planner.CreatePlan(q)
	result, err := s.planner.Execute(r.Context(), plan)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

type searchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (s *HTTPServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req searchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if req.Limit <= 0 {
		req.Limit = 50
	}

	q := model.QueryRequest{
		Search: req.Query,
		Limit:  req.Limit,
	}

	plan := s.planner.CreatePlan(q)
	result, err := s.planner.Execute(r.Context(), plan)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *HTTPServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	var q model.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if err := q.Validate(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	plan := s.planner.CreatePlan(q)
	result, err := s.planner.Execute(r.Context(), plan)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
