package observability

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

func TestNewTracerProviderSetsConfiguredServiceName(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewTracerProvider(context.Background(), TracingConfig{Enabled: true, SampleRatio: 1, ServiceName: "omnicraft-worker"}, exporter)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())
	_, span := provider.Tracer("test").Start(context.Background(), "operation")
	span.End()
	require.NoError(t, provider.ForceFlush(context.Background()))
	var serviceName string
	for _, kv := range exporter.GetSpans()[0].Resource.Attributes() {
		if kv.Key == attribute.Key("service.name") {
			serviceName = kv.Value.AsString()
		}
	}
	require.Equal(t, "omnicraft-worker", serviceName)
}
