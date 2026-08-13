package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

const (
	testTraceparent = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	testTracestate  = "congo=t61rcWkgMzE,rojo=00f067aa0ba902b7"
)

func contentEnvelope(topic string, aggregateID int64) Envelope {
	payload, _ := marshalContentPayload(ContentEventPayload{
		ContentID:   aggregateID,
		AuthorID:    7,
		ContentType: "article",
		Status:      "published",
	})
	return Envelope{
		EventType:     topic,
		SchemaVersion: ContentSchemaVersion,
		AggregateID:   aggregateID,
		OccurredAt:    time.Now().UTC(),
		Traceparent:   testTraceparent,
		Tracestate:    testTracestate,
		Payload:       payload,
	}
}

// TestEnvelopeValidationPerTopic proves every content topic has its own schema
// validator entry and that validation rejects unknown topics, missing
// required payload fields and malformed JSON.
func TestEnvelopeValidationPerTopic(t *testing.T) {
	for _, topic := range []string{
		TopicContentPublished,
		TopicContentUpdated,
		TopicContentBanned,
		TopicContentDeleted,
	} {
		if _, ok := topicValidators[topic]; !ok {
			t.Fatalf("topic %s must have a schema validator registered", topic)
		}
		if err := contentEnvelope(topic, 1001).Validate(); err != nil {
			t.Fatalf("valid %s envelope rejected: %v", topic, err)
		}
	}

	unknown := contentEnvelope("content.exploded", 1001)
	if err := unknown.Validate(); err == nil {
		t.Fatal("unknown event type must be rejected")
	}

	missingFields := contentEnvelope(TopicContentPublished, 1001)
	rawMissing, _ := json.Marshal(map[string]interface{}{"content_id": 1001})
	missingFields.Payload = rawMissing
	if err := missingFields.Validate(); err == nil {
		t.Fatal("payload without author_id must be rejected")
	}

	malformed := contentEnvelope(TopicContentPublished, 1001)
	malformed.Payload = []byte(`{not json`)
	if err := malformed.Validate(); err == nil {
		t.Fatal("malformed payload must be rejected")
	}

	zeroAggregate := contentEnvelope(TopicContentPublished, 0)
	if err := zeroAggregate.Validate(); err == nil {
		t.Fatal("zero aggregate_id must be rejected")
	}
}

// TestContentEnvelopeBuilder rejects invalid topics and keeps the fixed
// envelope shape.
func TestContentEnvelopeBuilder(t *testing.T) {
	env, err := NewContentEnvelope(TopicContentPublished, 42, testTraceparent, testTracestate,
		ContentEventPayload{ContentID: 42, AuthorID: 7, ContentType: "video", Status: "published"})
	if err != nil {
		t.Fatalf("NewContentEnvelope failed: %v", err)
	}
	if env.EventType != TopicContentPublished || env.SchemaVersion != ContentSchemaVersion ||
		env.AggregateID != 42 || env.Traceparent != testTraceparent || env.Tracestate != testTracestate {
		t.Fatalf("envelope fields corrupted: %+v", env)
	}
	if env.OccurredAt.IsZero() {
		t.Fatal("occurred_at must be set by the builder")
	}

	if _, err := NewContentEnvelope("content.exploded", 42, "", "", ContentEventPayload{}); err == nil {
		t.Fatal("NewContentEnvelope must reject non-content topics")
	}
}

// TestTraceContextRoundTrip proves the context carriers preserve the W3C
// trace context unchanged.
func TestTraceContextRoundTrip(t *testing.T) {
	ctx := WithTraceContext(context.Background(), testTraceparent, testTracestate)
	traceparent, tracestate := FromContext(ctx)
	if traceparent != testTraceparent || tracestate != testTracestate {
		t.Fatalf("trace context round-trip = (%q, %q), want (%q, %q)", traceparent, tracestate, testTraceparent, testTracestate)
	}

	emptyTP, emptyTS := FromContext(context.Background())
	if emptyTP != "" || emptyTS != "" {
		t.Fatalf("empty context must yield empty trace context, got (%q, %q)", emptyTP, emptyTS)
	}
}
