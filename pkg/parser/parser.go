package parser

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

// Parser interface for different log formats
type Parser interface {
	Parse(line string) (*storage.LogEntry, error)
	CanParse(line string) bool
}

// JSONParser handles standard JSON logs
type JSONParser struct{}

// NewJSONParser creates a new JSON parser
func NewJSONParser() *JSONParser {
	return &JSONParser{}
}

// CanParse checks if the line is valid JSON
func (p *JSONParser) CanParse(line string) bool {
	var obj map[string]interface{}
	return json.Unmarshal([]byte(line), &obj) == nil
}

// Parse parses a JSON log line
func (p *JSONParser) Parse(line string) (*storage.LogEntry, error) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return nil, err
	}

	entry := &storage.LogEntry{
		ID:     generateID(),
		Fields: make(map[string]interface{}),
		Raw:    line,
	}

	// Extract timestamp
	if ts, ok := obj["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
		delete(obj, "timestamp")
	} else if ts, ok := obj["time"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
		delete(obj, "time")
	}

	// If no timestamp found, use current time
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Extract level
	if level, ok := obj["level"].(string); ok {
		entry.Level = strings.ToUpper(level)
		delete(obj, "level")
	} else if level, ok := obj["severity"].(string); ok {
		entry.Level = strings.ToUpper(level)
		delete(obj, "severity")
	} else {
		entry.Level = "INFO"
	}

	// Extract message
	if msg, ok := obj["message"].(string); ok {
		entry.Message = msg
		delete(obj, "message")
	} else if msg, ok := obj["msg"].(string); ok {
		entry.Message = msg
		delete(obj, "msg")
	}

	// Remaining fields go to Fields
	for k, v := range obj {
		entry.Fields[k] = v
	}

	return entry, nil
}

// SlogParser handles Go slog format
type SlogParser struct{}

// NewSlogParser creates a new slog parser
func NewSlogParser() *SlogParser {
	return &SlogParser{}
}

// CanParse checks if the line is valid JSON (slog is JSON-based)
func (p *SlogParser) CanParse(line string) bool {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return false
	}
	// Slog typically has "time" and "msg" fields
	_, hasTime := obj["time"]
	_, hasMsg := obj["msg"]
	return hasTime && hasMsg
}

// Parse parses a slog format line
func (p *SlogParser) Parse(line string) (*storage.LogEntry, error) {
	// Slog uses the same structure as JSON, just with specific field names
	return NewJSONParser().Parse(line)
}

// generateID creates a unique ID for a log entry
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// NormalizeLevel normalizes log levels to standard values
func NormalizeLevel(level string) string {
	level = strings.ToUpper(strings.TrimSpace(level))
	switch level {
	case "ERROR", "ERR":
		return "ERROR"
	case "WARN", "WARNING":
		return "WARN"
	case "INFO", "INFORMATION":
		return "INFO"
	case "DEBUG", "DBG":
		return "DEBUG"
	case "TRACE", "TRC":
		return "TRACE"
	case "FATAL", "CRITICAL", "CRIT":
		return "FATAL"
	default:
		return level
	}
}
