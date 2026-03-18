package main

import (
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mchurichi/peek/internal/config"
	"github.com/mchurichi/peek/pkg/storage"
)

func TestValidateNoPositionalArgs(t *testing.T) {
	if err := validateNoPositionalArgs(nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := validateNoPositionalArgs([]string{"unexpected"}); err == nil {
		t.Fatalf("expected error for positional args")
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
		err  bool
	}{
		{in: "24h", want: 24 * time.Hour},
		{in: "7d", want: 168 * time.Hour},
		{in: "2w", want: 336 * time.Hour},
		{in: "bad", err: true},
	}
	for _, tt := range tests {
		got, err := parseDuration(tt.in)
		if tt.err {
			if err == nil {
				t.Fatalf("parseDuration(%q) expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseDuration(%q) error = %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("parseDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := expandPath("~/peek")
	want := filepath.Join(home, "peek")
	if got != want {
		t.Fatalf("expandPath mismatch: got %q want %q", got, want)
	}

	if plain := expandPath("/tmp/peek"); plain != "/tmp/peek" {
		t.Fatalf("plain path changed: %q", plain)
	}
}

func TestRunDbCommandValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing", args: nil},
		{name: "unknown", args: []string{"unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := runDbCommand(tt.args); err == nil {
				t.Fatalf("expected error for args=%v", tt.args)
			}
		})
	}
}

func TestRunDbSubcommandsWithTempDB(t *testing.T) {
	dbPath := t.TempDir()

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "stats", run: func() error { return runDbStats([]string{"--db-path", dbPath}) }},
		{name: "clean level", run: func() error { return runDbClean([]string{"--db-path", dbPath, "--level", "DEBUG", "--force"}) }},
		{name: "clean older than", run: func() error { return runDbClean([]string{"--db-path", dbPath, "--older-than", "1h", "--force"}) }},
		{name: "clean all", run: func() error { return runDbClean([]string{"--db-path", dbPath, "--force"}) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
		})
	}
}

