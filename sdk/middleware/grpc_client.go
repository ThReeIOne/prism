package middleware

import (
	"context"

	"github.com/shengli/prism/sdk"
	"github.com/shengli/prism/sdk/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// UnaryClientInterceptor returns a gRPC unary client interceptor that creates client spans.
func UnaryClientInterceptor(tracer *sdk.Tracer) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx, span := tracer.StartSpan(ctx, method, sdk.WithKind(sdk.KindClient))
		defer tracer.FinishSpan(ctx, span)

		span.SetTag("rpc.system", "grpc")
		span.SetTag("peer.service", cc.Target())

		ctx = propagation.InjectGRPC(ctx, span)

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			span.SetError(err)
			if st, ok := status.FromError(err); ok {
				span.SetTag("rpc.grpc.status_code", st.Code().String())
			}
		} else {
			span.SetTag("rpc.grpc.status_code", "OK")
		}
		return err
	}
}
