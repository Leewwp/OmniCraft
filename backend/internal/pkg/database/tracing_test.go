package database

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

type tracingRecord struct {
	ID    uint `gorm:"primaryKey"`
	Value string
}

func TestGORMTracingEmitsDBSpanWithoutQueryValues(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Use(tracing.NewPlugin(
		tracing.WithTracerProvider(provider),
		tracing.WithoutQueryVariables(),
	)))
	require.NoError(t, db.AutoMigrate(&tracingRecord{}))

	ctx, span := provider.Tracer("database-test").Start(context.Background(), "request")
	require.NoError(t, db.WithContext(ctx).Create(&tracingRecord{Value: "database-secret"}).Error)
	span.End()
	require.NoError(t, provider.ForceFlush(context.Background()))

	require.NotEmpty(t, exporter.GetSpans())
	for _, exported := range exporter.GetSpans() {
		for _, attr := range exported.Attributes {
			require.NotContains(t, attr.Value.AsString(), "database-secret")
		}
	}
}
