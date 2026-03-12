package query

import (
	"net/http"
	"time"
)

func (s *Server) getLatencyStats(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	operation := r.URL.Query().Get("operation")

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	granularity := parseGranularity(r.URL.Query().Get("granularity"))

	points, err := s.store.GetLatencyStats(r.Context(), service, operation, start, end, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service":     service,
		"operation":   operation,
		"granularity": granularity.String(),
		"data":        points,
	})
}

func (s *Server) getThroughputStats(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")

	start, end, err := parseTimeRange(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	granularity := parseGranularity(r.URL.Query().Get("granularity"))

	points, err := s.store.GetThroughputStats(r.Context(), service, start, end, granularity)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"service":     service,
		"granularity": granularity.String(),
		"data":        points,
	})
}

func parseGranularity(s string) time.Duration {
	if s == "" {
		return time.Minute
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Minute
	}
	return d
}
