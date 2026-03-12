package middleware

import (
	"net/http"
	"strconv"

	"github.com/shengli/prism/sdk"
	"github.com/shengli/prism/sdk/propagation"
)

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.wrote {
		w.status = code
		w.wrote = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.status = 200
		w.wrote = true
	}
	return w.ResponseWriter.Write(b)
}

// HTTPServerMiddleware creates a middleware that automatically traces incoming HTTP requests.
func HTTPServerMiddleware(tracer *sdk.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			spanCtx := propagation.Extract(r.Header)
			operation := r.Method + " " + r.URL.Path

			opts := []sdk.SpanOption{sdk.WithKind(sdk.KindServer)}
			if spanCtx != nil {
				opts = append(opts, sdk.WithSpanContext(spanCtx))
			}

			ctx, span := tracer.StartSpan(r.Context(), operation, opts...)
			span.SetTag("http.method", r.Method)
			span.SetTag("http.url", r.URL.String())
			span.SetTag("http.host", r.Host)

			ww := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(ww, r.WithContext(ctx))

			span.SetTag("http.status_code", strconv.Itoa(ww.status))
			if ww.status >= 400 {
				span.Status = sdk.StatusError
			}
			tracer.FinishSpan(r.Context(), span)
		})
	}
}
