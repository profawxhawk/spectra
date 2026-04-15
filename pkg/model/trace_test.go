package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v5"

	"github.com/spectra-ai/spectra/pkg/model"
)

func TestSpanValidate(t *testing.T) {
	t.Run("valid span passes", func(t *testing.T) {
		s := model.NewSpan("trace-1", "llm_call", model.SpanKindLLM)
		assert.NoError(t, s.Validate())
	})

	t.Run("empty span_id fails", func(t *testing.T) {
		s := model.NewSpan("trace-1", "llm_call", model.SpanKindLLM)
		s.SpanID = ""
		assert.EqualError(t, s.Validate(), "spectra: span_id is required")
	})

	t.Run("empty trace_id fails", func(t *testing.T) {
		s := model.NewSpan("trace-1", "llm_call", model.SpanKindLLM)
		s.TraceID = ""
		assert.EqualError(t, s.Validate(), "spectra: trace_id is required")
	})

	t.Run("empty name fails", func(t *testing.T) {
		s := model.NewSpan("trace-1", "llm_call", model.SpanKindLLM)
		s.Name = ""
		assert.EqualError(t, s.Validate(), "spectra: name is required")
	})

	t.Run("invalid kind fails", func(t *testing.T) {
		s := model.NewSpan("trace-1", "llm_call", model.SpanKind("bogus"))
		assert.EqualError(t, s.Validate(), "spectra: invalid span kind")
	})
}

func TestNewSpan(t *testing.T) {
	s := model.NewSpan("trace-abc", "embeddings", model.SpanKindRetriever)

	assert.NotEmpty(t, s.SpanID)
	assert.Equal(t, "trace-abc", s.TraceID)
	assert.Equal(t, "embeddings", s.Name)
	assert.Equal(t, model.SpanKindRetriever, s.Kind)
	assert.Equal(t, model.SpanStatusOK, s.Status)
	assert.NotNil(t, s.Metadata)
	assert.NotNil(t, s.Attributes)
	assert.NotNil(t, s.Events)
	assert.WithinDuration(t, time.Now(), s.StartTime, 2*time.Second)
}

func TestSpanMsgpackRoundTrip(t *testing.T) {
	original := model.Span{
		SpanID:    "span-1",
		TraceID:   "trace-1",
		ParentID:  "parent-1",
		Name:      "gpt4_call",
		Kind:      model.SpanKindLLM,
		StartTime: time.Now().UTC().Truncate(time.Millisecond),
		EndTime:   time.Now().UTC().Add(2 * time.Second).Truncate(time.Millisecond),
		Status:    model.SpanStatusOK,
		Input:     "What is the meaning of life?",
		Output:    "42",
		Metadata:  map[string]string{"model": "gpt-4o"},
		Attributes: map[string]interface{}{
			"temperature": 0.7,
			"max_tokens":  float64(1000),
		},
		Events: []model.Event{
			{
				Name:      "token_generated",
				Timestamp: time.Now().UTC().Truncate(time.Millisecond),
				Data:      map[string]interface{}{"token": "42"},
			},
		},
		Metrics: model.SpanMetrics{
			LatencyMs:        1500.5,
			PromptTokens:     100,
			CompletionTokens: 10,
			TotalTokens:      110,
			CostUSD:          0.005,
		},
	}

	data, err := msgpack.Marshal(&original)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	var decoded model.Span
	err = msgpack.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, original.SpanID, decoded.SpanID)
	assert.Equal(t, original.TraceID, decoded.TraceID)
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Kind, decoded.Kind)
	assert.Equal(t, original.Input, decoded.Input)
	assert.Equal(t, original.Output, decoded.Output)
	assert.Equal(t, original.Metrics.LatencyMs, decoded.Metrics.LatencyMs)
	assert.Equal(t, original.Metrics.TotalTokens, decoded.Metrics.TotalTokens)
	assert.True(t, original.StartTime.Equal(decoded.StartTime))
	assert.True(t, original.EndTime.Equal(decoded.EndTime))
}

func TestQueryRequestValidate(t *testing.T) {
	t.Run("valid request passes", func(t *testing.T) {
		q := model.QueryRequest{
			Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: "abc"}},
			Limit:   50,
		}
		assert.NoError(t, q.Validate())
	})

	t.Run("search-only request passes", func(t *testing.T) {
		q := model.QueryRequest{
			Search: "error timeout",
			Limit:  10,
		}
		assert.NoError(t, q.Validate())
	})

	t.Run("zero limit fails", func(t *testing.T) {
		q := model.QueryRequest{
			Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: "abc"}},
			Limit:   0,
		}
		assert.Error(t, q.Validate())
	})

	t.Run("limit over 1000 fails", func(t *testing.T) {
		q := model.QueryRequest{
			Filters: []model.Filter{{Field: "trace_id", Operator: model.OpEq, Value: "abc"}},
			Limit:   1001,
		}
		assert.Error(t, q.Validate())
	})

	t.Run("empty filters and search fails", func(t *testing.T) {
		q := model.QueryRequest{Limit: 10}
		assert.Error(t, q.Validate())
	})
}
