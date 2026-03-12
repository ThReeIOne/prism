package query

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shengli/prism/internal/storage"
)

// TraceResponse is the JSON shape of a trace.
type TraceResponse struct {
	TraceID string         `json:"trace_id"`
	Spans   []SpanResponse `json:"spans"`
}

// SpanResponse is the JSON shape of a single span.
type SpanResponse struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Operation    string            `json:"operation"`
	Service      string            `json:"service"`
	Kind         string            `json:"kind"`
	StartUs      uint64            `json:"start_us"`
	DurationUs   uint64            `json:"duration_us"`
	Status       string            `json:"status"`
	Tags         map[string]string `json:"tags,omitempty"`
	Events       string            `json:"events,omitempty"`
}

func spanRecordToResponse(sr storage.SpanRecord) SpanResponse {
	return SpanResponse{
		TraceID:      sr.TraceID,
		SpanID:       sr.SpanID,
		ParentSpanID: sr.ParentSpanID,
		Operation:    sr.Operation,
		Service:      sr.Service,
		Kind:         sr.Kind,
		StartUs:      sr.StartUs,
		DurationUs:   sr.DurationUs,
		Status:       sr.Status,
		Tags:         sr.Tags,
		Events:       sr.Events,
	}
}

func (s *Server) getTrace(w http.ResponseWriter, r *http.Request) {
	traceID := chi.URLParam(r, "traceID")
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "traceID is required")
		return
	}

	result, err := s.store.GetTrace(r.Context(), traceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}

	resp := TraceResponse{
		TraceID: result.TraceID,
		Spans:   make([]SpanResponse, 0, len(result.Spans)),
	}
	for _, sp := range result.Spans {
		resp.Spans = append(resp.Spans, spanRecordToResponse(sp))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) searchTraces(w http.ResponseWriter, r *http.Request) {
	params, err := parseSearchParams(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	results, err := s.store.SearchTraces(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	traces := make([]TraceResponse, 0, len(results))
	for _, tr := range results {
		resp := TraceResponse{
			TraceID: tr.TraceID,
			Spans:   make([]SpanResponse, 0, len(tr.Spans)),
		}
		for _, sp := range tr.Spans {
			resp.Spans = append(resp.Spans, spanRecordToResponse(sp))
		}
		traces = append(traces, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"traces": traces,
		"total":  len(traces),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
