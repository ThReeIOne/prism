package query

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/shengli/prism/internal/storage"
)

// Server is the Query API server.
type Server struct {
	store  storage.Storage
	router chi.Router
}

// NewServer creates a new Query API server.
func NewServer(store storage.Storage) *Server {
	s := &Server{store: store}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(corsMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		// Trace endpoints
		r.Get("/traces/{traceID}", s.getTrace)
		r.Get("/traces", s.searchTraces)

		// Service endpoints
		r.Get("/services", s.getServices)
		r.Get("/services/{name}/operations", s.getOperations)

		// Dependency endpoints
		r.Get("/dependencies", s.getDependencies)

		// Stats endpoints
		r.Get("/stats/latency", s.getLatencyStats)
		r.Get("/stats/throughput", s.getThroughputStats)
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	s.router = r
}

// Handler returns the http.Handler for the server.
func (s *Server) Handler() http.Handler {
	return s.router
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(addr string) error {
	slog.Info("query server starting", "addr", addr)
	return http.ListenAndServe(addr, s.router)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
