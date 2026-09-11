package queue

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestTraceContextRoundTripsThroughQueueMetadata(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	tracer := provider.Tracer("queue-test")
	ctx, span := tracer.Start(context.Background(), "producer")
	defer span.End()

	metadata := map[string]string{}
	InjectTraceContext(ctx, metadata)
	require.NotEmpty(t, metadata["traceparent"])

	consumerCtx := ExtractTraceContext(context.Background(), metadata)
	require.Equal(t, oteltrace.SpanContextFromContext(ctx).TraceID(), oteltrace.SpanContextFromContext(consumerCtx).TraceID())
	require.Equal(t, oteltrace.SpanContextFromContext(ctx).SpanID(), oteltrace.SpanContextFromContext(consumerCtx).SpanID())
	require.NotContains(t, metadata, "prompt")
}
