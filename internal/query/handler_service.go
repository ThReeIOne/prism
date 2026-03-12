package query

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getServices(w http.ResponseWriter, r *http.Request) {
	var lookback time.Duration // zero means no time filter
	if lb := r.URL.Query().Get("lookback"); lb != "" {
		if d, err := time.ParseDuration(lb); err == nil && d > 0 {
			lookback = d
		}
	}

	services, err := s.store.GetServices(r.Context(), lookback)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"services": services,
	})
}

func (s *Server) getOperations(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "service name is required")
		return
	}

	ops, err := s.store.GetOperations(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"service":    name,
		"operations": ops,
	})
}
