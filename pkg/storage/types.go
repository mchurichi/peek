package storage

import (
	"encoding/json"
	"time"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Fields    map[string]interface{} `json:"fields"`
	Raw       string                 `json:"raw"`
}

// ToJSON serializes the LogEntry to JSON
func (l *LogEntry) ToJSON() ([]byte, error) {
	return json.Marshal(l)
}

// FromJSON deserializes JSON to LogEntry
func FromJSON(data []byte) (*LogEntry, error) {
	var entry LogEntry
	err := json.Unmarshal(data, &entry)
	return &entry, err
}
