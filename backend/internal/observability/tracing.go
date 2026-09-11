package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// TracingConfig controls application tracing. Sampling is deliberately
// head-based: the decision is made once at the root span and is inherited by
// every child span in the request or worker context.
type TracingConfig struct {
	Enabled     bool    `mapstructure:"enabled"`
	Endpoint    string  `mapstructure:"endpoint"`
	SampleRatio float64 `mapstructure:"sample_ratio"`
	Backend     string  `mapstructure:"backend"`
	ServiceName string  `mapstructure:"service_name"`
}

// NewTracerProvider builds a provider using an explicitly supplied exporter
// (primarily for tests) or the configured OTLP/gRPC exporter. A missing or
// unavailable collector is intentionally not a startup or request error; the
// SDK drops telemetry after its bounded export queue is exhausted.
func NewTracerProvider(ctx context.Context, cfg TracingConfig, exporters ...sdktrace.SpanExporter) (*sdktrace.TracerProvider, error) {
	if cfg.SampleRatio < 0 || cfg.SampleRatio > 1 {
		return nil, fmt.Errorf("observability.tracing.sample_ratio must be between 0 and 1")
	}

	sampler := sdktrace.NeverSample()
	if cfg.Enabled {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))
	}
	serviceName := strings.TrimSpace(cfg.ServiceName)
	if serviceName == "" {
		serviceName = "omnicraft-server"
	}
	res, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("create tracing resource: %w", err)
	}
	providerOptions := []sdktrace.TracerProviderOption{sdktrace.WithSampler(sampler), sdktrace.WithResource(res)}

	var exporter sdktrace.SpanExporter
	for _, candidate := range exporters {
		if candidate != nil {
			exporter = candidate
			break
		}
	}
	if cfg.Enabled && exporter == nil && strings.TrimSpace(cfg.Endpoint) != "" {
		endpoint, insecure, err := otlpEndpoint(cfg.Endpoint)
		if err != nil {
			return nil, err
		}
		options := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
		if insecure {
			options = append(options, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, options...)
		if err != nil {
			// Exporter construction can fail for malformed local configuration,
			// but collector availability is checked asynchronously by the SDK.
			return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
		}
		exporter = &loggingSpanExporter{delegate: exporter}
	}
	if exporter != nil && cfg.Enabled {
		providerOptions = append(providerOptions, sdktrace.WithBatcher(exporter))
	}
	return sdktrace.NewTracerProvider(providerOptions...), nil
}

// InstallTracerProvider installs the application provider and the W3C
// propagator used by HTTP and queue boundaries. It returns a shutdown function
// so each composition root can flush without making telemetry a dependency of
// business shutdown.
func InstallTracerProvider(provider *sdktrace.TracerProvider) func(context.Context) error {
	if provider == nil {
		return func(context.Context) error { return nil }
	}
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return provider.Shutdown
}

// TraceID returns the current OTel trace ID, or an empty string when the
// context is not carrying a sampled/remote span. It never manufactures a
// second request identifier.
func TraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id := oteltrace.SpanContextFromContext(ctx).TraceID()
	if !id.IsValid() {
		return ""
	}
	return id.String()
}

func otlpEndpoint(raw string) (string, bool, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false, errors.New("observability.tracing.endpoint must not be empty")
	}
	if !strings.Contains(value, "://") {
		return value, true, nil
	}
	u, err := url.Parse(value)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", false, errors.New("observability.tracing.endpoint must be host:port or an http(s) URL")
	}
	return u.Host, u.Scheme == "http", nil
}

func logTraceExportFailure(err error) {
	if err != nil {
		slog.Warn("OTel trace export failed; business operation continues", "error", err)
	}
}

type loggingSpanExporter struct {
	delegate sdktrace.SpanExporter
}

func (e *loggingSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.delegate.ExportSpans(ctx, spans)
	logTraceExportFailure(err)
	return err
}

func (e *loggingSpanExporter) Shutdown(ctx context.Context) error {
	return e.delegate.Shutdown(ctx)
}
