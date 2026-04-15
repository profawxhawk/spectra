package model

import "errors"

// QueryRequest defines a structured query for spans.
type QueryRequest struct {
	Filters   []Filter `json:"filters"`
	Search    string   `json:"search"`     // full-text search term
	OrderBy   string   `json:"order_by"`   // field to sort by
	Ascending bool     `json:"ascending"`  // sort direction
	Limit     int      `json:"limit"`      // max results
	Offset    int      `json:"offset"`     // pagination offset
}

// Filter represents a single field filter condition.
type Filter struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}

// Operator defines the comparison operation for a filter.
type Operator string

const (
	OpEq       Operator = "eq"
	OpNe       Operator = "ne"
	OpGt       Operator = "gt"
	OpGte      Operator = "gte"
	OpLt       Operator = "lt"
	OpLte      Operator = "lte"
	OpContains Operator = "contains"
	OpIn       Operator = "in"
)

// QueryResult holds the results of a query execution.
type QueryResult struct {
	Spans      []Span `json:"spans"`
	TotalCount int    `json:"total_count"`
	HasMore    bool   `json:"has_more"`
}

// Validate checks that the query request has valid parameters.
func (q *QueryRequest) Validate() error {
	if q.Limit <= 0 {
		return errors.New("spectra: limit must be greater than 0")
	}
	if q.Limit > 1000 {
		return errors.New("spectra: limit must not exceed 1000")
	}
	if len(q.Filters) == 0 && q.Search == "" {
		return errors.New("spectra: at least one filter or search term is required")
	}
	return nil
}
