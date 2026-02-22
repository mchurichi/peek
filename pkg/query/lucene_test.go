package query

import (
	"testing"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"empty query", "", false},
		{"wildcard query", "*", false},
		{"simple field query", "level:ERROR", false},
		{"keyword query", "timeout", false},
		{"AND query", "level:ERROR AND service:api", false},
		{"OR query", "level:ERROR OR level:WARN", false},
		{"NOT query", "NOT level:DEBUG", false},
		{"complex query", "(level:ERROR OR level:WARN) AND service:api", false},
		{"wildcard field", "message:*timeout*", false},
		{"quoted string", `message:"connection refused"`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := Parse(tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && q == nil {
				t.Error("Parse() returned nil query")
			}
		})
	}
}

func TestFieldFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *FieldFilter
		entry  *storage.LogEntry
		want   bool
	}{
		{
			name:   "match level field",
			filter: &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "match message field",
			filter: &FieldFilter{Field: "message", Value: "timeout", Exact: false},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "match custom field",
			filter: &FieldFilter{Field: "service", Value: "api", Exact: false},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api"}},
			want:   true,
		},
		{
			name:   "no match - different level",
			filter: &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "exact match required",
			filter: &FieldFilter{Field: "level", Value: "ERROR", Exact: true},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "exact match failed",
			filter: &FieldFilter{Field: "message", Value: "timeout", Exact: true},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "case insensitive match",
			filter: &FieldFilter{Field: "level", Value: "error", Exact: false},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "missing field",
			filter: &FieldFilter{Field: "missing", Value: "value", Exact: false},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.entry); got != tt.want {
				t.Errorf("FieldFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestKeywordFilter(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		entry   *storage.LogEntry
		want    bool
	}{
		{
			name:    "match in message",
			keyword: "timeout",
			entry:   &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:    true,
		},
		{
			name:    "match in field",
			keyword: "api",
			entry:   &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api-gateway"}},
			want:    true,
		},
		{
			name:    "no match",
			keyword: "notfound",
			entry:   &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api"}},
			want:    false,
		},
		{
			name:    "case insensitive",
			keyword: "ERROR",
			entry:   &storage.LogEntry{Level: "ERROR", Message: "error occurred", Fields: map[string]interface{}{}},
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &KeywordFilter{Keyword: tt.keyword}
			if got := filter.Match(tt.entry); got != tt.want {
				t.Errorf("KeywordFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAndFilter(t *testing.T) {
	entry := &storage.LogEntry{
		Level:   "ERROR",
		Message: "connection timeout",
		Fields:  map[string]interface{}{"service": "api"},
	}

	tests := []struct {
		name   string
		filter *AndFilter
		want   bool
	}{
		{
			name: "both match",
			filter: &AndFilter{
				Left:  &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "api", Exact: false},
			},
			want: true,
		},
		{
			name: "left matches, right doesn't",
			filter: &AndFilter{
				Left:  &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "web", Exact: false},
			},
			want: false,
		},
		{
			name: "neither matches",
			filter: &AndFilter{
				Left:  &FieldFilter{Field: "level", Value: "INFO", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "web", Exact: false},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(entry); got != tt.want {
				t.Errorf("AndFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrFilter(t *testing.T) {
	entry := &storage.LogEntry{
		Level:   "ERROR",
		Message: "connection timeout",
		Fields:  map[string]interface{}{"service": "api"},
	}

	tests := []struct {
		name   string
		filter *OrFilter
		want   bool
	}{
		{
			name: "both match",
			filter: &OrFilter{
				Left:  &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "api", Exact: false},
			},
			want: true,
		},
		{
			name: "left matches only",
			filter: &OrFilter{
				Left:  &FieldFilter{Field: "level", Value: "ERROR", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "web", Exact: false},
			},
			want: true,
		},
		{
			name: "right matches only",
			filter: &OrFilter{
				Left:  &FieldFilter{Field: "level", Value: "INFO", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "api", Exact: false},
			},
			want: true,
		},
		{
			name: "neither matches",
			filter: &OrFilter{
				Left:  &FieldFilter{Field: "level", Value: "INFO", Exact: false},
				Right: &FieldFilter{Field: "service", Value: "web", Exact: false},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(entry); got != tt.want {
				t.Errorf("OrFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNotFilter(t *testing.T) {
	entry := &storage.LogEntry{
		Level:   "ERROR",
		Message: "test",
		Fields:  map[string]interface{}{},
	}

	tests := []struct {
		name   string
		filter *NotFilter
		want   bool
	}{
		{
			name:   "negate matching filter",
			filter: &NotFilter{Filter: &FieldFilter{Field: "level", Value: "ERROR", Exact: false}},
			want:   false,
		},
		{
			name:   "negate non-matching filter",
			filter: &NotFilter{Filter: &FieldFilter{Field: "level", Value: "INFO", Exact: false}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(entry); got != tt.want {
				t.Errorf("NotFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWildcardFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *WildcardFilter
		entry  *storage.LogEntry
		want   bool
	}{
		{
			name:   "prefix wildcard",
			filter: &WildcardFilter{Field: "message", Pattern: "*timeout"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "suffix wildcard",
			filter: &WildcardFilter{Field: "message", Pattern: "connection*"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "both wildcards",
			filter: &WildcardFilter{Field: "message", Pattern: "*time*"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "no match",
			filter: &WildcardFilter{Field: "message", Pattern: "*error*"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "case insensitive",
			filter: &WildcardFilter{Field: "message", Pattern: "*TIMEOUT*"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "custom field wildcard",
			filter: &WildcardFilter{Field: "service", Pattern: "api*"},
			entry:  &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api-gateway"}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.entry); got != tt.want {
				t.Errorf("WildcardFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimestampRangeFilter(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name   string
		filter *TimestampRangeFilter
		entry  *storage.LogEntry
		want   bool
	}{
		{
			name:   "within range",
			filter: &TimestampRangeFilter{Start: start, End: end},
			entry:  &storage.LogEntry{Timestamp: mid, Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "before range",
			filter: &TimestampRangeFilter{Start: start, End: end},
			entry:  &storage.LogEntry{Timestamp: start.Add(-time.Hour), Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "after range",
			filter: &TimestampRangeFilter{Start: start, End: end},
			entry:  &storage.LogEntry{Timestamp: end.Add(time.Hour), Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "at start boundary",
			filter: &TimestampRangeFilter{Start: start, End: end},
			entry:  &storage.LogEntry{Timestamp: start, Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "no start limit",
			filter: &TimestampRangeFilter{End: end},
			entry:  &storage.LogEntry{Timestamp: start.Add(-time.Hour * 24), Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "no end limit",
			filter: &TimestampRangeFilter{Start: start},
			entry:  &storage.LogEntry{Timestamp: end.Add(time.Hour * 24), Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.entry); got != tt.want {
				t.Errorf("TimestampRangeFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNumericRangeFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter *NumericRangeFilter
		entry  *storage.LogEntry
		want   bool
	}{
		{
			name:   "int within range",
			filter: &NumericRangeFilter{Field: "status", Start: 200, End: 299},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{"status": 200}},
			want:   true,
		},
		{
			name:   "float within range",
			filter: &NumericRangeFilter{Field: "duration", Start: 0.5, End: 2.0},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{"duration": 1.5}},
			want:   true,
		},
		{
			name:   "string number within range",
			filter: &NumericRangeFilter{Field: "count", Start: 10, End: 20},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{"count": "15"}},
			want:   true,
		},
		{
			name:   "below range",
			filter: &NumericRangeFilter{Field: "status", Start: 200, End: 299},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{"status": 199}},
			want:   false,
		},
		{
			name:   "above range",
			filter: &NumericRangeFilter{Field: "status", Start: 200, End: 299},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{"status": 300}},
			want:   false,
		},
		{
			name:   "missing field",
			filter: &NumericRangeFilter{Field: "missing", Start: 0, End: 100},
			entry:  &storage.LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
		{
			name:   "non-numeric field",
			filter: &NumericRangeFilter{Field: "message", Start: 0, End: 100},
			entry:  &storage.LogEntry{Level: "INFO", Message: "not a number", Fields: map[string]interface{}{"message": "text"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.entry); got != tt.want {
				t.Errorf("NumericRangeFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuery_Match(t *testing.T) {
	tests := []struct {
		name  string
		query string
		entry *storage.LogEntry
		want  bool
	}{
		{
			name:  "simple field match",
			query: "level:ERROR",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "AND query both match",
			query: "level:ERROR AND service:api",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api"}},
			want:  true,
		},
		{
			name:  "AND query one doesn't match",
			query: "level:ERROR AND service:web",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{"service": "api"}},
			want:  false,
		},
		{
			name:  "OR query one matches",
			query: "level:ERROR OR level:WARN",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "NOT query",
			query: "NOT level:DEBUG",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "wildcard match",
			query: "message:*timeout*",
			entry: &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "keyword match in message",
			query: "timeout",
			entry: &storage.LogEntry{Level: "ERROR", Message: "connection timeout", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "empty query matches all",
			query: "",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:  true,
		},
		{
			name:  "wildcard query matches all",
			query: "*",
			entry: &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, err := Parse(tt.query)
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if got := q.Match(tt.entry); got != tt.want {
				t.Errorf("Query.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTimeValue(t *testing.T) {
	now := time.Now()
	p := &parser{}

	tests := []struct {
		name string
		val  string
		want func(time.Time) bool // Function to validate the result
	}{
		{
			name: "now",
			val:  "now",
			want: func(t time.Time) bool {
				return t.After(now.Add(-time.Second)) && t.Before(now.Add(time.Second))
			},
		},
		{
			name: "now-1h",
			val:  "now-1h",
			want: func(t time.Time) bool {
				expected := now.Add(-time.Hour)
				return t.After(expected.Add(-time.Second)) && t.Before(expected.Add(time.Second))
			},
		},
		{
			name: "now-7d",
			val:  "now-7d",
			want: func(t time.Time) bool {
				expected := now.Add(-7 * 24 * time.Hour)
				return t.After(expected.Add(-time.Second)) && t.Before(expected.Add(time.Second))
			},
		},
		{
			name: "now-2w",
			val:  "now-2w",
			want: func(t time.Time) bool {
				expected := now.Add(-14 * 24 * time.Hour)
				return t.After(expected.Add(-time.Second)) && t.Before(expected.Add(time.Second))
			},
		},
		{
			name: "absolute RFC3339",
			val:  "2024-01-15T10:30:00Z",
			want: func(t time.Time) bool {
				expected := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
				return t.Equal(expected)
			},
		},
		{
			name: "datetime without timezone",
			val:  "2026-02-01T10:00:00",
			want: func(t time.Time) bool {
				expected := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
				return t.Equal(expected)
			},
		},
		{
			name: "date-only string",
			val:  "2026-02-01",
			want: func(t time.Time) bool {
				expected := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
				return t.Equal(expected)
			},
		},
		{
			name: "epoch milliseconds",
			val:  "1740000000000",
			want: func(t time.Time) bool {
				expected := time.Unix(0, 1740000000000*int64(time.Millisecond)).UTC()
				return t.Equal(expected)
			},
		},
		{
			name: "invalid format returns zero",
			val:  "invalid",
			want: func(t time.Time) bool {
				return t.IsZero()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.parseTimeValue(tt.val)
			if !tt.want(got) {
				t.Errorf("parseTimeValue(%v) = %v, validation failed", tt.val, got)
			}
		})
	}
}

func TestParseNumericValue(t *testing.T) {
	p := &parser{}

	tests := []struct {
		name string
		val  string
		want float64
	}{
		{"integer", "42", 42.0},
		{"float", "3.14", 3.14},
		{"negative", "-10", -10.0},
		{"zero", "0", 0.0},
		{"invalid", "not-a-number", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := p.parseNumericValue(tt.val); got != tt.want {
				t.Errorf("parseNumericValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllFilter(t *testing.T) {
	filter := &AllFilter{}
	entry := &storage.LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}}

	if !filter.Match(entry) {
		t.Error("AllFilter should match all entries")
	}
}
