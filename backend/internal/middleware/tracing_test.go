package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTracingMiddlewareKeepsRequestIDSeparateFromOTelTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	exporter := tracetest.NewInMemoryExporter()
	provider := trace.NewTracerProvider(trace.WithSyncer(exporter))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	router := gin.New()
	router.Use(RequestID(), Tracing(provider))
	router.GET("/trace", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"request_id": c.GetString("request_id"),
			"trace_id":   c.GetString("trace_id"),
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, exporter.GetSpans(), 1)
	require.NotEmpty(t, rec.Header().Get("X-Request-ID"))
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, rec.Header().Get("X-Request-ID"), body["request_id"])
	require.Equal(t, exporter.GetSpans()[0].SpanContext.TraceID().String(), body["trace_id"])
	require.NotEqual(t, body["request_id"], body["trace_id"])
}
