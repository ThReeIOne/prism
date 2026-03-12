package query

import (
	"crypto/subtle"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shengli/prism/internal/metrics"
	"github.com/shengli/prism/internal/storage"
	"github.com/shengli/prism/web"
)

// Server is the Query API server.
type Server struct {
	store       storage.Storage
	router      chi.Router
	token       string // optional bearer token; empty = no auth
	corsOrigins string // value for Access-Control-Allow-Origin
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithToken sets the optional bearer token for /api/v1/* routes.
func WithToken(token string) ServerOption {
	return func(s *Server) {
		s.token = token
	}
}

// WithCORSOrigins sets the allowed origin for CORS responses.
func WithCORSOrigins(origins string) ServerOption {
	return func(s *Server) {
		s.corsOrigins = origins
	}
}

// NewServer creates a new Query API server.
func NewServer(store storage.Storage, opts ...ServerOption) *Server {
	s := &Server{
		store: store,
	}
	for _, o := range opts {
		o(s)
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(s.corsMiddleware)
	r.Use(metricsMiddleware)

	r.Route("/api/v1", func(r chi.Router) {
		// Apply bearer token auth if configured
		if s.token != "" {
			r.Use(s.authMiddleware)
		}

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

	// Prometheus metrics
	r.Handle("/metrics", promhttp.Handler())

	// Serve embedded frontend SPA
	s.mountFrontend(r)

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

// corsMiddleware sets CORS headers using the configured origins.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	origins := s.corsOrigins
	if origins == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates the Authorization: Bearer <token> header.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.token
		if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid or missing bearer token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics for /metrics and /health
		if r.URL.Path == "/metrics" || r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		rec := &responseRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)
		duration := time.Since(start).Seconds()

		endpoint := chi.RouteContext(r.Context()).RoutePattern()
		if endpoint == "" {
			endpoint = r.URL.Path
		}
		status := strconv.Itoa(rec.status)

		metrics.QueryDuration.WithLabelValues(endpoint, status).Observe(duration)
		metrics.QueryRequests.WithLabelValues(endpoint, status).Inc()
	})
}

func (s *Server) mountFrontend(r chi.Router) {
	// Serve embedded frontend from web/dist
	distFS, err := fs.Sub(web.Assets, "dist")
	if err != nil {
		slog.Warn("frontend assets not available", "error", err)
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	// Catch-all: serve static files or fall back to index.html for SPA routing
	r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to open the file from embedded FS
		f, err := distFS.Open(path)
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fall back to index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
