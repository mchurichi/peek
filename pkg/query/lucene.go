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

	parser.skipWhitespace()
	if parser.pos < len(parser.input) {
		return nil, fmt.Errorf("unexpected token near %q", parser.input[parser.pos:])
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
		if p.peekOp("OR") {
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
		if p.peekOp("AND") {
			p.consume(3)
			p.skipWhitespace()
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
		} else if p.pos < len(p.input) && !p.peekOp("OR") && !p.peek(")") {
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
	if p.peekOp("NOT") {
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

	// Handle required (+) prefix — Lucene semantics: clause is required (same as default AND).
	if p.peekChar('+') {
		p.consume(1)
		return p.parsePrimary()
	}

	// Handle prohibited (-) prefix — Lucene semantics: clause must NOT match.
	if p.peekChar('-') {
		p.consume(1)
		filter, err := p.parsePrimary()
		if err != nil {
			return nil, err
		}
		return &NotFilter{Filter: filter}, nil
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

		// Strip boost suffix before other value checks so "foo"^2 parses correctly
		value = stripBoost(value)

		// Handle quoted strings (phrase match)
		if strings.HasPrefix(value, "\"") {
			value = strings.Trim(value, "\"")
			return &FieldFilter{Field: field, Value: value, Exact: true}, nil
		}

		// Handle regex values: field:/regex/
		if strings.HasPrefix(value, "/") {
			regexStr := p.extractRegex(value)
			re, err := regexp.Compile(regexStr)
			if err != nil {
				return nil, fmt.Errorf("invalid regex: %w", err)
			}
			return &RegexFilter{Field: field, Regex: re}, nil
		}

		// Handle existence: field:*
		if value == "*" {
			return &ExistenceFilter{Field: field}, nil
		}

		// Handle wildcards (* and ?)
		if strings.ContainsAny(value, "*?") {
			re, err := compileWildcard(value)
			if err != nil {
				return nil, fmt.Errorf("invalid wildcard pattern %q: %w", value, err)
			}
			return &WildcardFilter{Field: field, Pattern: value, re: re}, nil
		}

		return &FieldFilter{Field: field, Value: value, Exact: false}, nil
	}

	// Strip boost from bare keyword
	token = stripBoost(token)
	if token == "" {
		return &AllFilter{}, nil
	}

	// Bare quoted phrase — search message and fields for the phrase
	if len(token) >= 2 && token[0] == '"' && token[len(token)-1] == '"' {
		return &KeywordFilter{Keyword: token[1 : len(token)-1]}, nil
	}

	// Check for comparison operator: field > value, field >= value, etc.
	savedPos := p.pos
	p.skipWhitespace()
	if op := p.readComparisonOp(); op != "" {
		p.skipWhitespace()
		valToken := p.readToken()
		if valToken == "" {
			return nil, fmt.Errorf("expected value after '%s'", op)
		}
		return p.makeComparisonFilter(token, op, valToken)
	}
	p.pos = savedPos

	// Keyword search (searches message and fields)
	return &KeywordFilter{Keyword: token}, nil
}

// extractRegex extracts the regex string from a value that starts with "/".
// If the value already ends with "/" (complete token), the content between
// the slashes is returned. Otherwise, additional characters are consumed
// from the parser input until the closing "/" is found — this handles regex
// patterns that were cut short by "(" or ")" in the token reader.
func (p *parser) extractRegex(value string) string {
	// Complete regex already captured (e.g. "/regex/")
	if len(value) >= 2 && value[len(value)-1] == '/' {
		return value[1 : len(value)-1]
	}
	// Partial — strip leading "/" and continue reading until closing "/"
	regexStr := value[1:]
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '/' {
			p.pos++ // consume closing /
			break
		}
		regexStr += string(ch)
		p.pos++
	}
	return regexStr
}

// stripBoost removes a trailing "^number" boosting suffix from a token.
// Boosting is accepted for query-string compatibility but ignored for filtering.
func stripBoost(s string) string {
	if idx := strings.LastIndex(s, "^"); idx > 0 {
		if _, err := strconv.ParseFloat(s[idx+1:], 64); err == nil {
			return s[:idx]
		}
	}
	return s
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

	// Read until whitespace or special char.
	// When :/regex/ is detected the regex body is consumed verbatim so that
	// characters like ( ) > < inside the pattern don't end the token early.
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ':' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '/' {
			// field:/regex/ — consume ':' and the slash-delimited pattern
			p.pos += 2 // consume ':' and opening '/'
			for p.pos < len(p.input) && p.input[p.pos] != '/' {
				p.pos++
			}
			if p.pos < len(p.input) {
				p.pos++ // consume closing '/'
			}
			break
		}
		if ch == ' ' || ch == '(' || ch == ')' || ch == '>' || ch == '<' {
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

// peekOp checks for a boolean operator keyword (AND/OR/NOT) case-insensitively,
// ensuring it is followed by whitespace, '(', or end-of-input (word boundary).
func (p *parser) peekOp(op string) bool {
	n := len(op)
	if p.pos+n > len(p.input) {
		return false
	}
	if strings.ToUpper(p.input[p.pos:p.pos+n]) != op {
		return false
	}
	end := p.pos + n
	if end >= len(p.input) {
		return true
	}
	ch := p.input[end]
	return ch == ' ' || ch == '('
}

// readComparisonOp consumes and returns a comparison operator (>=, <=, >, <)
// at the current position, or returns "" if none is present.
func (p *parser) readComparisonOp() string {
	if p.pos+1 < len(p.input) {
		two := p.input[p.pos : p.pos+2]
		if two == ">=" || two == "<=" {
			p.pos += 2
			return two
		}
	}
	if p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '>' || ch == '<' {
			p.pos++
			return string(ch)
		}
	}
	return ""
}

// makeComparisonFilter builds a comparison filter for field op value.
// For "timestamp" the value is parsed as a time; otherwise as a float.
func (p *parser) makeComparisonFilter(field, op, val string) (Filter, error) {
	// Strip surrounding quotes if present
	if len(val) >= 2 && val[0] == '"' && val[len(val)-1] == '"' {
		val = val[1 : len(val)-1]
	}
	if field == "timestamp" {
		t := p.parseTimeValue(val)
		if t.IsZero() {
			return nil, fmt.Errorf("invalid timestamp value %q", val)
		}
		return &TimestampComparisonFilter{Op: op, Value: t}, nil
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid numeric value %q for comparison operator '%s'", val, op)
	}
	return &NumericComparisonFilter{Field: field, Op: op, Value: n}, nil
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

// WildcardFilter matches field values with wildcards (* and ?)
type WildcardFilter struct {
	Field   string
	Pattern string
	re      *regexp.Regexp // compiled from Pattern; nil only in direct test construction
}

// compileWildcard turns a wildcard pattern into a compiled case-insensitive
// regex. Non-wildcard characters are escaped so dots, parens, etc. are
// treated as literals rather than regex metacharacters.
func compileWildcard(pattern string) (*regexp.Regexp, error) {
	var sb strings.Builder
	sb.WriteString("(?i)^")
	for _, ch := range pattern {
		switch ch {
		case '*':
			sb.WriteString(".*")
		case '?':
			sb.WriteByte('.')
		default:
			sb.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	sb.WriteByte('$')
	return regexp.Compile(sb.String())
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

	re := f.re
	if re == nil {
		// Fallback for filters constructed directly in tests without the parser.
		re, _ = compileWildcard(f.Pattern)
		if re == nil {
			return false
		}
	}
	return re.MatchString(value)
}

// ExistenceFilter matches entries where the specified field is present.
// For built-in fields (level, message) it matches when the value is non-empty.
// For custom fields it matches when the key exists in the entry's Fields map.
type ExistenceFilter struct {
	Field string
}

func (f *ExistenceFilter) Match(entry *storage.LogEntry) bool {
	switch f.Field {
	case "level":
		return entry.Level != ""
	case "message":
		return entry.Message != ""
	case "timestamp":
		return !entry.Timestamp.IsZero()
	default:
		_, ok := entry.Fields[f.Field]
		return ok
	}
}

// RegexFilter matches entries where the field value matches the given regular expression.
type RegexFilter struct {
	Field string
	Regex *regexp.Regexp
}

func (f *RegexFilter) Match(entry *storage.LogEntry) bool {
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

	return f.Regex.MatchString(value)
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

// NumericComparisonFilter filters numeric fields with >, <, >=, <= operators
type NumericComparisonFilter struct {
	Field string
	Op    string
	Value float64
}

func (f *NumericComparisonFilter) Match(entry *storage.LogEntry) bool {
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
	switch f.Op {
	case ">":
		return value > f.Value
	case ">=":
		return value >= f.Value
	case "<":
		return value < f.Value
	case "<=":
		return value <= f.Value
	}
	return false
}

// TimestampComparisonFilter filters by timestamp with >, <, >=, <= operators
type TimestampComparisonFilter struct {
	Op    string
	Value time.Time
}

func (f *TimestampComparisonFilter) Match(entry *storage.LogEntry) bool {
	t := entry.Timestamp
	switch f.Op {
	case ">":
		return t.After(f.Value)
	case ">=":
		return !t.Before(f.Value)
	case "<":
		return t.Before(f.Value)
	case "<=":
		return !t.After(f.Value)
	}
	return false
}
