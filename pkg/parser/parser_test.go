package parser

import (
	"testing"
)

func TestJSONParser_CanParse(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  bool
	}{
		{
			name: "valid JSON object",
			line: `{"level":"ERROR","message":"test"}`,
			want: true,
		},
		{
			name: "valid JSON with timestamp",
			line: `{"timestamp":"2024-01-15T10:30:00Z","level":"INFO","message":"hello"}`,
			want: true,
		},
		{
			name: "invalid JSON",
			line: `not json`,
			want: false,
		},
		{
			name: "empty line",
			line: ``,
			want: false,
		},
		{
			name: "malformed JSON",
			line: `{"level":"ERROR"`,
			want: false,
		},
	}

	parser := NewJSONParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line); got != tt.want {
				t.Errorf("JSONParser.CanParse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJSONParser_Parse(t *testing.T) {
	tests := []struct {
		name         string
		line         string
		wantLevel    string
		wantMessage  string
		wantFieldKey string
		wantErr      bool
	}{
		{
			name:        "complete JSON log",
			line:        `{"timestamp":"2024-01-15T10:30:00Z","level":"ERROR","message":"test error","service":"api"}`,
			wantLevel:   "ERROR",
			wantMessage: "test error",
			wantFieldKey: "service",
			wantErr:     false,
		},
		{
			name:        "JSON with msg field",
			line:        `{"time":"2024-01-15T10:30:00Z","level":"WARN","msg":"warning message"}`,
			wantLevel:   "WARN",
			wantMessage: "warning message",
			wantErr:     false,
		},
		{
			name:        "JSON with severity field",
			line:        `{"timestamp":"2024-01-15T10:30:00Z","severity":"info","message":"info message"}`,
			wantLevel:   "INFO",
			wantMessage: "info message",
			wantErr:     false,
		},
		{
			name:        "JSON without level defaults to INFO",
			line:        `{"timestamp":"2024-01-15T10:30:00Z","message":"no level"}`,
			wantLevel:   "INFO",
			wantMessage: "no level",
			wantErr:     false,
		},
		{
			name:        "JSON without timestamp uses current time",
			line:        `{"level":"DEBUG","message":"no timestamp"}`,
			wantLevel:   "DEBUG",
			wantMessage: "no timestamp",
			wantErr:     false,
		},
		{
			name:    "invalid JSON returns error",
			line:    `not json`,
			wantErr: true,
		},
	}

	parser := NewJSONParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSONParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("JSONParser.Parse() Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("JSONParser.Parse() Message = %v, want %v", entry.Message, tt.wantMessage)
			}
			if entry.Raw != tt.line {
				t.Errorf("JSONParser.Parse() Raw = %v, want %v", entry.Raw, tt.line)
			}
			if entry.ID == "" {
				t.Errorf("JSONParser.Parse() ID is empty")
			}
			if tt.wantFieldKey != "" {
				if _, ok := entry.Fields[tt.wantFieldKey]; !ok {
					t.Errorf("JSONParser.Parse() missing field %v", tt.wantFieldKey)
				}
			}
		})
	}
}

