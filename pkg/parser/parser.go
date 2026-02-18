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

// LogfmtParser handles key=value log format (logfmt)
type LogfmtParser struct{}

// NewLogfmtParser creates a new logfmt parser
func NewLogfmtParser() *LogfmtParser {
	return &LogfmtParser{}
}

// CanParse checks if the line looks like logfmt (key=value pairs)
func (p *LogfmtParser) CanParse(line string) bool {
	// Must contain at least a msg= or level= to be logfmt
	has := func(key string) bool {
		return strings.Contains(line, key+"=")
	}
	return has("msg") || (has("level") && (has("source") || has("time") || has("error")))
}

// Parse parses a logfmt line into a LogEntry
func (p *LogfmtParser) Parse(line string) (*storage.LogEntry, error) {
	fields := parseLogfmt(line)

	entry := &storage.LogEntry{
		ID:     generateID(),
		Fields: make(map[string]interface{}),
		Raw:    line,
	}

	// Extract timestamp
	if ts, ok := fields["time"]; ok {
		if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
			entry.Timestamp = t
		} else if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
		delete(fields, "time")
	}
	if entry.Timestamp.IsZero() {
		if ts, ok := fields["timestamp"]; ok {
			if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
				entry.Timestamp = t
			}
			delete(fields, "timestamp")
		}
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Extract level
	if level, ok := fields["level"]; ok {
		entry.Level = NormalizeLevel(level)
		delete(fields, "level")
	} else {
		entry.Level = "INFO"
	}

	// Extract message
	if msg, ok := fields["msg"]; ok {
		entry.Message = msg
		delete(fields, "msg")
	} else if msg, ok := fields["message"]; ok {
		entry.Message = msg
		delete(fields, "message")
	}

	// Remaining fields
	for k, v := range fields {
		entry.Fields[k] = v
	}

	return entry, nil
}

// parseLogfmt parses a logfmt-style line into key-value pairs.
// Handles: key=value, key="quoted value", key="value with \"escapes\""
func parseLogfmt(line string) map[string]string {
	result := make(map[string]string)
	i := 0
	n := len(line)

	for i < n {
		// Skip whitespace
		for i < n && line[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}

		// Read key
		keyStart := i
		for i < n && line[i] != '=' && line[i] != ' ' {
			i++
		}
		if i >= n || line[i] != '=' {
			continue
		}
		key := line[keyStart:i]
		i++ // skip '='

		if i >= n {
			result[key] = ""
			break
		}

		// Read value
		var value string
		if line[i] == '"' {
			// Quoted value
			i++ // skip opening quote
			var b strings.Builder
			for i < n {
				if line[i] == '\\' && i+1 < n {
					b.WriteByte(line[i+1])
					i += 2
				} else if line[i] == '"' {
					i++ // skip closing quote
					break
				} else {
					b.WriteByte(line[i])
					i++
				}
			}
			value = b.String()
		} else {
			// Unquoted value
			valStart := i
			for i < n && line[i] != ' ' {
				i++
			}
			value = line[valStart:i]
		}

		result[key] = value
	}

	return result
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
