package storage

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func newBehaviorStorage(t *testing.T) *BadgerStorage {
	t.Helper()
	s, err := NewBadgerStorage(Config{DBPath: t.TempDir(), RetentionSize: 1024 * 1024 * 100, RetentionDays: 30})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func addEntry(t *testing.T, s *BadgerStorage, id string, ts time.Time, level string, fields map[string]interface{}) {
	t.Helper()
	if err := s.Store(&LogEntry{ID: id, Timestamp: ts, Level: level, Message: id, Fields: fields, Raw: id}); err != nil {
		t.Fatalf("Store(%s) error = %v", id, err)
	}
}

func TestQueryWithTimeRangeAndFieldDiscovery(t *testing.T) {
	s := newBehaviorStorage(t)
	base := time.Now().UTC().Add(-3 * time.Hour)
	addEntry(t, s, "old", base, "INFO", map[string]interface{}{"service": "api", "status": 200})
	addEntry(t, s, "mid", base.Add(time.Hour), "ERROR", map[string]interface{}{"service": "worker", "status": 500})
	addEntry(t, s, "new", base.Add(2*time.Hour), "ERROR", map[string]interface{}{"service": "api", "status": 500})

	tr := &TimeRange{Start: base.Add(30 * time.Minute), End: base.Add(90 * time.Minute)}
	results, total, err := s.QueryWithTimeRange(AllFilter{}, tr, 10, 0)
	if err != nil {
		t.Fatalf("QueryWithTimeRange() error = %v", err)
	}
	if total != 1 || len(results) != 1 || results[0].ID != "mid" {
		t.Fatalf("unexpected range query result: total=%d len=%d", total, len(results))
	}

	fields, err := s.GetFields(base.Add(30*time.Minute), base.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("GetFields() error = %v", err)
	}
	if len(fields) == 0 {
		t.Fatalf("expected fields to be discovered")
	}
}

func TestDeletionAndOldestNewest(t *testing.T) {
	s := newBehaviorStorage(t)
	now := time.Now().UTC()
	addEntry(t, s, "1", now.Add(-2*time.Hour), "INFO", nil)
	addEntry(t, s, "2", now.Add(-time.Hour), "DEBUG", nil)
	addEntry(t, s, "3", now, "DEBUG", nil)

	oldest, newest, err := s.GetOldestNewest()
	if err != nil {
		t.Fatalf("GetOldestNewest() error = %v", err)
	}
	if !oldest.Before(newest) {
		t.Fatalf("expected oldest < newest")
	}

	deleted, err := s.DeleteByLevel("DEBUG")
	if err != nil || deleted != 2 {
		t.Fatalf("DeleteByLevel() deleted=%d err=%v", deleted, err)
	}

	addEntry(t, s, "4", now.Add(-3*time.Hour), "INFO", nil)
	cutDeleted, err := s.DeleteOlderThan(now.Add(-90 * time.Minute))
	if err != nil || cutDeleted == 0 {
		t.Fatalf("DeleteOlderThan() deleted=%d err=%v", cutDeleted, err)
	}

	addEntry(t, s, "5", now.Add(-10*time.Minute), "WARN", nil)

	allDeleted, err := s.DeleteAll()
	if err != nil {
		t.Fatalf("DeleteAll() error = %v", err)
	}
	if allDeleted == 0 {
		t.Fatalf("expected DeleteAll() to remove remaining entries")
	}
}

func TestSubscribeAndMaintenanceAPIs(t *testing.T) {
	s := newBehaviorStorage(t)
	ch, cancel := s.Subscribe(AllFilter{})
	defer cancel()

	now := time.Now().UTC()
	addEntry(t, s, "sub-1", now.Add(150*time.Millisecond), "INFO", map[string]interface{}{"service": "api"})

	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for subscription update")
	}

	if err := s.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if s.GetDBPath() == "" {
		t.Fatalf("GetDBPath() returned empty path")
	}

	// Compaction may or may not reclaim space, but should not fail on a healthy DB.
	if _, err := s.CompactDatabaseFully(); err != nil {
		t.Fatalf("CompactDatabaseFully() error = %v", err)
	}
}

func TestInternalRetentionHelpersAndCompactDatabase(t *testing.T) {
	s := newBehaviorStorage(t)
	now := time.Now().UTC()
	addEntry(t, s, "a", now.Add(-3*time.Hour), "INFO", nil)
	addEntry(t, s, "b", now.Add(-2*time.Hour), "INFO", nil)
	addEntry(t, s, "c", now.Add(-time.Hour), "INFO", nil)

	if err := s.deleteOldestEntries(1); err != nil {
		t.Fatalf("deleteOldestEntries() error = %v", err)
	}

	// CompactDatabase may report no rewrite depending on DB state; both are acceptable.
	_ = s.CompactDatabase()

	s.retentionSize = 1
	if err := s.enforceRetention(); err != nil {
		t.Fatalf("enforceRetention(size) error = %v", err)
	}

	s.retentionSize = 0
	s.retentionDays = 1
	if err := s.enforceRetention(); err != nil {
		t.Fatalf("enforceRetention(days) error = %v", err)
	}
}

func TestScanPropagatesCallbackError(t *testing.T) {
	s := newBehaviorStorage(t)
	addEntry(t, s, "scan", time.Now().UTC(), "INFO", nil)

	expected := "stop"
	err := s.Scan(func(*LogEntry) error { return errors.New(expected) })
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Scan() error = %v", err)
	}
}

func TestCompactDatabaseFullyAfterCloseReturnsError(t *testing.T) {
	s, err := NewBadgerStorage(Config{DBPath: t.TempDir(), RetentionSize: 1024 * 1024 * 100, RetentionDays: 30})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	if err := s.Store(&LogEntry{ID: "closed", Timestamp: time.Now().UTC(), Level: "INFO", Message: "closed", Raw: "closed"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if _, err := s.CompactDatabaseFully(); err == nil {
		t.Fatalf("expected compaction error on closed database")
	}
}

func TestGetStatsTracksUnknownLevelAndExpandPath(t *testing.T) {
	s := newBehaviorStorage(t)
	addEntry(t, s, "u", time.Now().UTC(), "", nil)

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.Levels["Unknown"] == 0 {
		t.Fatalf("expected Unknown level bucket")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	if got := expandPath("~/peek-db"); !strings.Contains(got, home) {
		t.Fatalf("expandPath did not use HOME: %q", got)
	}
}

func TestEnforceRetentionNoopWhenDisabled(t *testing.T) {
	s := newBehaviorStorage(t)
	s.retentionDays = 0
	s.retentionSize = 0
	if err := s.enforceRetention(); err != nil {
		t.Fatalf("enforceRetention() error = %v", err)
	}
}
