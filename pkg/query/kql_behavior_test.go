package query

import (
	"testing"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

func TestParseDurationExtended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "weeks", input: "2w", want: 14 * 24 * time.Hour},
		{name: "days", input: "3d", want: 3 * 24 * time.Hour},
		{name: "go duration", input: "30m", want: 30 * time.Minute},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDurationExtended(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseDurationExtended(%q) expected error", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDurationExtended(%q) error = %v", tt.input, err)
			}
			if got != tt.want {
				t.Fatalf("parseDurationExtended(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTimeValueFormats(t *testing.T) {
	p := &parser{}
	tests := []struct {
		name  string
		input string
	}{
		{name: "date", input: "2025-01-01"},
		{name: "datetime", input: "2025-01-01T10:11:12"},
		{name: "epoch milliseconds", input: "1735689600000"},
		{name: "relative now", input: "now-1h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.parseTimeValue(tt.input); got.IsZero() {
				t.Fatalf("parseTimeValue(%q) returned zero time", tt.input)
			}
		})
	}
}

func TestQueryMatchingBehavior(t *testing.T) {
	entry := &storage.LogEntry{Level: "ERROR", Message: "db failure", Fields: map[string]interface{}{"service": "api", "status": 503}}

	tests := []struct {
		name    string
		query   string
		want    bool
		wantErr bool
	}{
		{name: "boolean query", query: `(level:ERROR AND service:api) OR message:timeout`, want: true},
		{name: "not and keyword", query: `NOT level:INFO AND message:failure`, want: true},
		{name: "exists query", query: `service:* AND status >= 500`, want: true},
		{name: "same-field values", query: `level:(ERROR OR WARN)`, want: true},
		{name: "legacy range rejected", query: `status:[500 TO 599]`, wantErr: true},
		{name: "parse error", query: `(level:ERROR`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := Parse(tt.query)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected error", tt.query)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", tt.query, err)
			}
			if got := q.Match(entry); got != tt.want {
				t.Fatalf("query match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadComparisonValueAndScopedGroup(t *testing.T) {
	p := &parser{input: `>= 2025-01-01T00:00:00Z`, pos: 0}
	if op := p.readComparator(); op != ">=" {
		t.Fatalf("readComparator() = %q, want >=", op)
	}
	value, err := p.readComparisonValue()
	if err != nil {
		t.Fatalf("readComparisonValue() error = %v", err)
	}
	if value != "2025-01-01T00:00:00Z" {
		t.Fatalf("readComparisonValue() = %q", value)
	}

	p = &parser{input: `(ERROR OR WARN)`, pos: 0}
	filter, err := p.parseScopedGroup("level")
	if err != nil {
		t.Fatalf("parseScopedGroup() error = %v", err)
	}
	entry := &storage.LogEntry{Level: "WARN", Message: "warn", Fields: map[string]interface{}{}}
	if !filter.Match(entry) {
		t.Fatalf("expected scoped group to match WARN level")
	}
}

func TestFilterMatchingEdgeCases(t *testing.T) {
	entry := &storage.LogEntry{
		Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
		Level:     "INFO",
		Message:   "service started",
		Fields: map[string]interface{}{
			"service": "api-server",
			"latency": "45.5",
			"status":  201,
		},
	}

	tests := []struct {
		name   string
		filter Filter
		want   bool
	}{
		{name: "exact field match", filter: &FieldFilter{Field: "message", Value: "service started", Exact: true}, want: true},
		{name: "wildcard message", filter: &WildcardFilter{Field: "message", Pattern: "service*"}, want: true},
		{name: "wildcard missing field", filter: &WildcardFilter{Field: "missing", Pattern: "*"}, want: false},
		{name: "timestamp before range", filter: &TimestampRangeFilter{Start: time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)}, want: false},
		{name: "numeric from string", filter: &NumericRangeFilter{Field: "latency", Start: 40, End: 50}, want: true},
		{name: "numeric missing field", filter: &NumericRangeFilter{Field: "missing", Start: 1, End: 2}, want: false},
		{name: "numeric parse failure", filter: &NumericRangeFilter{Field: "service", Start: 1, End: 2}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(entry); got != tt.want {
				t.Fatalf("match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimestampComparisonFilter(t *testing.T) {
	filter := &ComparisonFilter{Field: "timestamp", Operator: "<", Value: "2025-01-02T00:00:00Z", parser: &parser{}}
	entry := &storage.LogEntry{Timestamp: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)}
	if !filter.Match(entry) {
		t.Fatalf("expected timestamp comparison filter to match entry")
	}
}
