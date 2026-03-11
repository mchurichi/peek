package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

type Query struct {
	filters []Filter
}

type Filter interface {
	Match(entry *storage.LogEntry) bool
}

func Parse(queryStr string) (*Query, error) {
	if strings.TrimSpace(queryStr) == "" || strings.TrimSpace(queryStr) == "*" {
		return &Query{filters: []Filter{&AllFilter{}}}, nil
	}

	p := &parser{input: queryStr}
	filter, err := p.parse()
	if err != nil {
		return nil, err
	}

	p.skipWhitespace()
	if p.pos < len(p.input) {
		return nil, fmt.Errorf("unexpected token near %q", p.input[p.pos:])
	}

	return &Query{filters: []Filter{filter}}, nil
}

func (q *Query) Match(entry *storage.LogEntry) bool {
	for _, filter := range q.filters {
		if !filter.Match(entry) {
			return false
		}
	}
	return true
}

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
		if !p.consumeKeyword("OR") {
			break
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &OrFilter{Left: left, Right: right}
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
		if p.consumeKeyword("AND") {
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
			continue
		}

		if p.pos < len(p.input) && !p.peekChar(')') && !p.peekKeyword("OR") {
			right, err := p.parseNot()
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
			continue
		}

		break
	}

	return left, nil
}

func (p *parser) parseNot() (Filter, error) {
	p.skipWhitespace()
	if p.consumeKeyword("NOT") {
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
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of query")
	}

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

	if p.peekChar('[') || p.peekChar('{') {
		return nil, fmt.Errorf("Lucene-style ranges are not supported; use comparison operators such as >= or <=")
	}

	if p.peekChar('"') {
		phrase, err := p.readQuotedString()
		if err != nil {
			return nil, err
		}
		return &KeywordFilter{Keyword: phrase, Exact: true}, nil
	}

	ident := p.readIdentifier()
	if ident == "" {
		return nil, fmt.Errorf("unexpected end of query")
	}

	p.skipWhitespace()
	if op := p.readComparator(); op != "" {
		value, err := p.readComparisonValue()
		if err != nil {
			return nil, err
		}
		return &ComparisonFilter{Field: ident, Operator: op, Value: value, parser: p}, nil
	}

	if p.peekChar(':') {
		p.consume(1)
		return p.parseFieldExpression(ident)
	}

	return &KeywordFilter{Keyword: ident}, nil
}

func (p *parser) parseFieldExpression(field string) (Filter, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("expected value after %s:", field)
	}

	if p.peekChar('(') {
		return p.parseScopedGroup(field)
	}
	if p.peekChar('[') || p.peekChar('{') {
		return nil, fmt.Errorf("Lucene-style ranges are not supported; use comparison operators such as %s >= ...", field)
	}
	if p.peekChar('"') {
		value, err := p.readQuotedString()
		if err != nil {
			return nil, err
		}
		return &FieldFilter{Field: field, Value: value, Exact: true}, nil
	}

	value := p.readValueToken()
	if value == "" {
		return nil, fmt.Errorf("expected value after %s:", field)
	}
	return buildFieldValueFilter(field, value), nil
}

func (p *parser) parseScopedGroup(field string) (Filter, error) {
	if !p.peekChar('(') {
		return nil, fmt.Errorf("expected opening parenthesis after %s:", field)
	}
	p.consume(1)
	filter, err := p.parseScopedOr(field)
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

func (p *parser) parseScopedOr(field string) (Filter, error) {
	left, err := p.parseScopedAnd(field)
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if !p.consumeKeyword("OR") {
			break
		}
		right, err := p.parseScopedAnd(field)
		if err != nil {
			return nil, err
		}
		left = &OrFilter{Left: left, Right: right}
	}
	return left, nil
}

func (p *parser) parseScopedAnd(field string) (Filter, error) {
	left, err := p.parseScopedNot(field)
	if err != nil {
		return nil, err
	}
	for {
		p.skipWhitespace()
		if p.consumeKeyword("AND") {
			right, err := p.parseScopedNot(field)
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
			continue
		}
		if p.pos < len(p.input) && !p.peekChar(')') && !p.peekKeyword("OR") {
			right, err := p.parseScopedNot(field)
			if err != nil {
				return nil, err
			}
			left = &AndFilter{Left: left, Right: right}
			continue
		}
		break
	}
	return left, nil
}

func (p *parser) parseScopedNot(field string) (Filter, error) {
	p.skipWhitespace()
	if p.consumeKeyword("NOT") {
		filter, err := p.parseScopedValue(field)
		if err != nil {
			return nil, err
		}
		return &NotFilter{Filter: filter}, nil
	}
	return p.parseScopedValue(field)
}

