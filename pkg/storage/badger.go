package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

const (
	logPrefix        = "log:"
	levelIndexPrefix = "index:level:"
	metaPrefix       = "meta:"
)

// BadgerStorage implements log storage with Badger
type BadgerStorage struct {
	db              *badger.DB
	retentionSize   int64 // in bytes
	retentionDays   int
	mu              sync.RWMutex
	writeCount      int
	cleanupInterval int
	cleanupChan     chan struct{}
	doneChan        chan struct{}
}

// Config holds storage configuration
type Config struct {
	DBPath        string
	RetentionSize int64 // in bytes (e.g., 1GB = 1073741824)
	RetentionDays int
}

// NewBadgerStorage creates a new Badger storage instance
func NewBadgerStorage(cfg Config) (*BadgerStorage, error) {
	// Expand home directory
	dbPath := expandPath(cfg.DBPath)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dbPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open Badger database
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil      // Disable badger logging
	opts.SyncWrites = true // Ensure writes are synced to disk

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	// Run value log garbage collection in background
	go func() {
		db.RunValueLogGC(0.5)
	}()

	s := &BadgerStorage{
		db:              db,
		retentionSize:   cfg.RetentionSize,
		retentionDays:   cfg.RetentionDays,
		cleanupInterval: 1000, // Run cleanup every 1000 writes
		cleanupChan:     make(chan struct{}, 1),
		doneChan:        make(chan struct{}),
	}

	// Run initial cleanup
	if err := s.enforceRetention(); err != nil {
		return nil, fmt.Errorf("failed to enforce retention: %w", err)
	}

	// Start background cleanup worker
	go s.cleanupWorker()

	return s, nil
}

// Store saves a log entry
func (s *BadgerStorage) Store(entry *LogEntry) error {
	s.mu.Lock()
	s.writeCount++
	shouldCleanup := s.writeCount%s.cleanupInterval == 0
	s.mu.Unlock()

	// Generate key: log:{timestamp}:{id}
	key := fmt.Sprintf("%s%d:%s", logPrefix, entry.Timestamp.UnixNano(), entry.ID)

	// Serialize entry
	data, err := entry.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	// Store in Badger
	err = s.db.Update(func(txn *badger.Txn) error {
		// Store main entry
		if err := txn.Set([]byte(key), data); err != nil {
			return err
		}

		// Store level index
		levelKey := fmt.Sprintf("%s%s:%d:%s", levelIndexPrefix, entry.Level, entry.Timestamp.UnixNano(), entry.ID)
		if err := txn.Set([]byte(levelKey), []byte(entry.ID)); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to store entry: %w", err)
	}

	// Periodically run cleanup
	if shouldCleanup {
		// Use a channel to prevent multiple concurrent cleanups
		select {
		case s.cleanupChan <- struct{}{}:
			// Cleanup will be handled by background worker
		default:
			// Cleanup already in progress, skip
		}
	}

	return nil
}

// Query retrieves log entries based on filters
func (s *BadgerStorage) Query(filter Filter, limit, offset int) ([]*LogEntry, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []*LogEntry
	total := 0
	skipped := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)

		// Iterate forward through all log entries
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return nil // Skip invalid entries
				}

				// Apply filter
				if !filter.Match(entry) {
					return nil
				}

				total++

				// Handle pagination
				if skipped < offset {
					skipped++
					return nil
				}

				if len(entries) < limit {
					entries = append(entries, entry)
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return entries, total, err
}

