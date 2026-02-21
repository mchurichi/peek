package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

// Query represents a parsed Lucene-style query
type Query struct {
	filters []Filter
}

// Filter represents a query filter condition
type Filter interface {
	Match(entry *storage.LogEntry) bool
}

// Parse parses a Lucene-style query string
func Parse(queryStr string) (*Query, error) {
	if queryStr == "" || queryStr == "*" {
		return &Query{filters: []Filter{&AllFilter{}}}, nil
	}

	parser := &parser{
		input: queryStr,
		pos:   0,
	}

	filter, err := parser.parse()
	if err != nil {
		return nil, err
	}

	return &Query{filters: []Filter{filter}}, nil
}

// Match checks if an entry matches the query
func (q *Query) Match(entry *storage.LogEntry) bool {
	for _, filter := range q.filters {
		if !filter.Match(entry) {
			return false
		}
	}
	return true
}

// parser implements a simple Lucene query parser
type parser struct {
	input string
	pos   int
}

func (p *parser) parse() (Filter, error) {
	return p.parseOr()
}

func (p *parser) parseOr() (Filter, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.peek("OR") {
			p.consume(2)
			p.skipWhitespace()
			right, err := p.parseAnd()
			if err != nil {
				return nil, err
			}
			left = &OrFilter{Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *parser) parseAnd() (Filter, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}

	for {
		p.skipWhitespace()
		if p.peek("AND") {
			p.consume(3)
			p.skipWhitespace()
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
		} else if p.pos < len(p.input) && !p.peek("OR") && !p.peek(")") {
			// Implicit AND
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
		} else {
			break
		}
	}

	return left, nil
}

func (p *parser) parseNot() (Filter, error) {
	p.skipWhitespace()
	if p.peek("NOT") {
		p.consume(3)
		p.skipWhitespace()
		filter, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &NotFilter{Filter: filter}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Filter, error) {
	p.skipWhitespace()

	// Handle parentheses
	if p.peekChar('(') {
		p.consume(1)
		filter, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		p.skipWhitespace()
		if !p.peekChar(')') {
			return nil, fmt.Errorf("expected closing parenthesis")
		}
		p.consume(1)
		return filter, nil
	}

	// Parse field:value or keyword
	token := p.readToken()
	if token == "" {
		return nil, fmt.Errorf("unexpected end of query")
	}

	// Check for field:value syntax
	if strings.Contains(token, ":") {
		parts := strings.SplitN(token, ":", 2)
		field := parts[0]
		value := parts[1]

		// Handle range queries
		if strings.HasPrefix(value, "[") {
			return p.parseRange(field, value)
		}

		// Handle quoted strings
		if strings.HasPrefix(value, "\"") {
			value = strings.Trim(value, "\"")
			return &FieldFilter{Field: field, Value: value, Exact: true}, nil
		}

		// Handle wildcards
		if strings.Contains(value, "*") {
			return &WildcardFilter{Field: field, Pattern: value}, nil
		}

		return &FieldFilter{Field: field, Value: value, Exact: false}, nil
	}

	// Keyword search (searches message and fields)
	return &KeywordFilter{Keyword: token}, nil
}

func (p *parser) parseRange(field, rangeStr string) (Filter, error) {
	// Range format: [start TO end]
	rangeStr = strings.TrimPrefix(rangeStr, "[")
	rangeStr = strings.TrimSuffix(rangeStr, "]")

	parts := strings.Split(rangeStr, " TO ")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range format")
	}

	start := strings.TrimSpace(parts[0])
	end := strings.TrimSpace(parts[1])

	// Handle timestamp ranges
	if field == "timestamp" {
		return &TimestampRangeFilter{
			Start: p.parseTimeValue(start),
			End:   p.parseTimeValue(end),
		}, nil
	}

	// Handle numeric ranges
	return &NumericRangeFilter{
		Field: field,
		Start: p.parseNumericValue(start),
		End:   p.parseNumericValue(end),
	}, nil
}

// reDay matches a number followed by 'd' (days), e.g. "7d".
var reDay = regexp.MustCompile(`(\d+)d`)

// reWeek matches a number followed by 'w' (weeks), e.g. "2w".
var reWeek = regexp.MustCompile(`(\d+)w`)

// parseDurationExtended extends time.ParseDuration to support day ('d') and
// week ('w') units by converting them to hours before parsing.
func parseDurationExtended(s string) (time.Duration, error) {
	s = reDay.ReplaceAllStringFunc(s, func(m string) string {
		n, _ := strconv.Atoi(m[:len(m)-1])
		return fmt.Sprintf("%dh", n*24)
	})
	s = reWeek.ReplaceAllStringFunc(s, func(m string) string {
		n, _ := strconv.Atoi(m[:len(m)-1])
		return fmt.Sprintf("%dh", n*7*24)
	})
	return time.ParseDuration(s)
}

func (p *parser) parseTimeValue(val string) time.Time {
	// Handle relative time (e.g., now-1h, now-7d, now-2w)
	if strings.HasPrefix(val, "now") {
		duration := strings.TrimPrefix(val, "now")
		if duration == "" {
			return time.Now()
		}
		duration = strings.TrimPrefix(duration, "-")
		if d, err := parseDurationExtended(duration); err == nil {
			return time.Now().Add(-d)
		}
	}

	// Parse absolute RFC3339 timestamp
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t
	}

	// Parse datetime without timezone (assume UTC)
	if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	}

	// Parse date-only string (start of day UTC)
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}

	// Parse epoch milliseconds (values > 1e12 are clearly milliseconds, not seconds)
	if ms, err := strconv.ParseInt(val, 10, 64); err == nil && ms > 1_000_000_000_000 {
		return time.Unix(0, ms*int64(time.Millisecond)).UTC()
	}

	return time.Time{}
}