func (p *parser) parseScopedValue(field string) (Filter, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return nil, fmt.Errorf("unexpected end of grouped expression")
	}
	if p.peekChar('(') {
		p.consume(1)
		filter, err := p.parseScopedOr(field)
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
	if p.peekChar('"') {
		value, err := p.readQuotedString()
		if err != nil {
			return nil, err
		}
		return &FieldFilter{Field: field, Value: value, Exact: true}, nil
	}
	value := p.readValueToken()
	if value == "" {
		return nil, fmt.Errorf("expected value in grouped expression")
	}
	return buildFieldValueFilter(field, value), nil
}

func buildFieldValueFilter(field, value string) Filter {
	if value == "*" {
		return &ExistsFilter{Field: field}
	}
	if strings.Contains(value, "*") {
		return &WildcardFilter{Field: field, Pattern: value}
	}
	return &FieldFilter{Field: field, Value: value, Exact: false}
}

var reDay = regexp.MustCompile(`(\d+)d`)

var reWeek = regexp.MustCompile(`(\d+)w`)

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
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02T15:04:05", val); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	}
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
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

func (p *parser) skipWhitespace() {
	for p.pos < len(p.input) && (p.input[p.pos] == ' ' || p.input[p.pos] == '\t' || p.input[p.pos] == '\n') {
		p.pos++
	}
}

func (p *parser) peekChar(ch byte) bool {
	return p.pos < len(p.input) && p.input[p.pos] == ch
}

func (p *parser) consume(n int) {
	p.pos += n
}

func isBoundaryByte(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '(' || ch == ')'
}

func (p *parser) peekKeyword(word string) bool {
	p.skipWhitespace()
	end := p.pos + len(word)
	if end > len(p.input) {
		return false
	}
	if !strings.EqualFold(p.input[p.pos:end], word) {
		return false
	}
	if end < len(p.input) && !isBoundaryByte(p.input[end]) {
		return false
	}
	return true
}

func (p *parser) consumeKeyword(word string) bool {
	if !p.peekKeyword(word) {
		return false
	}
	p.pos += len(word)
	p.skipWhitespace()
	return true
}

func (p *parser) readIdentifier() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == ':' || ch == '(' || ch == ')' || ch == '>' || ch == '<' {
			break
		}
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *parser) readComparator() string {
	if p.pos >= len(p.input) {
		return ""
	}
	if strings.HasPrefix(p.input[p.pos:], ">=") || strings.HasPrefix(p.input[p.pos:], "<=") {
		op := p.input[p.pos : p.pos+2]
		p.pos += 2
		p.skipWhitespace()
		return op
	}
	if p.input[p.pos] == '>' || p.input[p.pos] == '<' {
		op := p.input[p.pos : p.pos+1]
		p.pos++
		p.skipWhitespace()
		return op
	}
	return ""
}

func (p *parser) readQuotedString() (string, error) {
	if !p.peekChar('"') {
		return "", fmt.Errorf("expected quoted string")
	}
	p.consume(1)
	var b strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' {
			if p.pos+1 >= len(p.input) {
				return "", fmt.Errorf("unterminated escape sequence")
			}
			b.WriteByte(p.input[p.pos+1])
			p.pos += 2
			continue
		}
		if ch == '"' {
			p.consume(1)
			return b.String(), nil
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", fmt.Errorf("unterminated quoted string")
}

func (p *parser) readValueToken() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == ')' {
			break
		}
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *parser) readComparisonValue() (string, error) {
	p.skipWhitespace()
	if p.pos >= len(p.input) {
		return "", fmt.Errorf("expected value after comparison operator")
	}
	if p.peekChar('"') {
		return p.readQuotedString()
	}
	value := p.readValueToken()
	if value == "" {
		return "", fmt.Errorf("expected value after comparison operator")
	}
	return value, nil
}

type AllFilter struct{}

func (f *AllFilter) Match(entry *storage.LogEntry) bool { return true }

type AndFilter struct {
	Left  Filter
	Right Filter
}

func (f *AndFilter) Match(entry *storage.LogEntry) bool {
	return f.Left.Match(entry) && f.Right.Match(entry)
}

type OrFilter struct {
	Left  Filter
	Right Filter
}

func (f *OrFilter) Match(entry *storage.LogEntry) bool {
	return f.Left.Match(entry) || f.Right.Match(entry)
}

type NotFilter struct {
	Filter Filter
}

func (f *NotFilter) Match(entry *storage.LogEntry) bool { return !f.Filter.Match(entry) }

type FieldFilter struct {
	Field string
	Value string
	Exact bool
}

