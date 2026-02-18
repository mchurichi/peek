package parser

import (
	"testing"
	"time"
)

func TestDetector_Parse(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantLevel   string
		wantMessage string
		wantFormat  string // "json", "logfmt", or "raw"
	}{
		{
			name:        "auto-detect JSON",
			line:        `{"level":"ERROR","message":"test error"}`,
			wantLevel:   "ERROR",
			wantMessage: "test error",
			wantFormat:  "json",
		},
		{
			name:        "auto-detect logfmt",
			line:        `level=INFO msg="test message"`,
			wantLevel:   "INFO",
			wantMessage: "test message",
			wantFormat:  "logfmt",
		},
		{
			name:        "fallback to raw for plain text",
			line:        `This is just plain text`,
			wantLevel:   "INFO",
			wantMessage: "This is just plain text",
			wantFormat:  "raw",
		},
		{
			name:        "fallback to raw for malformed JSON",
			line:        `{"level":"ERROR"`,
			wantLevel:   "INFO",
			wantMessage: `{"level":"ERROR"`,
			wantFormat:  "raw",
		},
		{
			name:        "fallback to raw for empty line",
			line:        ``,
			wantLevel:   "INFO",
			wantMessage: "",
			wantFormat:  "raw",
		},
	}

	detector := NewDetector()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := detector.Parse(tt.line)
			if err != nil {
				t.Errorf("Detector.Parse() error = %v", err)
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("Detector.Parse() Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("Detector.Parse() Message = %v, want %v", entry.Message, tt.wantMessage)
			}
			if entry.Raw != tt.line {
				t.Errorf("Detector.Parse() Raw = %v, want %v", entry.Raw, tt.line)
			}
			if entry.ID == "" {
				t.Errorf("Detector.Parse() ID is empty")
			}
			if entry.Fields == nil {
				t.Errorf("Detector.Parse() Fields is nil")
			}
		})
	}
}

func TestDetector_ParseWithFormat(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		format      string
		wantLevel   string
		wantMessage string
		wantErr     bool
	}{
		{
			name:        "explicit JSON format",
			line:        `{"level":"ERROR","message":"test"}`,
			format:      "json",
			wantLevel:   "ERROR",
			wantMessage: "test",
			wantErr:     false,
		},
		{
			name:        "explicit logfmt format",
			line:        `level=INFO msg="test"`,
			format:      "logfmt",
			wantLevel:   "INFO",
			wantMessage: "test",
			wantErr:     false,
		},
		{
			name:        "auto format falls back to Parse",
			line:        `plain text`,
			format:      "auto",
			wantLevel:   "INFO",
			wantMessage: "plain text",
			wantErr:     false,
		},
		{
			name:    "JSON format with non-JSON line",
			line:    `not json`,
			format:  "json",
			wantErr: true,
		},
		{
			name:    "logfmt format with non-logfmt line",
			line:    `just text`,
			format:  "logfmt",
			wantErr: true,
		},
		{
			name:    "unknown format",
			line:    `anything`,
			format:  "unknown",
			wantErr: true,
		},
	}

	detector := NewDetector()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := detector.ParseWithFormat(tt.line, tt.format)
			if (err != nil) != tt.wantErr {
				t.Errorf("Detector.ParseWithFormat() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("Detector.ParseWithFormat() Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("Detector.ParseWithFormat() Message = %v, want %v", entry.Message, tt.wantMessage)
			}
		})
	}
}

func TestDetector_RawFallback(t *testing.T) {
	// Test that raw fallback creates entries with current timestamp
	detector := NewDetector()

	// Override timeNow for predictable testing
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	originalTimeNow := timeNow
	timeNow = func() time.Time { return fixedTime }
	defer func() { timeNow = originalTimeNow }()

	entry, err := detector.Parse("plain text log")
	if err != nil {
		t.Fatalf("Detector.Parse() error = %v", err)
	}

	if !entry.Timestamp.Equal(fixedTime) {
		t.Errorf("Raw fallback Timestamp = %v, want %v", entry.Timestamp, fixedTime)
	}
	if entry.Level != "INFO" {
		t.Errorf("Raw fallback Level = %v, want INFO", entry.Level)
	}
	if entry.Message != "plain text log" {
		t.Errorf("Raw fallback Message = %v, want 'plain text log'", entry.Message)
	}
}

func TestDetector_JSONPriority(t *testing.T) {
	// Test that detector tries logfmt before JSON
	// A line with both "msg=" and valid JSON should be parsed as logfmt
	detector := NewDetector()

	// This line is valid JSON but also looks like logfmt
	line := `msg="test" {"key":"value"}`

	entry, err := detector.Parse(line)
	if err != nil {
		t.Fatalf("Detector.Parse() error = %v", err)
	}

	// Should be detected as logfmt since logfmt parser is tried first
	if entry.Message != "test" {
		t.Errorf("Expected logfmt parsing, got Message = %v", entry.Message)
	}
}

func TestDetector_EdgeCases(t *testing.T) {
	detector := NewDetector()

	tests := []struct {
		name string
		line string
	}{
		{"empty string", ""},
		{"only whitespace", "   "},
		{"special characters", "!@#$%^&*()"},
		{"unicode", "日本語のログ"},
		{"very long line", string(make([]byte, 10000))},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := detector.Parse(tt.line)
			if err != nil {
				t.Errorf("Detector.Parse() error = %v", err)
			}
			if entry == nil {
				t.Error("Detector.Parse() returned nil entry")
			}
			if entry != nil && entry.ID == "" {
				t.Error("Detector.Parse() returned entry with empty ID")
			}
		})
	}
}