// GetStats returns storage statistics
func (s *BadgerStorage) GetStats() (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := Stats{
		Levels: make(map[string]int),
	}

	// Count total logs and by level
	err := s.db.View(func(txn *badger.Txn) error {
		// Count total logs
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			stats.TotalLogs++
		}

		// Count by level
		levelPrefix := []byte(levelIndexPrefix)
		it2 := txn.NewIterator(opts)
		defer it2.Close()

		currentLevel := ""
		for it2.Seek(levelPrefix); it2.ValidForPrefix(levelPrefix); it2.Next() {
			key := string(it2.Item().Key())
			// Extract level from key: index:level:{LEVEL}:{timestamp}:{id}
			parts := strings.SplitN(key[len(levelIndexPrefix):], ":", 2)
			if len(parts) > 0 {
				level := parts[0]
				if level != currentLevel {
					currentLevel = level
				}
				stats.Levels[level]++
			}
		}

		return nil
	})

	if err != nil {
		return stats, err
	}

	// Get DB size
	lsm, vlog := s.db.Size()
	stats.DBSizeMB = float64(lsm+vlog) / (1024 * 1024)

	return stats, nil
}

// enforceRetention removes old entries based on retention policy
func (s *BadgerStorage) enforceRetention() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check size-based retention
	lsm, vlog := s.db.Size()
	currentSize := lsm + vlog

	if s.retentionSize > 0 && currentSize > s.retentionSize {
		// Delete oldest entries
		return s.deleteOldestEntries(int(float64(currentSize-s.retentionSize) * 1.2)) // Delete 20% more to have buffer
	}

	// Check time-based retention
	if s.retentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -s.retentionDays)
		return s.deleteEntriesOlderThan(cutoff)
	}

	return nil
}

// deleteOldestEntries deletes approximately targetBytes worth of oldest entries
func (s *BadgerStorage) deleteOldestEntries(targetBytes int) error {
	var keysToDelete [][]byte
	var indexKeysToDelete [][]byte
	deletedSize := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			keysToDelete = append(keysToDelete, item.KeyCopy(nil))
			deletedSize += int(item.EstimatedSize())

			// Also collect the corresponding level index key
			err := item.Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return nil // Skip invalid entries
				}
				levelKey := fmt.Sprintf("%s%s:%d:%s", levelIndexPrefix, entry.Level, entry.Timestamp.UnixNano(), entry.ID)
				indexKeysToDelete = append(indexKeysToDelete, []byte(levelKey))
				return nil
			})
			if err != nil {
				return err
			}

			if deletedSize >= targetBytes {
				break
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Delete keys in batches
	return s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		for _, key := range indexKeysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// deleteEntriesOlderThan deletes entries older than the cutoff time
func (s *BadgerStorage) deleteEntriesOlderThan(cutoff time.Time) error {
	cutoffNano := cutoff.UnixNano()
	var keysToDelete [][]byte
	var indexKeysToDelete [][]byte

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := string(it.Item().Key())
			// Extract timestamp from key: log:{timestamp}:{id}
			parts := strings.SplitN(key[len(logPrefix):], ":", 2)
			if len(parts) >= 1 {
				var ts int64
				fmt.Sscanf(parts[0], "%d", &ts)
				if ts < cutoffNano {
					keysToDelete = append(keysToDelete, it.Item().KeyCopy(nil))

					// Also collect the corresponding level index key
					err := it.Item().Value(func(val []byte) error {
						entry, err := FromJSON(val)
						if err != nil {
							return nil // Skip invalid entries
						}
						levelKey := fmt.Sprintf("%s%s:%d:%s", levelIndexPrefix, entry.Level, entry.Timestamp.UnixNano(), entry.ID)
						indexKeysToDelete = append(indexKeysToDelete, []byte(levelKey))
						return nil
					})
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Delete keys in batches
	return s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		for _, key := range indexKeysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})
}

// GetOldestNewest returns the oldest and newest timestamps in the database
func (s *BadgerStorage) GetOldestNewest() (oldest, newest time.Time, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)

		// Get oldest (first entry)
		it.Seek(prefix)
		if it.ValidForPrefix(prefix) {
			err := it.Item().Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return err
				}
				oldest = entry.Timestamp
				return nil
			})
			if err != nil {
				return err
			}
		}

		// Get newest (last entry)
		opts.Reverse = true
		it2 := txn.NewIterator(opts)
		defer it2.Close()

		// Seek to the end of the log prefix
		it2.Seek(append(prefix, 0xFF))
		if it2.ValidForPrefix(prefix) {
			err := it2.Item().Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return err
				}
				newest = entry.Timestamp
				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	return oldest, newest, err
}