func (f *FieldFilter) Match(entry *storage.LogEntry) bool {
	value, ok := valueForField(entry, f.Field)
	if !ok {
		return false
	}
	if f.Exact {
		return strings.EqualFold(value, f.Value)
	}
	return strings.Contains(strings.ToLower(value), strings.ToLower(f.Value))
}

type KeywordFilter struct {
	Keyword string
	Exact   bool
}

func (f *KeywordFilter) Match(entry *storage.LogEntry) bool {
	keyword := strings.ToLower(f.Keyword)
	for _, candidate := range searchableStrings(entry) {
		text := strings.ToLower(candidate)
		if f.Exact {
			if strings.Contains(text, keyword) {
				return true
			}
			continue
		}
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

type WildcardFilter struct {
	Field   string
	Pattern string
}

func (f *WildcardFilter) Match(entry *storage.LogEntry) bool {
	value, ok := valueForField(entry, f.Field)
	if !ok {
		return false
	}
	pattern := regexp.QuoteMeta(f.Pattern)
	pattern = strings.ReplaceAll(pattern, "\\*", ".*")
	matched, _ := regexp.MatchString("(?i)^"+pattern+"$", value)
	return matched
}

type ExistsFilter struct {
	Field string
}

func (f *ExistsFilter) Match(entry *storage.LogEntry) bool {
	_, ok := valueForField(entry, f.Field)
	return ok
}

type ComparisonFilter struct {
	Field    string
	Operator string
	Value    string
	parser   *parser
}

func (f *ComparisonFilter) Match(entry *storage.LogEntry) bool {
	if strings.EqualFold(f.Field, "timestamp") {
		cmp := f.parser.parseTimeValue(f.Value)
		if cmp.IsZero() {
			return false
		}
		return compareTimes(entry.Timestamp, cmp, f.Operator)
	}

	left, ok := numericFieldValue(entry, f.Field)
	if !ok {
		return false
	}
	right, err := strconv.ParseFloat(f.Value, 64)
	if err != nil {
		return false
	}
	return compareFloats(left, right, f.Operator)
}

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

type NumericRangeFilter struct {
	Field string
	Start float64
	End   float64
}

func (f *NumericRangeFilter) Match(entry *storage.LogEntry) bool {
	value, ok := numericFieldValue(entry, f.Field)
	if !ok {
		return false
	}
	return value >= f.Start && value <= f.End
}

func valueForField(entry *storage.LogEntry, field string) (string, bool) {
	switch strings.ToLower(field) {
	case "level":
		if entry.Level == "" {
			return "", false
		}
		return entry.Level, true
	case "message":
		if entry.Message == "" {
			return "", false
		}
		return entry.Message, true
	case "timestamp":
		if entry.Timestamp.IsZero() {
			return "", false
		}
		return entry.Timestamp.UTC().Format(time.RFC3339), true
	case "raw":
		if entry.Raw == "" {
			return "", false
		}
		return entry.Raw, true
	default:
		if v, ok := entry.Fields[field]; ok {
			return fmt.Sprintf("%v", v), true
		}
		return "", false
	}
}

func numericFieldValue(entry *storage.LogEntry, field string) (float64, bool) {
	if v, ok := entry.Fields[field]; ok {
		switch val := v.(type) {
		case float64:
			return val, true
		case int:
			return float64(val), true
		case int64:
			return float64(val), true
		case jsonNumber:
			parsed, err := strconv.ParseFloat(string(val), 64)
			return parsed, err == nil
		case string:
			parsed, err := strconv.ParseFloat(val, 64)
			return parsed, err == nil
		default:
			return 0, false
		}
	}
	return 0, false
}

type jsonNumber string

func searchableStrings(entry *storage.LogEntry) []string {
	values := []string{entry.Level, entry.Message, entry.Raw}
	if !entry.Timestamp.IsZero() {
		values = append(values, entry.Timestamp.UTC().Format(time.RFC3339))
	}
	for _, v := range entry.Fields {
		values = append(values, fmt.Sprintf("%v", v))
	}
	return values
}

func compareTimes(left, right time.Time, op string) bool {
	switch op {
	case ">":
		return left.After(right)
	case ">=":
		return left.After(right) || left.Equal(right)
	case "<":
		return left.Before(right)
	case "<=":
		return left.Before(right) || left.Equal(right)
	default:
		return false
	}
}

func compareFloats(left, right float64, op string) bool {
	switch op {
	case ">":
		return left > right
	case ">=":
		return left >= right
	case "<":
		return left < right
	case "<=":
		return left <= right
	default:
		return false
	}
}
