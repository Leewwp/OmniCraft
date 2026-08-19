package llm

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func startLLMSpan(ctx context.Context, operation, model, system string, temperature float64) (context.Context, oteltrace.Span) {
	ctx, span := otel.Tracer("omnicraft/llm").Start(ctx, "llm."+operation)
	span.SetAttributes(
		attribute.String("gen_ai.system", system),
		attribute.String("gen_ai.request.model", model),
		attribute.Float64("gen_ai.request.temperature", temperature),
	)
	return ctx, span
}

func finishLLMSpan(span oteltrace.Span, err error, usage *TokenUsage) {
	finishLLMSpanWithTotal(span, err, usage, -1)
}

func finishLLMSpanWithTotal(span oteltrace.Span, err error, usage *TokenUsage, totalTokens int) {
	if span == nil {
		return
	}
	if usage != nil {
		span.SetAttributes(
			attribute.Int("gen_ai.usage.input_tokens", usage.PromptTokens),
			attribute.Int("gen_ai.usage.output_tokens", usage.CompletionTokens),
		)
	}
	if totalTokens >= 0 {
		span.SetAttributes(attribute.Int("gen_ai.usage.total_tokens", totalTokens))
	}
	if err != nil {
		// Error descriptions may contain provider-controlled text. Keep the
		// span status bounded and never record the raw error or prompt.
		span.SetStatus(codes.Error, "llm request failed")
	}
	span.End()
}
