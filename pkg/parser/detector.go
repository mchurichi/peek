package parser

import (
	"fmt"
	"time"

	"github.com/mchurichi/peek/pkg/storage"
)

// Detector auto-detects and parses log formats
type Detector struct {
	parsers []Parser
}

// NewDetector creates a new format detector
func NewDetector() *Detector {
	return &Detector{
		parsers: []Parser{
			NewLogfmtParser(), // Try logfmt first (key=value)
			NewJSONParser(),   // Then generic JSON
		},
	}
}

// Parse attempts to parse a line with auto-detection
func (d *Detector) Parse(line string) (*storage.LogEntry, error) {
	// Try each parser
	for _, parser := range d.parsers {
		if parser.CanParse(line) {
			return parser.Parse(line)
		}
	}

	// If no parser worked, create a raw entry
	return &storage.LogEntry{
		ID:        generateID(),
		Timestamp: timeNow(),
		Level:     "INFO",
		Message:   line,
		Fields:    make(map[string]interface{}),
		Raw:       line,
	}, nil
}

// ParseWithFormat parses a line with a specific format
func (d *Detector) ParseWithFormat(line, format string) (*storage.LogEntry, error) {
	var parser Parser

	switch format {
	case "json":
		parser = NewJSONParser()
	case "logfmt":
		parser = NewLogfmtParser()
	case "auto":
		return d.Parse(line)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}

	if !parser.CanParse(line) {
		return nil, fmt.Errorf("line does not match format %s", format)
	}

	return parser.Parse(line)
}

// timeNow is a helper for testing
var timeNow = func() time.Time {
	return time.Now()
}