// DeleteAll deletes all log entries from the database
func (s *BadgerStorage) DeleteAll() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	var keysToDelete [][]byte

	// Collect all log keys and level index keys
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		// Collect log keys
		logPrefixBytes := []byte(logPrefix)
		for it.Seek(logPrefixBytes); it.ValidForPrefix(logPrefixBytes); it.Next() {
			keysToDelete = append(keysToDelete, it.Item().KeyCopy(nil))
			count++
		}

		// Collect level index keys
		levelPrefixBytes := []byte(levelIndexPrefix)
		it2 := txn.NewIterator(opts)
		defer it2.Close()
		for it2.Seek(levelPrefixBytes); it2.ValidForPrefix(levelPrefixBytes); it2.Next() {
			keysToDelete = append(keysToDelete, it2.Item().KeyCopy(nil))
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// Delete all keys in batches
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	return count, err
}

// DeleteByLevel deletes all log entries with the specified level
func (s *BadgerStorage) DeleteByLevel(level string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	var keysToDelete [][]byte
	var indexKeysToDelete [][]byte

	// Collect keys for entries with the specified level
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return nil // Skip invalid entries
				}

				if entry.Level == level {
					keysToDelete = append(keysToDelete, item.KeyCopy(nil))
					count++

					// Also collect corresponding level index key
					levelKey := fmt.Sprintf("%s%s:%d:%s", levelIndexPrefix, entry.Level, entry.Timestamp.UnixNano(), entry.ID)
					indexKeysToDelete = append(indexKeysToDelete, []byte(levelKey))
				}

				return nil
			})
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// Delete all keys
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		for _, key := range indexKeysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	return count, err
}

