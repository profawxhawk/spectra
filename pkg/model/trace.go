package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Trace represents a complete execution trace containing one or more spans.
type Trace struct {
	TraceID   string            `json:"trace_id" msgpack:"trace_id"`
	StartTime time.Time         `json:"start_time" msgpack:"start_time"`
	EndTime   time.Time         `json:"end_time" msgpack:"end_time"`
	Metadata  map[string]string `json:"metadata" msgpack:"metadata"`
	Spans     []Span            `json:"spans" msgpack:"spans"`
}

// Span represents a single unit of work within a trace.
type Span struct {
	SpanID     string                 `json:"span_id" msgpack:"span_id"`
	TraceID    string                 `json:"trace_id" msgpack:"trace_id"`
	ParentID   string                 `json:"parent_id,omitempty" msgpack:"parent_id"`
	Name       string                 `json:"name" msgpack:"name"`
	Kind       SpanKind               `json:"kind" msgpack:"kind"`
	StartTime  time.Time              `json:"start_time" msgpack:"start_time"`
	EndTime    time.Time              `json:"end_time" msgpack:"end_time"`
	Status     SpanStatus             `json:"status" msgpack:"status"`
	Input      string                 `json:"input" msgpack:"input"`
	Output     string                 `json:"output" msgpack:"output"`
	Metadata   map[string]string      `json:"metadata" msgpack:"metadata"`
	Attributes map[string]interface{} `json:"attributes" msgpack:"attributes"`
	Events     []Event                `json:"events" msgpack:"events"`
	Metrics    SpanMetrics            `json:"metrics" msgpack:"metrics"`
}

// SpanKind classifies the type of work a span represents.
type SpanKind string

const (
	SpanKindLLM       SpanKind = "llm"
	SpanKindTool      SpanKind = "tool"
	SpanKindAgent     SpanKind = "agent"
	SpanKindRetriever SpanKind = "retriever"
	SpanKindChain     SpanKind = "chain"
	SpanKindGeneric   SpanKind = "generic"
)

var validSpanKinds = map[SpanKind]bool{
	SpanKindLLM:       true,
	SpanKindTool:      true,
	SpanKindAgent:     true,
	SpanKindRetriever: true,
	SpanKindChain:     true,
	SpanKindGeneric:   true,
}

// SpanStatus indicates whether the span completed successfully.
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "ok"
	SpanStatusError SpanStatus = "error"
)

// Event represents a timestamped annotation within a span.
type Event struct {
	Name      string                 `json:"name" msgpack:"name"`
	Timestamp time.Time              `json:"timestamp" msgpack:"timestamp"`
	Data      map[string]interface{} `json:"data" msgpack:"data"`
}

// SpanMetrics holds performance and cost metrics for a span.
type SpanMetrics struct {
	LatencyMs        float64 `json:"latency_ms" msgpack:"latency_ms"`
	PromptTokens     int     `json:"prompt_tokens" msgpack:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens" msgpack:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens" msgpack:"total_tokens"`
	CostUSD          float64 `json:"cost_usd" msgpack:"cost_usd"`
}

// Validate checks that required fields are set and values are valid.
func (s *Span) Validate() error {
	if s.SpanID == "" {
		return errors.New("spectra: span_id is required")
	}
	if s.TraceID == "" {
		return errors.New("spectra: trace_id is required")
	}
	if s.Name == "" {
		return errors.New("spectra: name is required")
	}
	if !validSpanKinds[s.Kind] {
		return errors.New("spectra: invalid span kind")
	}
	return nil
}

// NewSpan creates a new Span with generated IDs and sensible defaults.
func NewSpan(traceID, name string, kind SpanKind) *Span {
	now := time.Now().UTC()
	return &Span{
		SpanID:     uuid.New().String(),
		TraceID:    traceID,
		Name:       name,
		Kind:       kind,
		StartTime:  now,
		Status:     SpanStatusOK,
		Metadata:   make(map[string]string),
		Attributes: make(map[string]interface{}),
		Events:     make([]Event, 0),
	}
}
