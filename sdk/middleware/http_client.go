package middleware

import (
	"net/http"
	"strconv"

	"github.com/shengli/prism/sdk"
	"github.com/shengli/prism/sdk/propagation"
)

// TracedTransport wraps an http.RoundTripper to automatically create client spans
// and propagate trace context.
type TracedTransport struct {
	Tracer  *sdk.Tracer
	Wrapped http.RoundTripper
}

func (t *TracedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, span := t.Tracer.StartSpan(req.Context(),
		req.Method+" "+req.URL.Host+req.URL.Path,
		sdk.WithKind(sdk.KindClient),
	)
	defer t.Tracer.FinishSpan(span)

	propagation.Inject(span, req.Header)
	span.SetTag("http.method", req.Method)
	span.SetTag("peer.service", req.URL.Host)
	span.SetTag("http.url", req.URL.String())

	resp, err := t.Wrapped.RoundTrip(req.WithContext(ctx))
	if err != nil {
		span.SetError(err)
		return nil, err
	}
	span.SetTag("http.status_code", strconv.Itoa(resp.StatusCode))
	if resp.StatusCode >= 400 {
		span.Status = sdk.StatusError
	}
	return resp, nil
}
