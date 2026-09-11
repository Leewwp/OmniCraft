package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Tracing creates the HTTP server span and places its context back on the Gin
// request so DB, queue, LLM and SSE code all observe the same trace. The
// independent request_id middleware remains responsible for X-Request-ID.
func Tracing(provider *trace.TracerProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		parent := propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		).Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		tracer := otel.Tracer("omnicraft/http")
		if provider != nil {
			tracer = provider.Tracer("omnicraft/http")
		}
		ctx, span := tracer.Start(parent, c.Request.Method+" "+c.Request.URL.Path, oteltrace.WithSpanKind(oteltrace.SpanKindServer))
		defer span.End()

		c.Request = c.Request.WithContext(ctx)
		traceID := span.SpanContext().TraceID().String()
		c.Set("trace_id", traceID)
		span.SetAttributes(
			attribute.String("http.request.method", c.Request.Method),
			attribute.String("url.path", c.Request.URL.Path),
		)

		c.Next()
		span.SetAttributes(attribute.Int("http.response.status_code", c.Writer.Status()))
		if c.Writer.Status() >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(c.Writer.Status()))
		}
	}
}
