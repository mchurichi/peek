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
	"syscall"

	"github.com/mchurichi/peek/internal/config"
	"github.com/mchurichi/peek/pkg/parser"
	"github.com/mchurichi/peek/pkg/server"
	"github.com/mchurichi/peek/pkg/storage"
)

func main() {
	// Define flags
	configPath := flag.String("config", "~/.peek/config.toml", "Path to config file")
	dbPath := flag.String("db-path", "", "Database path (overrides config)")
	retentionSize := flag.String("retention-size", "", "Max storage size (e.g., 1GB, 500MB)")
	retentionDays := flag.Int("retention-days", 0, "Max age of logs in days")
	format := flag.String("format", "auto", "Log format: auto, json, slog")
	port := flag.Int("port", 0, "HTTP server port (for server mode)")
	noBrowser := flag.Bool("no-browser", false, "Don't auto-open browser")
	help := flag.Bool("help", false, "Show help")

	// Check for subcommand first
	args := os.Args[1:]
	mode := "collect" // Default mode

	if len(args) > 0 && args[0] == "server" {
		mode = "server"
		// Remove "server" from args and reparse flags
		os.Args = append([]string{os.Args[0]}, args[1:]...)
		flag.Parse()
	} else if len(args) > 0 && args[0] == "help" || (len(args) > 0 && args[0] == "--help") {
		printHelp()
		return
	} else {
		flag.Parse()
		if *help {
			printHelp()
			return
		}
		// Determine mode based on stdin
		if isStdinPiped() {
			mode = "collect"
		} else if len(flag.Args()) == 0 {
			// No args and no stdin, default to server
			mode = "server"
		}
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
		if err := runCollectMode(cfg); err != nil {
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
    cat app.log | peek [OPTIONS]        Collect logs from stdin (+ embedded web UI)
    peek server [OPTIONS]                Start web server (browse previously collected logs)

OPTIONS:
    --config FILE          Path to config file (default: ~/.peek/config.toml)
    --db-path PATH         Database path (default: ~/.peek/db)
    --retention-size SIZE  Max storage (e.g., 1GB, 500MB)
    --retention-days DAYS  Max age of logs (e.g., 7, 30)
    --format FORMAT        auto | json | slog (default: auto)
    --port PORT            HTTP port for web UI (default: 8080, works in both modes)
    --no-browser           Don't auto-open browser
    --help                 Show this help

EXAMPLES:
    # Collect and view logs in real time
    cat app.log | peek

    # Collect from a running process with custom port
    kubectl logs my-pod -f | peek --port 8081

    # Browse previously collected logs
    peek server

    # Custom configuration
    peek server --port 9000 --db-path /tmp/peek-db

For more information: https://github.com/mchurichi/peek
`)
}

func isStdinPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func runCollectMode(cfg *config.Config) error {
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

	// Start embedded server for real-time viewing
	srv := server.NewServer(db)
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
	srv := server.NewServer(db)

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