func TestRunServerModeGracefulShutdown(t *testing.T) {
	cfg := &config.Config{}
	*cfg = *config.DefaultConfig()
	cfg.Storage.DBPath = t.TempDir()
	cfg.Server.AutoOpenBrowser = false
	cfg.Server.Port = 0

	go func() {
		time.Sleep(300 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	if err := runServerMode(cfg); err != nil {
		t.Fatalf("runServerMode() error = %v", err)
	}
}

func TestRunCollectModeProcessesInputAndShutsDown(t *testing.T) {
	cfg := &config.Config{}
	*cfg = *config.DefaultConfig()
	cfg.Storage.DBPath = t.TempDir()
	cfg.Server.AutoOpenBrowser = false
	cfg.Server.Port = 0
	cfg.Parsing.Format = "json"

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	go func() {
		_, _ = w.WriteString(`{"timestamp":"2025-01-01T00:00:00Z","level":"INFO","message":"ok"}` + "\n")
		_ = w.Close()
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	if err := runCollectMode(cfg, true); err != nil {
		t.Fatalf("runCollectMode() error = %v", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintHelpersAndMainSubcommands(t *testing.T) {
	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "printVersion", run: func(t *testing.T) {
			if out := captureStdout(t, printVersion); out == "" {
				t.Fatalf("printVersion produced no output")
			}
		}},
		{name: "printHelp", run: func(t *testing.T) {
			if out := captureStdout(t, printHelp); out == "" {
				t.Fatalf("printHelp produced no output")
			}
		}},
		{name: "main version", run: func(t *testing.T) {
			origArgs := os.Args
			defer func() { os.Args = origArgs }()
			os.Args = []string{"peek", "version"}
			if out := captureStdout(t, main); out == "" {
				t.Fatalf("main version output empty")
			}
		}},
		{name: "main help", run: func(t *testing.T) {
			origArgs := os.Args
			defer func() { os.Args = origArgs }()
			os.Args = []string{"peek", "help"}
			if out := captureStdout(t, main); out == "" {
				t.Fatalf("main help output empty")
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.run(t) })
	}
}

func TestIsStdinPipedAndOpenBrowser(t *testing.T) {
	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer func() {
		os.Stdin = orig
		_ = r.Close()
		_ = w.Close()
	}()

	os.Stdin = r
	if !isStdinPiped() {
		t.Fatalf("expected stdin to look piped")
	}

	openBrowser("http://localhost:0")
}

func TestMainDbSubcommands(t *testing.T) {
	dbPath := t.TempDir()
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tests := [][]string{
		{"peek", "db", "stats", "--db-path", dbPath},
		{"peek", "db", "clean", "--db-path", dbPath, "--force"},
	}

	for _, args := range tests {
		os.Args = args
		_ = captureStdout(t, main)
	}
}

func TestMainCollectModeFlow(t *testing.T) {
	dbPath := t.TempDir()
	origArgs := os.Args
	origStdin := os.Stdin
	defer func() {
		os.Args = origArgs
		os.Stdin = origStdin
	}()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r

	os.Args = []string{"peek", "--db-path", dbPath, "--format", "json", "--no-browser", "--port", "0", "--all"}

	go func() {
		_, _ = w.WriteString(`{"timestamp":"2025-01-01T00:00:00Z","level":"INFO","message":"from-main"}` + "\n")
		_ = w.Close()
	}()
	go func() {
		time.Sleep(700 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	main()
}

func TestRunDbCleanValidationAndAbortPaths(t *testing.T) {
	dbPath := t.TempDir()

	if err := runDbClean([]string{"--db-path", dbPath, "--older-than", "nonsense", "--force"}); err == nil {
		t.Fatalf("expected invalid duration error")
	}

	if err := runDbClean([]string{"--db-path", dbPath, "--level", "MISSING", "--force"}); err != nil {
		t.Fatalf("expected no-op clean for missing level, got %v", err)
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		_ = r.Close()
		_ = w.Close()
	}()

	_, _ = w.WriteString("n\n")
	_ = w.Close()
	if err := runDbClean([]string{"--db-path", dbPath}); err != nil {
		t.Fatalf("runDbClean abort path error = %v", err)
	}
}

func TestRunDbStatsConfigError(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "bad.toml")
	if err := os.WriteFile(cfg, []byte("[storage\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := runDbStats([]string{"--config", cfg}); err == nil {
		t.Fatalf("expected config parse error")
	}
}

func TestRunServerModePortInUseReturnsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port

	cfg := config.DefaultConfig()
	cfg.Storage.DBPath = t.TempDir()
	cfg.Server.AutoOpenBrowser = false
	cfg.Server.Port = port

	err = runServerMode(cfg)
	if err == nil {
		t.Fatalf("expected server start error for occupied port")
	}
}

func TestRunDbStatsAndCleanWithExistingLogs(t *testing.T) {
	dbPath := t.TempDir()
	db, err := storage.NewBadgerStorage(storage.Config{DBPath: dbPath, RetentionSize: 1024 * 1024 * 100, RetentionDays: 30})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	if err := db.Store(&storage.LogEntry{ID: "1", Timestamp: time.Now().UTC().Add(-time.Hour), Level: "INFO", Message: "m1", Raw: "m1"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := db.Store(&storage.LogEntry{ID: "2", Timestamp: time.Now().UTC(), Level: "ERROR", Message: "m2", Raw: "m2"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := runDbStats([]string{"--db-path", dbPath}); err != nil {
		t.Fatalf("runDbStats() error = %v", err)
	}

	origStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		_ = r.Close()
		_ = w.Close()
	}()
	_, _ = w.WriteString("y\n")
	_ = w.Close()

	if err := runDbClean([]string{"--db-path", dbPath, "--level", "ERROR"}); err != nil {
		t.Fatalf("runDbClean(confirm yes) error = %v", err)
	}
}

func TestMainFatalPaths(t *testing.T) {
	tests := []struct {
		name string
		args string
	}{
		{name: "unknown command", args: "peek unknowncmd"},
		{name: "positional arg", args: "peek --no-browser extra"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run", "TestMainFatalHelper")
			cmd.Env = append(os.Environ(), "PEEK_FATAL_HELPER_ARGS="+tt.args)
			err := cmd.Run()
			if err == nil {
				t.Fatalf("expected non-zero exit for %s", tt.name)
			}
		})
	}
}

func TestMainFatalHelper(t *testing.T) {
	args := os.Getenv("PEEK_FATAL_HELPER_ARGS")
	if args == "" {
		t.Skip("helper")
	}
	os.Args = strings.Split(args, " ")
	main()
}

func TestRunCollectModeFreshModeWithExistingLogs(t *testing.T) {
	dbPath := t.TempDir()
	db, err := storage.NewBadgerStorage(storage.Config{DBPath: dbPath, RetentionSize: 1024 * 1024 * 100, RetentionDays: 30})
	if err != nil {
		t.Fatalf("NewBadgerStorage() error = %v", err)
	}
	if err := db.Store(&storage.LogEntry{ID: "seed", Timestamp: time.Now().UTC().Add(-time.Minute), Level: "INFO", Message: "seed", Raw: "seed"}); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	cfg := config.DefaultConfig()
	cfg.Storage.DBPath = dbPath
	cfg.Server.AutoOpenBrowser = false
	cfg.Server.Port = 0
	cfg.Parsing.Format = "json"

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdin = r
	defer func() { os.Stdin = orig }()

	go func() {
		_, _ = w.WriteString("not-json\n")
		_, _ = w.WriteString(`{"timestamp":"2025-01-01T00:00:00Z","level":"INFO","message":"ok"}` + "\n")
		_ = w.Close()
	}()

	go func() {
		time.Sleep(500 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(os.Interrupt)
	}()

	if err := runCollectMode(cfg, false); err != nil {
		t.Fatalf("runCollectMode(fresh mode) error = %v", err)
	}
}
