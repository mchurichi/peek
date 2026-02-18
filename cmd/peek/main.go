package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mchurichi/peek/internal/config"
	"github.com/mchurichi/peek/pkg/parser"
	"github.com/mchurichi/peek/pkg/server"
	"github.com/mchurichi/peek/pkg/storage"
)

func main() {
	// Check for subcommand first
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "help", "--help":
			printHelp()
			return
		case "db":
			if err := runDbCommand(args[1:]); err != nil {
				log.Fatalf("DB command error: %v", err)
			}
			return
		case "server":
			// Handle server subcommand
			os.Args = append([]string{os.Args[0]}, args[1:]...)
		}
	}

	// Define flags
	configPath := flag.String("config", "~/.peek/config.toml", "Path to config file")
	dbPath := flag.String("db-path", "", "Database path (overrides config)")
	retentionSize := flag.String("retention-size", "", "Max storage size (e.g., 1GB, 500MB)")
	retentionDays := flag.Int("retention-days", 0, "Max age of logs in days")
	format := flag.String("format", "auto", "Log format: auto, json, logfmt")
	port := flag.Int("port", 0, "HTTP server port (for server mode)")
	noBrowser := flag.Bool("no-browser", false, "Don't auto-open browser")
	all := flag.Bool("all", false, "Show all historic logs (collect mode only)")
	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	if *help {
		printHelp()
		return
	}

	// Determine mode based on stdin and args
	mode := "collect" // Default mode
	if len(args) > 0 && args[0] == "server" {
		mode = "server"
	} else if isStdinPiped() {
		mode = "collect"
	} else if len(flag.Args()) == 0 {
		// No args and no stdin, default to server
		mode = "server"
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override config with CLI flags
	if *dbPath != "" {
		cfg.Storage.DBPath = *dbPath
	}
	if *retentionSize != "" {
		cfg.Storage.RetentionSize = *retentionSize
	}
	if *retentionDays > 0 {
		cfg.Storage.RetentionDays = *retentionDays
	}
	if *format != "auto" {
		cfg.Parsing.Format = *format
	}
	if *port > 0 {
		cfg.Server.Port = *port
	}
	if *noBrowser {
		cfg.Server.AutoOpenBrowser = false
	}

	// Execute based on mode
	if mode == "collect" {
		if err := runCollectMode(cfg, *all); err != nil {
			log.Fatalf("Collect mode error: %v", err)
		}
	} else {
		if err := runServerMode(cfg); err != nil {
			log.Fatalf("Server mode error: %v", err)
		}
	}
}

func printHelp() {
	fmt.Println(`Peek - Minimalist Log Collector & Viewer

USAGE:
    cat app.log | peek [OPTIONS]         Collect logs from stdin (+ embedded web UI)
    peek server [OPTIONS]                Start web server (browse previously collected logs)
    peek db stats                        Show database info
    peek db clean [OPTIONS]              Delete logs from database

COLLECT OPTIONS:
    --all                  Show all historic logs alongside new ones (default: only current session)
    --config FILE          Path to config file (default: ~/.peek/config.toml)
    --db-path PATH         Database path (default: ~/.peek/db)
    --retention-size SIZE  Max storage (e.g., 1GB, 500MB)
    --retention-days DAYS  Max age of logs (e.g., 7, 30)
    --format FORMAT        auto | json | logfmt (default: auto)
    --port PORT            HTTP port for web UI (default: 8080)
    --no-browser           Don't auto-open browser

SERVER OPTIONS:
    --config FILE      Path to config file (default: ~/.peek/config.toml)
    --db-path PATH     Database path (default: ~/.peek/db)
    --port PORT        HTTP port (default: 8080)
    --no-browser       Don't auto-open browser

DB CLEAN OPTIONS:
    --older-than DURATION  Delete logs older than duration (e.g., 24h, 7d, 2w)
    --level LEVEL          Delete only logs matching level (e.g., DEBUG)
    --force                Skip confirmation prompt

EXAMPLES:
    # Collect and view logs in real time (fresh mode - only current session)
    cat app.log | peek

    # Collect and view all historic logs alongside new ones
    cat app.log | peek --all

    # Collect from a running process with custom port
    kubectl logs my-pod -f | peek --port 8081

    # Browse previously collected logs
    peek server

    # Show database info
    peek db stats

    # Delete all logs (with confirmation)
    peek db clean

    # Delete old logs without confirmation
    peek db clean --older-than 7d --force

    # Delete debug logs
    peek db clean --level DEBUG

For more information: https://github.com/mchurichi/peek`)
}

func isStdinPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func runDbCommand(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: peek db [stats|clean]")
		return fmt.Errorf("missing db subcommand")
	}

	subcommand := args[0]
	switch subcommand {
	case "stats":
		return runDbStats(args[1:])
	case "clean":
		return runDbClean(args[1:])
	default:
		return fmt.Errorf("unknown db subcommand: %s", subcommand)
	}
}

func runDbStats(args []string) error {
	fs := flag.NewFlagSet("db stats", flag.ExitOnError)
	configPath := fs.String("config", "~/.peek/config.toml", "Path to config file")
	dbPath := fs.String("db-path", "", "Database path (overrides config)")
	fs.Parse(args)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if *dbPath != "" {
		cfg.Storage.DBPath = *dbPath
	}

	// Initialize storage
	storageCfg := storage.Config{
		DBPath:        expandPath(cfg.Storage.DBPath),
		RetentionSize: cfg.GetRetentionSizeBytes(),
		RetentionDays: cfg.Storage.RetentionDays,
	}

	db, err := storage.NewBadgerStorage(storageCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer db.Close()

	// Get stats
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	oldest, newest, err := db.GetOldestNewest()
	if err != nil {
		return fmt.Errorf("failed to get oldest/newest: %w", err)
	}

	// Print stats
	fmt.Println("Database Statistics")
	fmt.Println("===================")
	fmt.Printf("Path:          %s\n", db.GetDBPath())
	fmt.Printf("Total logs:    %d\n", stats.TotalLogs)
	fmt.Printf("Database size: %.2f MB\n", stats.DBSizeMB)
	if !oldest.IsZero() {
		fmt.Printf("Oldest entry:  %s\n", oldest.Format(time.RFC3339))
	}
	if !newest.IsZero() {
		fmt.Printf("Newest entry:  %s\n", newest.Format(time.RFC3339))
	}
	if len(stats.Levels) > 0 {
		fmt.Println("\nLogs by level:")
		for level, count := range stats.Levels {
			fmt.Printf("  %s: %d\n", level, count)
		}
	}

	return nil
}

func runDbClean(args []string) error {
	fs := flag.NewFlagSet("db clean", flag.ExitOnError)
	configPath := fs.String("config", "~/.peek/config.toml", "Path to config file")
	dbPath := fs.String("db-path", "", "Database path (overrides config)")
	olderThan := fs.String("older-than", "", "Delete logs older than duration (e.g., 24h, 7d, 2w)")
	level := fs.String("level", "", "Delete only logs matching level (e.g., DEBUG)")
	force := fs.Bool("force", false, "Skip confirmation prompt")
	fs.Parse(args)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	if *dbPath != "" {
		cfg.Storage.DBPath = *dbPath
	}

	// Initialize storage
	storageCfg := storage.Config{
		DBPath:        expandPath(cfg.Storage.DBPath),
		RetentionSize: cfg.GetRetentionSizeBytes(),
		RetentionDays: cfg.Storage.RetentionDays,
	}

	db, err := storage.NewBadgerStorage(storageCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer db.Close()

	// Get stats before deletion
	stats, err := db.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	var deleteCount int
	var confirmMsg string

	if *level != "" {
		// Count entries to delete
		if count, ok := stats.Levels[*level]; ok {
			deleteCount = count
			confirmMsg = fmt.Sprintf("This will delete %d log entries with level %s (%.2f MB estimated). Continue?", count, *level, stats.DBSizeMB*(float64(count)/float64(stats.TotalLogs)))
		} else {
			fmt.Printf("No logs found with level %s\n", *level)
			return nil
		}
	} else if *olderThan != "" {
		// Parse duration
		duration, err := parseDuration(*olderThan)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		confirmMsg = fmt.Sprintf("This will delete logs older than %s. Continue?", duration)
	} else {
		// Delete all
		deleteCount = stats.TotalLogs
		confirmMsg = fmt.Sprintf("This will delete all %d log entries (%.2f MB). Continue?", deleteCount, stats.DBSizeMB)
	}

	// Confirm deletion
	if !*force {
		fmt.Printf("⚠️  %s [y/N] ", confirmMsg)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Perform deletion
	var deleted int
	if *level != "" {
		deleted, err = db.DeleteByLevel(*level)
		if err != nil {
			return fmt.Errorf("failed to delete by level: %w", err)
		}
	} else if *olderThan != "" {
		duration, _ := parseDuration(*olderThan)
		cutoff := time.Now().Add(-duration)
		
		deleted, err = db.DeleteOlderThan(cutoff)
		if err != nil {
			return fmt.Errorf("failed to delete older entries: %w", err)
		}
	} else {
		deleted, err = db.DeleteAll()
		if err != nil {
			return fmt.Errorf("failed to delete all: %w", err)
		}
	}

	// Compact database
	if err := db.CompactDatabase(); err != nil {
		log.Printf("Warning: Failed to compact database: %v", err)
	}

	fmt.Printf("✅ Deleted %d entries. Database compacted.\n", deleted)
	return nil
}

func parseDuration(s string) (time.Duration, error) {
	// Support Go durations (24h) and shorthand (7d, 2w)
	if strings.HasSuffix(s, "d") {
		days, err := time.ParseDuration(strings.TrimSuffix(s, "d") + "h")
		if err != nil {
			return 0, err
		}
		return days * 24, nil
	}
	if strings.HasSuffix(s, "w") {
		weeks, err := time.ParseDuration(strings.TrimSuffix(s, "w") + "h")
		if err != nil {
			return 0, err
		}
		return weeks * 24 * 7, nil
	}
	return time.ParseDuration(s)
}

func runCollectMode(cfg *config.Config, showAll bool) error {
	log.Println("Starting collect mode...")

	// Initialize storage (single instance shared with embedded server)
	storageCfg := storage.Config{
		DBPath:        expandPath(cfg.Storage.DBPath),
		RetentionSize: cfg.GetRetentionSizeBytes(),
		RetentionDays: cfg.Storage.RetentionDays,
	}

	db, err := storage.NewBadgerStorage(storageCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer db.Close()

	// Record start time for fresh mode
	var startTime *time.Time
	if !showAll {
		now := time.Now()
		startTime = &now
		log.Println("Fresh mode: only showing logs from current session")
	} else {
		log.Println("Showing all historic logs alongside new ones")
	}

	// Start embedded server for real-time viewing
	srv := server.NewServer(db, startTime)
	srv.StartBroadcastWorker()

	go func() {
		if err := srv.Start(cfg.Server.Port); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	if cfg.Server.AutoOpenBrowser {
		url := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
		go openBrowser(url)
	}

	log.Printf("Web UI available at http://localhost:%d", cfg.Server.Port)

	// Initialize parser
	detector := parser.NewDetector()

	// Read from stdin line by line
	scanner := bufio.NewScanner(os.Stdin)
	count := 0

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse log entry
		entry, err := detector.ParseWithFormat(line, cfg.Parsing.Format)
		if err != nil {
			log.Printf("Warning: Failed to parse line: %v", err)
			continue
		}

		// Store entry
		if err := db.Store(entry); err != nil {
			log.Printf("Warning: Failed to store entry: %v", err)
			continue
		}

		// Broadcast to connected WebSocket clients in real time
		srv.BroadcastLog(entry)

		count++
		if count%1000 == 0 {
			log.Printf("Collected %d log entries", count)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stdin: %w", err)
	}

	// Sync database before closing
	log.Println("Syncing database...")
	if err := db.Sync(); err != nil {
		log.Printf("Warning: Failed to sync database: %v", err)
	}

	log.Printf("Collection complete. Total entries: %d", count)
	log.Printf("Server still running at http://localhost:%d — press Ctrl+C to exit", cfg.Server.Port)

	// Keep server alive after stdin closes so user can still browse logs
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	log.Println("Shutting down...")
	return nil
}

func runServerMode(cfg *config.Config) error {
	log.Println("Starting server mode...")

	// Initialize storage
	storageCfg := storage.Config{
		DBPath:        expandPath(cfg.Storage.DBPath),
		RetentionSize: cfg.GetRetentionSizeBytes(),
		RetentionDays: cfg.Storage.RetentionDays,
	}

	db, err := storage.NewBadgerStorage(storageCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer db.Close()

	// Initialize server
	srv := server.NewServer(db, nil)

	// Start broadcast worker for real-time updates
	srv.StartBroadcastWorker()

	// Auto-open browser
	if cfg.Server.AutoOpenBrowser {
		url := fmt.Sprintf("http://localhost:%d", cfg.Server.Port)
		go openBrowser(url)
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Start(cfg.Server.Port)
	}()

	// Wait for shutdown signal or error
	select {
	case <-sigChan:
		log.Println("Shutting down gracefully...")
		return nil
	case err := <-errChan:
		return err
	}
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		log.Printf("Cannot auto-open browser on %s. Please open: %s", runtime.GOOS, url)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to open browser: %v. Please open: %s", err, url)
	}
}
