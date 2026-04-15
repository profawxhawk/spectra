package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/spectra-ai/spectra/pkg/model"
)

// matchFilter checks if a span matches a single filter.
func matchFilter(span *model.Span, f model.Filter) bool {
	var fieldVal interface{}

	switch f.Field {
	case "trace_id":
		fieldVal = span.TraceID
	case "span_id":
		fieldVal = span.SpanID
	case "name":
		fieldVal = span.Name
	case "kind":
		fieldVal = string(span.Kind)
	case "status":
		fieldVal = string(span.Status)
	case "parent_id":
		fieldVal = span.ParentID
	default:
		if v, ok := span.Metadata[f.Field]; ok {
			fieldVal = v
		} else if v, ok := span.Attributes[f.Field]; ok {
			fieldVal = v
		} else {
			return false
		}
	}

	strVal := fmt.Sprintf("%v", fieldVal)
	filterStr := fmt.Sprintf("%v", f.Value)

	switch f.Operator {
	case model.OpEq:
		return strVal == filterStr
	case model.OpNe:
		return strVal != filterStr
	case model.OpContains:
		return strings.Contains(strings.ToLower(strVal), strings.ToLower(filterStr))
	case model.OpGt:
		return strVal > filterStr
	case model.OpGte:
		return strVal >= filterStr
	case model.OpLt:
		return strVal < filterStr
	case model.OpLte:
		return strVal <= filterStr
	case model.OpIn:
		if vals, ok := f.Value.([]interface{}); ok {
			for _, v := range vals {
				if fmt.Sprintf("%v", v) == strVal {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// matchesAllFilters checks if a span matches all filters in a query.
func matchesAllFilters(span *model.Span, filters []model.Filter) bool {
	for _, f := range filters {
		if !matchFilter(span, f) {
			return false
		}
	}
	return true
}

// matchesSearch checks if a span's content matches a search query.
func matchesSearch(span *model.Span, search string) bool {
	if search == "" {
		return true
	}
	lower := strings.ToLower(search)
	content := strings.ToLower(span.Input + " " + span.Output + " " + span.Name)
	return strings.Contains(content, lower)
}

// sortSpans sorts spans by a field.
func sortSpans(spans []model.Span, orderBy string, ascending bool) {
	if orderBy == "" {
		return
	}

	getKey := func(s *model.Span) string {
		switch orderBy {
		case "timestamp", "start_time":
			return s.StartTime.Format(time.RFC3339Nano)
		case "name":
			return s.Name
		case "trace_id":
			return s.TraceID
		default:
			return ""
		}
	}

	for i := 0; i < len(spans); i++ {
		for j := i + 1; j < len(spans); j++ {
			ki, kj := getKey(&spans[i]), getKey(&spans[j])
			if (ascending && ki > kj) || (!ascending && ki < kj) {
				spans[i], spans[j] = spans[j], spans[i]
			}
		}
	}
}
