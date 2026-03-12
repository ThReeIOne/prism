package middleware

import (
	"context"

	"github.com/shengli/prism/sdk"
	"github.com/shengli/prism/sdk/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor returns a gRPC unary server interceptor that creates server spans.
func UnaryServerInterceptor(tracer *sdk.Tracer) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		spanCtx := propagation.ExtractGRPC(ctx)

		opts := []sdk.SpanOption{sdk.WithKind(sdk.KindServer)}
		if spanCtx != nil {
			opts = append(opts, sdk.WithSpanContext(spanCtx))
		}

		ctx, span := tracer.StartSpan(ctx, info.FullMethod, opts...)
		span.SetTag("rpc.system", "grpc")

		resp, err := handler(ctx, req)
		if err != nil {
			span.SetError(err)
			if st, ok := status.FromError(err); ok {
				span.SetTag("rpc.grpc.status_code", st.Code().String())
			}
		} else {
			span.SetTag("rpc.grpc.status_code", "OK")
		}
		tracer.FinishSpan(ctx, span)
		return resp, err
	}
}
