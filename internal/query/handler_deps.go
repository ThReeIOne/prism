package query

import (
	"net/http"
	"sort"
	"time"
)

func (s *Server) getDependencies(w http.ResponseWriter, r *http.Request) {
	var start, end time.Time

	// Only apply time filter when explicitly provided
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		var err error
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		var err error
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	deps, err := s.store.GetDependencies(r.Context(), start, end)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build node set from edges
	nodeSet := make(map[string]bool)
	for _, d := range deps {
		nodeSet[d.Parent] = true
		nodeSet[d.Child] = true
	}
	nodes := make([]map[string]string, 0, len(nodeSet))
	for name := range nodeSet {
		nodes = append(nodes, map[string]string{"id": name, "label": name})
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i]["id"] < nodes[j]["id"]
	})

	edges := make([]map[string]any, 0, len(deps))
	for _, d := range deps {
		edges = append(edges, map[string]any{
			"source":      d.Parent,
			"target":      d.Child,
			"call_count":  d.CallCount,
			"error_count": d.ErrorCount,
			"avg_latency": d.AvgLatency,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nodes": nodes,
		"edges": edges,
	})
}

func parseTimeRange(r *http.Request) (time.Time, time.Time, error) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var start, end time.Time
	var err error

	if startStr != "" {
		start, err = time.Parse(time.RFC3339, startStr)
		if err != nil {
			return start, end, err
		}
	} else {
		start = time.Now().Add(-1 * time.Hour)
	}

	if endStr != "" {
		end, err = time.Parse(time.RFC3339, endStr)
		if err != nil {
			return start, end, err
		}
	} else {
		end = time.Now()
	}

	return start, end, nil
}
