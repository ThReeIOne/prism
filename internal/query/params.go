package query

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/shengli/prism/internal/storage"
)

func parseSearchParams(r *http.Request) (storage.TraceSearchParams, error) {
	q := r.URL.Query()
	params := storage.TraceSearchParams{
		Service:   q.Get("service"),
		Operation: q.Get("operation"),
		Status:    q.Get("status"),
	}

	if v := q.Get("min_duration"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return params, fmt.Errorf("invalid min_duration: %w", err)
		}
		params.MinDuration = d
	}

	if v := q.Get("max_duration"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return params, fmt.Errorf("invalid max_duration: %w", err)
		}
		params.MaxDuration = d
	}

	if v := q.Get("start"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return params, fmt.Errorf("invalid start: %w", err)
		}
		params.Start = t
	}

	if v := q.Get("end"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return params, fmt.Errorf("invalid end: %w", err)
		}
		params.End = t
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return params, fmt.Errorf("invalid limit: %w", err)
		}
		params.Limit = n
	} else {
		params.Limit = 20
	}

	return params, nil
}