func (p *parser) parseNumericValue(val string) float64 {
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return f
	}
	return 0
}

func (p *parser) readToken() string {
	start := p.pos

	// Handle quoted strings
	if p.peekChar('"') {
		p.consume(1)
		for p.pos < len(p.input) && !p.peekChar('"') {
			p.pos++
		}
		if p.peekChar('"') {
			p.pos++
		}
		return p.input[start:p.pos]
	}

	// Handle ranges
	if p.peekChar('[') {
		p.consume(1)
		for p.pos < len(p.input) && !p.peekChar(']') {
			p.pos++
		}
		if p.peekChar(']') {
			p.pos++
		}
		return p.input[start:p.pos]
	}

	// Read until whitespace or special char
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '(' || ch == ')' {
			break
		}
		p.pos++
	}

	return p.input[start:p.pos]
}

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

func (p *parser) peek(str string) bool {
	if p.pos+len(str) > len(p.input) {
		return false
	}
	return p.input[p.pos:p.pos+len(str)] == str
}

func (p *parser) peekChar(ch byte) bool {
	if p.pos >= len(p.input) {
		return false
	}
	return p.input[p.pos] == ch
}

func (p *parser) consume(n int) {
	p.pos += n
}

// Filter implementations

// AllFilter matches all entries
type AllFilter struct{}

func (f *AllFilter) Match(entry *storage.LogEntry) bool {
	return true
}

// AndFilter combines two filters with AND logic
type AndFilter struct {
	Left  Filter
	Right Filter
}

func (f *AndFilter) Match(entry *storage.LogEntry) bool {
	return f.Left.Match(entry) && f.Right.Match(entry)
}

// OrFilter combines two filters with OR logic
type OrFilter struct {
	Left  Filter
	Right Filter
}

func (f *OrFilter) Match(entry *storage.LogEntry) bool {
	return f.Left.Match(entry) || f.Right.Match(entry)
}

// NotFilter negates a filter
type NotFilter struct {
	Filter Filter
}

func (f *NotFilter) Match(entry *storage.LogEntry) bool {
	return !f.Filter.Match(entry)
}

// FieldFilter matches a specific field value
type FieldFilter struct {
	Field string
	Value string
	Exact bool
}

func (f *FieldFilter) Match(entry *storage.LogEntry) bool {
	var value string

	// Check special fields
	switch f.Field {
	case "level":
		value = entry.Level
	case "message":
		value = entry.Message
	default:
		// Check in Fields map
		if v, ok := entry.Fields[f.Field]; ok {
			value = fmt.Sprintf("%v", v)
		} else {
			return false
		}
	}

	if f.Exact {
		return value == f.Value
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(f.Value))
}

// KeywordFilter searches across message and fields
type KeywordFilter struct {
	Keyword string
}

func (f *KeywordFilter) Match(entry *storage.LogEntry) bool {
	keyword := strings.ToLower(f.Keyword)

	// Search in message
	if strings.Contains(strings.ToLower(entry.Message), keyword) {
		return true
	}

	// Search in fields
	for _, v := range entry.Fields {
		if strings.Contains(strings.ToLower(fmt.Sprintf("%v", v)), keyword) {
			return true
		}
	}

	return false
}

// WildcardFilter matches field values with wildcards
type WildcardFilter struct {
	Field   string
	Pattern string
}

func (f *WildcardFilter) Match(entry *storage.LogEntry) bool {
	var value string

	switch f.Field {
	case "level":
		value = entry.Level
	case "message":
		value = entry.Message
	default:
		if v, ok := entry.Fields[f.Field]; ok {
			value = fmt.Sprintf("%v", v)
		} else {
			return false
		}
	}

	// Convert wildcard pattern to regex
	pattern := strings.ReplaceAll(f.Pattern, "*", ".*")
	pattern = "^" + pattern + "$"
	matched, _ := regexp.MatchString("(?i)"+pattern, value)
	return matched
}

// TimestampRangeFilter filters by timestamp range
type TimestampRangeFilter struct {
	Start time.Time
	End   time.Time
}

func (f *TimestampRangeFilter) Match(entry *storage.LogEntry) bool {
	if !f.Start.IsZero() && entry.Timestamp.Before(f.Start) {
		return false
	}
	if !f.End.IsZero() && entry.Timestamp.After(f.End) {
		return false
	}
	return true
}

// NumericRangeFilter filters numeric field values
type NumericRangeFilter struct {
	Field string
	Start float64
	End   float64
}

func (f *NumericRangeFilter) Match(entry *storage.LogEntry) bool {
	var value float64

	if v, ok := entry.Fields[f.Field]; ok {
		switch val := v.(type) {
		case float64:
			value = val
		case int:
			value = float64(val)
		case string:
			if parsed, err := strconv.ParseFloat(val, 64); err == nil {
				value = parsed
			} else {
				return false
			}
		default:
			return false
		}
	} else {
		return false
	}

	return value >= f.Start && value <= f.End
}
