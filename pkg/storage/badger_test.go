package storage

import (
	"testing"
	"time"
)

func TestNewBadgerStorage(t *testing.T) {
	// Use t.TempDir() for automatic cleanup
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100, // 100MB
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	if storage.db == nil {
		t.Error("NewBadgerStorage() db is nil")
	}
}

func TestBadgerStorage_Store(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	entry := &LogEntry{
		ID:        "test-id-1",
		Timestamp: time.Now(),
		Level:     "ERROR",
		Message:   "test error message",
		Fields:    map[string]interface{}{"service": "api", "retry": 3},
		Raw:       `{"level":"ERROR","message":"test error message"}`,
	}

	err = storage.Store(entry)
	if err != nil {
		t.Errorf("Store() error = %v", err)
	}
}

func TestBadgerStorage_Query(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	// Store test entries
	entries := []*LogEntry{
		{
			ID:        "id-1",
			Timestamp: time.Now().Add(-time.Hour),
			Level:     "ERROR",
			Message:   "error 1",
			Fields:    map[string]interface{}{"service": "api"},
			Raw:       "raw1",
		},
		{
			ID:        "id-2",
			Timestamp: time.Now().Add(-time.Minute * 30),
			Level:     "WARN",
			Message:   "warning 1",
			Fields:    map[string]interface{}{"service": "web"},
			Raw:       "raw2",
		},
		{
			ID:        "id-3",
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   "error 2",
			Fields:    map[string]interface{}{"service": "api"},
			Raw:       "raw3",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	tests := []struct {
		name      string
		filter    Filter
		limit     int
		offset    int
		wantCount int
		wantTotal int
	}{
		{
			name:      "all entries",
			filter:    AllFilter{},
			limit:     10,
			offset:    0,
			wantCount: 3,
			wantTotal: 3,
		},
		{
			name:      "filter by level",
			filter:    LevelFilter{Level: "ERROR"},
			limit:     10,
			offset:    0,
			wantCount: 2,
			wantTotal: 2,
		},
		{
			name:      "with pagination",
			filter:    AllFilter{},
			limit:     2,
			offset:    1,
			wantCount: 2,
			wantTotal: 3,
		},
		{
			name:      "limit exceeds available",
			filter:    AllFilter{},
			limit:     100,
			offset:    0,
			wantCount: 3,
			wantTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, total, err := storage.Query(tt.filter, tt.limit, tt.offset)
			if err != nil {
				t.Errorf("Query() error = %v", err)
				return
			}

			if len(results) != tt.wantCount {
				t.Errorf("Query() returned %d entries, want %d", len(results), tt.wantCount)
			}
			if total != tt.wantTotal {
				t.Errorf("Query() total = %d, want %d", total, tt.wantTotal)
			}
		})
	}
}

func TestBadgerStorage_GetStats(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	// Store entries with different levels
	entries := []*LogEntry{
		{
			ID:        "id-1",
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   "error",
			Fields:    map[string]interface{}{},
			Raw:       "raw1",
		},
		{
			ID:        "id-2",
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   "error",
			Fields:    map[string]interface{}{},
			Raw:       "raw2",
		},
		{
			ID:        "id-3",
			Timestamp: time.Now(),
			Level:     "WARN",
			Message:   "warning",
			Fields:    map[string]interface{}{},
			Raw:       "raw3",
		},
		{
			ID:        "id-4",
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   "info",
			Fields:    map[string]interface{}{},
			Raw:       "raw4",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	stats, err := storage.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}

	if stats.TotalLogs != 4 {
		t.Errorf("GetStats() TotalLogs = %d, want 4", stats.TotalLogs)
	}

	expectedLevels := map[string]int{
		"ERROR": 2,
		"WARN":  1,
		"INFO":  1,
	}

	for level, expectedCount := range expectedLevels {
		if count, ok := stats.Levels[level]; !ok {
			t.Errorf("GetStats() missing level %s", level)
		} else if count != expectedCount {
			t.Errorf("GetStats() level %s count = %d, want %d", level, count, expectedCount)
		}
	}

	if stats.DBSizeMB < 0 {
		t.Errorf("GetStats() DBSizeMB = %f, should be >= 0", stats.DBSizeMB)
	}
}

func TestBadgerStorage_Retention_Time(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 1, // 1 day retention
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	// Store old entries (older than retention)
	oldEntry := &LogEntry{
		ID:        "old-id",
		Timestamp: time.Now().AddDate(0, 0, -2), // 2 days ago
		Level:     "ERROR",
		Message:   "old entry",
		Fields:    map[string]interface{}{},
		Raw:       "old",
	}

	// Store recent entry
	newEntry := &LogEntry{
		ID:        "new-id",
		Timestamp: time.Now(),
		Level:     "ERROR",
		Message:   "new entry",
		Fields:    map[string]interface{}{},
		Raw:       "new",
	}

	if err := storage.Store(oldEntry); err != nil {
		t.Fatalf("Store(oldEntry) error = %v", err)
	}
	if err := storage.Store(newEntry); err != nil {
		t.Fatalf("Store(newEntry) error = %v", err)
	}

	// Manually trigger retention
	if err := storage.enforceRetention(); err != nil {
		t.Fatalf("enforceRetention() error = %v", err)
	}

	// Query all entries
	results, _, err := storage.Query(AllFilter{}, 100, 0)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	// Should only have the new entry
	if len(results) != 1 {
		t.Errorf("After retention, got %d entries, want 1", len(results))
	}

	if len(results) > 0 && results[0].ID != "new-id" {
		t.Errorf("After retention, got entry ID %s, want new-id", results[0].ID)
	}
}

func TestBadgerStorage_Scan(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	// Store test entries
	entries := []*LogEntry{
		{
			ID:        "id-1",
			Timestamp: time.Now(),
			Level:     "ERROR",
			Message:   "error 1",
			Fields:    map[string]interface{}{},
			Raw:       "raw1",
		},
		{
			ID:        "id-2",
			Timestamp: time.Now(),
			Level:     "WARN",
			Message:   "warning 1",
			Fields:    map[string]interface{}{},
			Raw:       "raw2",
		},
	}

	for _, entry := range entries {
		if err := storage.Store(entry); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	count := 0
	err = storage.Scan(func(entry *LogEntry) error {
		count++
		if entry.ID == "" {
			t.Error("Scan() returned entry with empty ID")
		}
		return nil
	})

	if err != nil {
		t.Errorf("Scan() error = %v", err)
	}

	if count != 2 {
		t.Errorf("Scan() processed %d entries, want 2", count)
	}
}

func TestBadgerStorage_EmptyDB(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	// Query empty database
	results, total, err := storage.Query(AllFilter{}, 10, 0)
	if err != nil {
		t.Errorf("Query() on empty DB error = %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Query() on empty DB returned %d entries, want 0", len(results))
	}
	if total != 0 {
		t.Errorf("Query() on empty DB total = %d, want 0", total)
	}

	// GetStats on empty database
	stats, err := storage.GetStats()
	if err != nil {
		t.Errorf("GetStats() on empty DB error = %v", err)
	}

	if stats.TotalLogs != 0 {
		t.Errorf("GetStats() on empty DB TotalLogs = %d, want 0", stats.TotalLogs)
	}
}

func TestBadgerStorage_CompactDatabaseFully_EmptyDB(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 7,
	}

	storage, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer storage.Close()

	res, err := storage.CompactDatabaseFully()
	if err != nil {
		t.Fatalf("CompactDatabaseFully() error = %v", err)
	}

	if res.Passes != 0 {
		t.Errorf("CompactDatabaseFully() passes = %d, want 0 on empty DB", res.Passes)
	}
	if res.BeforeBytes < 0 || res.AfterBytes < 0 || res.ReclaimedBytes < 0 {
		t.Errorf("CompactDatabaseFully() returned negative sizes: %+v", res)
	}
}

func TestLevelFilter_Match(t *testing.T) {
	tests := []struct {
		name   string
		filter LevelFilter
		entry  *LogEntry
		want   bool
	}{
		{
			name:   "match",
			filter: LevelFilter{Level: "ERROR"},
			entry:  &LogEntry{Level: "ERROR", Message: "test", Fields: map[string]interface{}{}},
			want:   true,
		},
		{
			name:   "no match",
			filter: LevelFilter{Level: "ERROR"},
			entry:  &LogEntry{Level: "INFO", Message: "test", Fields: map[string]interface{}{}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.entry); got != tt.want {
				t.Errorf("LevelFilter.Match() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLogEntry_ToJSON_FromJSON(t *testing.T) {
	original := &LogEntry{
		ID:        "test-id",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Level:     "ERROR",
		Message:   "test message",
		Fields:    map[string]interface{}{"service": "api", "count": float64(5)},
		Raw:       "raw log line",
	}

	// Serialize
	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error = %v", err)
	}

	// Deserialize
	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error = %v", err)
	}

	// Verify fields
	if restored.ID != original.ID {
		t.Errorf("FromJSON() ID = %v, want %v", restored.ID, original.ID)
	}
	if !restored.Timestamp.Equal(original.Timestamp) {
		t.Errorf("FromJSON() Timestamp = %v, want %v", restored.Timestamp, original.Timestamp)
	}
	if restored.Level != original.Level {
		t.Errorf("FromJSON() Level = %v, want %v", restored.Level, original.Level)
	}
	if restored.Message != original.Message {
		t.Errorf("FromJSON() Message = %v, want %v", restored.Message, original.Message)
	}
	if restored.Raw != original.Raw {
		t.Errorf("FromJSON() Raw = %v, want %v", restored.Raw, original.Raw)
	}

	// Check fields
	if restored.Fields["service"] != "api" {
		t.Errorf("FromJSON() Fields[service] = %v, want api", restored.Fields["service"])
	}
}

func TestBadgerStorage_DeleteOlderThan_CleansIndex(t *testing.T) {
	dbPath := t.TempDir()

	cfg := Config{
		DBPath:        dbPath,
		RetentionSize: 1024 * 1024 * 100,
		RetentionDays: 30,
	}

	store, err := NewBadgerStorage(cfg)
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	defer store.Close()

	now := time.Now()

	// Store old entries (2 days ago)
	oldEntries := []*LogEntry{
		{ID: "old-1", Timestamp: now.Add(-48 * time.Hour), Level: "DEBUG", Message: "old debug", Fields: map[string]interface{}{}, Raw: "old1"},
		{ID: "old-2", Timestamp: now.Add(-49 * time.Hour), Level: "ERROR", Message: "old error", Fields: map[string]interface{}{}, Raw: "old2"},
	}
	// Store recent entries
	newEntries := []*LogEntry{
		{ID: "new-1", Timestamp: now.Add(-1 * time.Hour), Level: "DEBUG", Message: "new debug", Fields: map[string]interface{}{}, Raw: "new1"},
		{ID: "new-2", Timestamp: now, Level: "ERROR", Message: "new error", Fields: map[string]interface{}{}, Raw: "new2"},
	}

	for _, e := range append(oldEntries, newEntries...) {
		if err := store.Store(e); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	// Verify initial stats
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.TotalLogs != 4 {
		t.Fatalf("Initial TotalLogs = %d, want 4", stats.TotalLogs)
	}

	// Delete entries older than 24 hours
	cutoff := now.Add(-24 * time.Hour)
	deleted, err := store.DeleteOlderThan(cutoff)
	if err != nil {
		t.Fatalf("DeleteOlderThan() error = %v", err)
	}
	if deleted != 2 {
		t.Errorf("DeleteOlderThan() deleted = %d, want 2", deleted)
	}

	// Verify stats after deletion — level counts must reflect only remaining entries
	stats, err = store.GetStats()
	if err != nil {
		t.Fatalf("GetStats() after delete error = %v", err)
	}
	if stats.TotalLogs != 2 {
		t.Errorf("After delete TotalLogs = %d, want 2", stats.TotalLogs)
	}
	if count, ok := stats.Levels["DEBUG"]; !ok || count != 1 {
		t.Errorf("After delete Levels[DEBUG] = %d, want 1", count)
	}
	if count, ok := stats.Levels["ERROR"]; !ok || count != 1 {
		t.Errorf("After delete Levels[ERROR] = %d, want 1", count)
	}

	// Verify only new entries remain via query
	results, _, err := store.Query(AllFilter{}, 100, 0)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("After delete Query() returned %d entries, want 2", len(results))
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want func(string) bool // Validation function
	}{
		{
			name: "home directory expansion",
			path: "~/test/path",
			want: func(result string) bool {
				return result != "~/test/path" && len(result) > len("~/test/path")
			},
		},
		{
			name: "absolute path unchanged",
			path: "/absolute/path",
			want: func(result string) bool {
				return result == "/absolute/path"
			},
		},
		{
			name: "relative path unchanged",
			path: "relative/path",
			want: func(result string) bool {
				return result == "relative/path"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.path)
			if !tt.want(got) {
				t.Errorf("expandPath(%v) = %v, validation failed", tt.path, got)
			}
		})
	}
}
