// Package events defines the stable event surface shared by the transactional
// outbox and its consumers (reliable-async-observability roadmap T02, issue
// #137). The envelope shape is fixed: every persisted and delivered event
// carries {event_id, event_type, schema_version, aggregate_id, occurred_at,
// traceparent, tracestate, payload}. Each event type has a registered schema
// validator, so producers and consumers agree on payload shape without code
// archaeology.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/propagation"
)

// Content event topics. These are the minimal event set the RAG indexer
// consumes (rag-deepening design §3): published triggers (re)indexing, updated
// triggers re-indexing of the changed version, banned removes the content from
// the index, deleted removes the content from the index.
const (
	TopicContentPublished     = "content.published"
	TopicContentUpdated       = "content.updated"
	TopicContentBanned        = "content.banned"
	TopicContentDeleted       = "content.deleted"
	TopicArchiveScanRequested = "archive.scan.requested"
)

// ContentSchemaVersion is the schema version of the current ContentEventPayload
// shape. Bump it (and keep old shapes under their own version) whenever the
// payload contract changes incompatibly.
const ContentSchemaVersion = 1

// Envelope is the fixed wire/storage shape of one event. EventID is populated
// by the outbox row id after persistence; producers create envelopes with
// EventID 0.
type Envelope struct {
	EventID       int64           `json:"event_id"`
	EventType     string          `json:"event_type"`
	SchemaVersion int             `json:"schema_version"`
	AggregateID   int64           `json:"aggregate_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Traceparent   string          `json:"traceparent,omitempty"`
	Tracestate    string          `json:"tracestate,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

// ContentEventPayload is the payload shape shared by the four content topics.
// ContentID and AuthorID are mandatory; ContentType and Status are context
// fields the consumer may act on (terminal status for published/banned).
type ContentEventPayload struct {
	ContentID   int64  `json:"content_id"`
	AuthorID    int64  `json:"author_id"`
	ContentType string `json:"content_type,omitempty"`
	Status      string `json:"status,omitempty"`
}

// ArchiveScanEventPayload identifies the S01 scan job created in the same
// transaction as the upload state change and outbox row.
type ArchiveScanEventPayload struct {
	JobID int64 `json:"job_id"`
}

// Validate checks the envelope against the fixed contract and the schema
// validator registered for its event type.
func (e Envelope) Validate() error {
	validator, ok := topicValidators[e.EventType]
	if !ok {
		return fmt.Errorf("events: unknown event type %q", e.EventType)
	}
	if e.SchemaVersion < 1 {
		return fmt.Errorf("events: invalid schema_version %d", e.SchemaVersion)
	}
	if e.AggregateID <= 0 {
		return fmt.Errorf("events: invalid aggregate_id %d", e.AggregateID)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("events: occurred_at is required")
	}
	if len(e.Payload) == 0 || !json.Valid(e.Payload) {
		return fmt.Errorf("events: payload must be valid JSON")
	}
	return validator(e.Payload)
}

// NewContentEnvelope builds and validates a content-topic envelope. Trace
// context is carried verbatim so W3C trace propagation survives the outbox
// round-trip.
func NewContentEnvelope(topic string, aggregateID int64, traceparent, tracestate string, payload ContentEventPayload) (Envelope, error) {
	payloadBytes, err := marshalContentPayload(payload)
	if err != nil {
		return Envelope{}, err
	}
	env := Envelope{
		EventType:     topic,
		SchemaVersion: ContentSchemaVersion,
		AggregateID:   aggregateID,
		OccurredAt:    time.Now().UTC(),
		Traceparent:   traceparent,
		Tracestate:    tracestate,
		Payload:       payloadBytes,
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// NewArchiveScanEnvelope builds the stable event consumed by the standalone
// archive scan worker. AggregateID is the attachment id; JobID is the
// authoritative workflow id and prevents workers from reconstructing object
// keys from a stream message id.
func NewArchiveScanEnvelope(aggregateID, jobID int64, traceparent, tracestate string) (Envelope, error) {
	payloadBytes, err := json.Marshal(ArchiveScanEventPayload{JobID: jobID})
	if err != nil {
		return Envelope{}, err
	}
	env := Envelope{
		EventType:     TopicArchiveScanRequested,
		SchemaVersion: ContentSchemaVersion,
		AggregateID:   aggregateID,
		OccurredAt:    time.Now().UTC(),
		Traceparent:   traceparent,
		Tracestate:    tracestate,
		Payload:       payloadBytes,
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

func marshalContentPayload(payload ContentEventPayload) (json.RawMessage, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("events: marshal content payload: %w", err)
	}
	return raw, nil
}

func validateContentPayload(topic string, raw json.RawMessage) error {
	var payload ContentEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("events: %s payload is not a content payload: %w", topic, err)
	}
	if payload.ContentID <= 0 {
		return fmt.Errorf("events: %s payload requires content_id > 0", topic)
	}
	if payload.AuthorID <= 0 {
		return fmt.Errorf("events: %s payload requires author_id > 0", topic)
	}
	return nil
}

func validateArchiveScanPayload(raw json.RawMessage) error {
	var payload ArchiveScanEventPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("events: archive scan payload is invalid: %w", err)
	}
	if payload.JobID <= 0 {
		return fmt.Errorf("events: archive scan payload requires job_id > 0")
	}
	return nil
}

// topicValidators holds one schema validator per known topic. A topic without
// an entry is rejected by Envelope.Validate.
var topicValidators = map[string]func(json.RawMessage) error{
	TopicContentPublished:     func(raw json.RawMessage) error { return validateContentPayload(TopicContentPublished, raw) },
	TopicContentUpdated:       func(raw json.RawMessage) error { return validateContentPayload(TopicContentUpdated, raw) },
	TopicContentBanned:        func(raw json.RawMessage) error { return validateContentPayload(TopicContentBanned, raw) },
	TopicContentDeleted:       func(raw json.RawMessage) error { return validateContentPayload(TopicContentDeleted, raw) },
	TopicArchiveScanRequested: validateArchiveScanPayload,
}

type traceContextKey struct{}

type traceContext struct {
	traceparent string
	tracestate  string
}

// WithTraceContext attaches the W3C trace context to ctx. The HTTP entry
// middleware (T08, OTel integration) is the production writer; the carrier
// itself is what keeps traceparent/tracestate stable from HTTP through the
// outbox row.
func WithTraceContext(ctx context.Context, traceparent, tracestate string) context.Context {
	return context.WithValue(ctx, traceContextKey{}, traceContext{traceparent: traceparent, tracestate: tracestate})
}

// FromContext extracts the W3C trace context attached to ctx, or empty strings
// when no context was carried.
func FromContext(ctx context.Context) (traceparent, tracestate string) {
	if ctx == nil {
		return "", ""
	}
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.traceparent, tc.tracestate
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("traceparent"), carrier.Get("tracestate")
}
