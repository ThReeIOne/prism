package propagation

import (
	"context"

	"github.com/shengli/prism/sdk"
	"google.golang.org/grpc/metadata"
)

const (
	GRPCTraceID = "x-prism-trace-id"
	GRPCSpanID  = "x-prism-span-id"
	GRPCSampled = "x-prism-sampled"
)

// InjectGRPC writes trace context into gRPC outgoing metadata.
func InjectGRPC(ctx context.Context, span *sdk.Span) context.Context {
	if span == nil {
		return ctx
	}
	md := metadata.Pairs(
		GRPCTraceID, span.TraceID,
		GRPCSpanID, span.SpanID,
		GRPCSampled, "1",
	)
	return metadata.NewOutgoingContext(ctx, md)
}

// ExtractGRPC reads trace context from gRPC incoming metadata.
func ExtractGRPC(ctx context.Context) *sdk.SpanContext {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	traceIDs := md.Get(GRPCTraceID)
	if len(traceIDs) == 0 {
		return nil
	}
	sc := &sdk.SpanContext{
		TraceID: traceIDs[0],
	}
	if spanIDs := md.Get(GRPCSpanID); len(spanIDs) > 0 {
		sc.ParentSpanID = spanIDs[0]
	}
	if sampled := md.Get(GRPCSampled); len(sampled) > 0 {
		sc.Sampled = sampled[0] == "1"
	}
	return sc
}