func TestLogfmtParser_CanParse(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{
			name: "valid logfmt with msg",
			line: `level=ERROR msg="connection timeout"`,
			want: true,
		},
		{
			name: "valid logfmt with level and time",
			line: `time=2024-01-15T10:30:00Z level=INFO source=api`,
			want: true,
		},
		{
			name: "valid logfmt with level and error",
			line: `level=ERROR error="database connection failed"`,
			want: true,
		},
		{
			name: "not logfmt - plain text",
			line: `this is just plain text`,
			want: false,
		},
		{
			name: "not logfmt - only level",
			line: `level=INFO`,
			want: false,
		},
	}

	parser := NewLogfmtParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parser.CanParse(tt.line); got != tt.want {
				t.Errorf("LogfmtParser.CanParse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogfmtParser_Parse(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantLevel   string
		wantMessage string
		wantField   string
		wantErr     bool
	}{
		{
			name:        "complete logfmt log",
			line:        `time=2024-01-15T10:30:00Z level=ERROR msg="connection timeout" service=api retry=3`,
			wantLevel:   "ERROR",
			wantMessage: "connection timeout",
			wantField:   "service",
			wantErr:     false,
		},
		{
			name:        "logfmt with quoted values",
			line:        `level=INFO msg="user logged in" user="john@example.com"`,
			wantLevel:   "INFO",
			wantMessage: "user logged in",
			wantField:   "user",
			wantErr:     false,
		},
		{
			name:        "logfmt with escaped quotes",
			line:        `level=WARN msg="error: \"invalid input\""`,
			wantLevel:   "WARN",
			wantMessage: `error: "invalid input"`,
			wantErr:     false,
		},
		{
			name:        "logfmt with message field",
			line:        `level=DEBUG message="debug info"`,
			wantLevel:   "DEBUG",
			wantMessage: "debug info",
			wantErr:     false,
		},
		{
			name:        "logfmt without level defaults to INFO",
			line:        `time=2024-01-15T10:30:00Z msg="no level"`,
			wantLevel:   "INFO",
			wantMessage: "no level",
			wantErr:     false,
		},
	}

	parser := NewLogfmtParser()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parser.Parse(tt.line)
			if (err != nil) != tt.wantErr {
				t.Errorf("LogfmtParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if entry.Level != tt.wantLevel {
				t.Errorf("LogfmtParser.Parse() Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if entry.Message != tt.wantMessage {
				t.Errorf("LogfmtParser.Parse() Message = %v, want %v", entry.Message, tt.wantMessage)
			}
			if entry.Raw != tt.line {
				t.Errorf("LogfmtParser.Parse() Raw = %v, want %v", entry.Raw, tt.line)
			}
			if entry.ID == "" {
				t.Errorf("LogfmtParser.Parse() ID is empty")
			}
			if tt.wantField != "" {
				if _, ok := entry.Fields[tt.wantField]; !ok {
					t.Errorf("LogfmtParser.Parse() missing field %v", tt.wantField)
				}
			}
		})
	}
}

func TestNormalizeLevel(t *testing.T) {
	tests := []struct {
		name  string
		level string
		want  string
	}{
		{"error lowercase", "error", "ERROR"},
		{"err", "err", "ERROR"},
		{"warn lowercase", "warn", "WARN"},
		{"warning", "warning", "WARN"},
		{"info lowercase", "info", "INFO"},
		{"debug lowercase", "debug", "DEBUG"},
		{"dbg", "dbg", "DEBUG"},
		{"trace lowercase", "trace", "TRACE"},
		{"trc", "trc", "TRACE"},
		{"fatal", "fatal", "FATAL"},
		{"critical", "critical", "FATAL"},
		{"crit", "crit", "FATAL"},
		{"uppercase ERROR", "ERROR", "ERROR"},
		{"with spaces", "  INFO  ", "INFO"},
		{"unknown level", "CUSTOM", "CUSTOM"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeLevel(tt.level); got != tt.want {
				t.Errorf("NormalizeLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseLogfmt(t *testing.T) {
	tests := []struct {
		name string
		line string
		want map[string]string
	}{
		{
			name: "simple key-value pairs",
			line: `key1=value1 key2=value2`,
			want: map[string]string{"key1": "value1", "key2": "value2"},
		},
		{
			name: "quoted values",
			line: `name="John Doe" email="john@example.com"`,
			want: map[string]string{"name": "John Doe", "email": "john@example.com"},
		},
		{
			name: "escaped quotes",
			line: `msg="He said \"hello\""`,
			want: map[string]string{"msg": `He said "hello"`},
		},
		{
			name: "mixed quoted and unquoted",
			line: `level=INFO msg="test message" count=5`,
			want: map[string]string{"level": "INFO", "msg": "test message", "count": "5"},
		},
		{
			name: "empty value",
			line: `key=`,
			want: map[string]string{"key": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLogfmt(tt.line)
			if len(got) != len(tt.want) {
				t.Errorf("parseLogfmt() returned %d fields, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("parseLogfmt() field %s = %v, want %v", k, got[k], v)
				}
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	id1 := generateID()
	id2 := generateID()

	if id1 == "" {
		t.Error("generateID() returned empty string")
	}
	if id1 == id2 {
		t.Error("generateID() returned duplicate IDs")
	}
	if len(id1) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("generateID() length = %d, want 16", len(id1))
	}
}
