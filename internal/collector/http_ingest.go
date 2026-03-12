package collector

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/shengli/prism/internal/metrics"
	"github.com/shengli/prism/internal/storage"
)

// HTTPSpan is the JSON representation of a span for the HTTP ingest API.
type HTTPSpan struct {
	TraceID      string            `json:"trace_id"`
	SpanID       string            `json:"span_id"`
	ParentSpanID string            `json:"parent_span_id,omitempty"`
	Operation    string            `json:"operation"`
	Service      string            `json:"service"`
	Kind         string            `json:"kind"`
	StartUs      uint64            `json:"start_us,omitempty"`
	StartTime    string            `json:"start_time,omitempty"` // RFC3339, alternative to start_us
	DurationUs   uint64            `json:"duration_us,omitempty"`
	DurationMs   float64           `json:"duration_ms,omitempty"` // alternative to duration_us
	Status       string            `json:"status"`
	Tags         map[string]string `json:"tags,omitempty"`
	Events       json.RawMessage   `json:"events,omitempty"`
}

// HTTPIngestHandler returns an http.Handler that accepts spans via JSON POST.
//
//	POST /api/v1/spans
//	Content-Type: application/json
//	Body: [{"trace_id":"...","span_id":"...","service":"my-app",...}]
//
// token is the optional bearer token to require (empty = no auth).
// origins is the value for Access-Control-Allow-Origin (e.g. "*").
func (c *Collector) HTTPIngestHandler(token, origins string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			writeCORS(w, origins)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			writeCORS(w, origins)
			writeJSONError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}

		// Bearer token auth (optional)
		if token != "" {
			auth := r.Header.Get("Authorization")
			expected := "Bearer " + token
			if !strings.EqualFold(strings.TrimSpace(auth), strings.TrimSpace(expected)) {
				writeCORS(w, origins)
				writeJSONError(w, http.StatusUnauthorized, "invalid or missing bearer token")
				return
			}
		}

		var spans []HTTPSpan
		if err := json.NewDecoder(r.Body).Decode(&spans); err != nil {
			writeCORS(w, origins)
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
			return
		}

		if len(spans) == 0 {
			writeCORS(w, origins)
			writeJSONError(w, http.StatusBadRequest, "empty span list")
			return
		}

		// Validate and convert
		records := make([]storage.SpanRecord, 0, len(spans))
		for i, sp := range spans {
			if sp.TraceID == "" || sp.SpanID == "" || sp.Service == "" {
				writeCORS(w, origins)
				writeJSONError(w, http.StatusBadRequest,
					fmt.Sprintf("span[%d]: trace_id, span_id, and service are required", i))
				return
			}
			records = append(records, httpSpanToRecord(sp))
		}

		// Buffer the spans (reuse the same flush path as gRPC)
		if full := c.bufferRecords(records); full {
			writeCORS(w, origins)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]string{"error": "collector buffer full"})
			return
		}

		metrics.SpansReceived.Add(float64(len(records)))
		metrics.SDKReportBatchSize.Observe(float64(len(records)))

		writeCORS(w, origins)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"accepted": len(records),
		})
	})
}

func httpSpanToRecord(sp HTTPSpan) storage.SpanRecord {
	startUs := sp.StartUs
	if startUs == 0 && sp.StartTime != "" {
		if t, err := time.Parse(time.RFC3339Nano, sp.StartTime); err == nil {
			startUs = uint64(t.UnixMicro())
		}
	}
	if startUs == 0 {
		startUs = uint64(time.Now().UnixMicro())
	}

	durationUs := sp.DurationUs
	if durationUs == 0 && sp.DurationMs > 0 {
		durationUs = uint64(sp.DurationMs * 1000)
	}

	kind := sp.Kind
	if kind == "" {
		kind = "INTERNAL"
	}

	status := sp.Status
	if status == "" {
		status = "ok"
	}

	return storage.SpanRecord{
		TraceID:      sp.TraceID,
		SpanID:       sp.SpanID,
		ParentSpanID: sp.ParentSpanID,
		Operation:    sp.Operation,
		Service:      sp.Service,
		Kind:         kind,
		StartUs:      startUs,
		DurationUs:   durationUs,
		Status:       status,
		Tags:         sp.Tags,
		Events:       string(sp.Events),
	}
}

func writeCORS(w http.ResponseWriter, origins string) {
	if origins == "" {
		origins = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origins)
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
