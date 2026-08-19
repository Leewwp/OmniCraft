package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewTracerProviderValidatesHeadRatioAndExportsSpans(t *testing.T) {
	ctx := context.Background()
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewTracerProvider(ctx, TracingConfig{Enabled: true, SampleRatio: 1}, exporter)
	require.NoError(t, err)

	tracer := provider.Tracer("omnicraft/test")
	_, span := tracer.Start(ctx, "test.operation")
	span.End()
	require.NoError(t, provider.ForceFlush(ctx))
	require.Len(t, exporter.GetSpans(), 1)
	require.NotEmpty(t, exporter.GetSpans()[0].SpanContext.TraceID())
	require.NoError(t, provider.Shutdown(ctx))

	_, err = NewTracerProvider(ctx, TracingConfig{Enabled: true, SampleRatio: 1.1}, exporter)
	require.Error(t, err)
	_ = otel.GetTracerProvider()
}
