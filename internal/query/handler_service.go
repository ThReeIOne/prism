package query

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getServices(w http.ResponseWriter, r *http.Request) {
	services, err := s.store.GetServices(r.Context())
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