// DeleteOlderThan deletes all log entries older than the cutoff time
func (s *BadgerStorage) DeleteOlderThan(cutoff time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffNano := cutoff.UnixNano()
	var keysToDelete [][]byte
	var indexKeysToDelete [][]byte
	count := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := string(it.Item().Key())
			// Extract timestamp from key: log:{timestamp}:{id}
			parts := strings.SplitN(key[len(logPrefix):], ":", 2)
			if len(parts) >= 1 {
				var ts int64
				fmt.Sscanf(parts[0], "%d", &ts)
				if ts < cutoffNano {
					keysToDelete = append(keysToDelete, it.Item().KeyCopy(nil))
					count++

					// Also collect the corresponding level index key
					err := it.Item().Value(func(val []byte) error {
						entry, err := FromJSON(val)
						if err != nil {
							return nil // Skip invalid entries
						}
						levelKey := fmt.Sprintf("%s%s:%d:%s", levelIndexPrefix, entry.Level, entry.Timestamp.UnixNano(), entry.ID)
						indexKeysToDelete = append(indexKeysToDelete, []byte(levelKey))
						return nil
					})
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	// Delete keys in batches
	err = s.db.Update(func(txn *badger.Txn) error {
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		for _, key := range indexKeysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}
		return nil
	})

	return count, err
}

// CompactDatabase runs garbage collection to reclaim disk space
func (s *BadgerStorage) CompactDatabase() error {
	return s.db.RunValueLogGC(0.5)
}

// GetDBPath returns the database path
func (s *BadgerStorage) GetDBPath() string {
	return s.db.Opts().Dir
}

// Close closes the database
func (s *BadgerStorage) Close() error {
	close(s.doneChan)
	return s.db.Close()
}

// Sync syncs the database to disk
func (s *BadgerStorage) Sync() error {
	return s.db.Sync()
}

// cleanupWorker runs in the background and handles cleanup requests
func (s *BadgerStorage) cleanupWorker() {
	for {
		select {
		case <-s.cleanupChan:
			s.enforceRetention()
		case <-s.doneChan:
			return
		}
	}
}

// Stats represents storage statistics
type Stats struct {
	TotalLogs int            `json:"total_logs"`
	DBSizeMB  float64        `json:"db_size_mb"`
	Levels    map[string]int `json:"levels"`
}

// Filter represents a query filter
type Filter interface {
	Match(entry *LogEntry) bool
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// AllFilter matches all entries
type AllFilter struct{}

func (f AllFilter) Match(entry *LogEntry) bool {
	return true
}

// LevelFilter matches entries by level
type LevelFilter struct {
	Level string
}

func (f LevelFilter) Match(entry *LogEntry) bool {
	return entry.Level == f.Level
}

// Scan iterates over all log entries
func (s *BadgerStorage) Scan(callback func(*LogEntry) error) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return nil // Skip invalid entries
				}
				return callback(entry)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetFields returns distinct field names and their top values from stored log entries.
// start and end are optional; zero values mean no bound.
func (s *BadgerStorage) GetFields(start, end time.Time) ([]FieldInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	const maxTopValues = 10

	// fieldValues maps field name → value → count
	fieldValues := make(map[string]map[string]int)

	// Initialize built-in fields so they always appear in the result.
	for _, b := range []string{"level", "message", "timestamp"} {
		fieldValues[b] = make(map[string]int)
	}

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(logPrefix)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				entry, err := FromJSON(val)
				if err != nil {
					return nil // skip
				}
				if !start.IsZero() && entry.Timestamp.Before(start) {
					return nil
				}
				if !end.IsZero() && entry.Timestamp.After(end) {
					return nil
				}
				// Built-in field values
				if entry.Level != "" {
					fieldValues["level"][entry.Level]++
				}
				// Dynamic fields
				for k, v := range entry.Fields {
					if fieldValues[k] == nil {
						fieldValues[k] = make(map[string]int)
					}
					fieldValues[k][fmt.Sprintf("%v", v)]++
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("get fields: %w", err)
	}

	// Build result slice.
	result := make([]FieldInfo, 0, len(fieldValues))
	for name, valCounts := range fieldValues {
		result = append(result, FieldInfo{
			Name:      name,
			Type:      "string",
			TopValues: topN(valCounts, maxTopValues),
		})
	}

	return result, nil
}

// topN returns up to n keys from counts, ordered by descending count.
func topN(counts map[string]int, n int) []string {
	type kv struct {
		key   string
		count int
	}
	items := make([]kv, 0, len(counts))
	for k, c := range counts {
		items = append(items, kv{k, c})
	}
	// Simple insertion sort (fields typically small)
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].count > items[j-1].count; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
	result := make([]string, 0, n)
	for i, item := range items {
		if i >= n {
			break
		}
		result = append(result, item.key)
	}
	return result
}

// Subscribe creates a channel for real-time log updates
func (s *BadgerStorage) Subscribe(filter Filter) (<-chan *LogEntry, func()) {
	ch := make(chan *LogEntry, 100)
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		defer close(ch)

		lastCheck := time.Now()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				// Query for new entries since last check
				now := time.Now()
				err := s.db.View(func(txn *badger.Txn) error {
					opts := badger.DefaultIteratorOptions
					opts.Reverse = true
					it := txn.NewIterator(opts)
					defer it.Close()

					// Start from newest
					prefix := []byte(logPrefix)
					for it.Seek(append(prefix, 0xFF)); it.ValidForPrefix(prefix); it.Next() {
						item := it.Item()
						err := item.Value(func(val []byte) error {
							entry, err := FromJSON(val)
							if err != nil {
								return nil
							}

							// Only send entries newer than last check
							if entry.Timestamp.After(lastCheck) && filter.Match(entry) {
								select {
								case ch <- entry:
								default:
									// Channel full, skip
								}
							}

							return nil
						})
						if err != nil {
							return err
						}
					}
					return nil
				})
				if err != nil {
					// Log error but continue
				}
				lastCheck = now
			}
		}
	}()

	cancel := func() {
		close(done)
	}

	return ch, cancel
}
