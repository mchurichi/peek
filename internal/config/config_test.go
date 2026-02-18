package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	// Check storage defaults
	if cfg.Storage.RetentionSize != "1GB" {
		t.Errorf("DefaultConfig() Storage.RetentionSize = %v, want 1GB", cfg.Storage.RetentionSize)
	}
	if cfg.Storage.RetentionDays != 7 {
		t.Errorf("DefaultConfig() Storage.RetentionDays = %v, want 7", cfg.Storage.RetentionDays)
	}
	if cfg.Storage.DBPath == "" {
		t.Error("DefaultConfig() Storage.DBPath is empty")
	}

	// Check server defaults
	if cfg.Server.Port != 8080 {
		t.Errorf("DefaultConfig() Server.Port = %v, want 8080", cfg.Server.Port)
	}
	if cfg.Server.AutoOpenBrowser != true {
		t.Errorf("DefaultConfig() Server.AutoOpenBrowser = %v, want true", cfg.Server.AutoOpenBrowser)
	}

	// Check parsing defaults
	if cfg.Parsing.Format != "auto" {
		t.Errorf("DefaultConfig() Parsing.Format = %v, want auto", cfg.Parsing.Format)
	}
	if cfg.Parsing.AutoTimestamp != true {
		t.Errorf("DefaultConfig() Parsing.AutoTimestamp = %v, want true", cfg.Parsing.AutoTimestamp)
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	// Loading a non-existent file should return default config, not an error
	cfg, err := Load("/nonexistent/config.toml")
	if err != nil {
		t.Errorf("Load() with non-existent file returned error = %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// Should match default config
	defaultCfg := DefaultConfig()
	if cfg.Storage.RetentionSize != defaultCfg.Storage.RetentionSize {
		t.Errorf("Load() non-existent file Storage.RetentionSize = %v, want %v", cfg.Storage.RetentionSize, defaultCfg.Storage.RetentionSize)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[storage]
retention_size = "2GB"
retention_days = 14
db_path = "/custom/db/path"

[server]
port = 9090
auto_open_browser = false

[parsing]
format = "json"
auto_timestamp = false
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded values
	if cfg.Storage.RetentionSize != "2GB" {
		t.Errorf("Load() Storage.RetentionSize = %v, want 2GB", cfg.Storage.RetentionSize)
	}
	if cfg.Storage.RetentionDays != 14 {
		t.Errorf("Load() Storage.RetentionDays = %v, want 14", cfg.Storage.RetentionDays)
	}
	if cfg.Storage.DBPath != "/custom/db/path" {
		t.Errorf("Load() Storage.DBPath = %v, want /custom/db/path", cfg.Storage.DBPath)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Load() Server.Port = %v, want 9090", cfg.Server.Port)
	}
	if cfg.Server.AutoOpenBrowser != false {
		t.Errorf("Load() Server.AutoOpenBrowser = %v, want false", cfg.Server.AutoOpenBrowser)
	}
	if cfg.Parsing.Format != "json" {
		t.Errorf("Load() Parsing.Format = %v, want json", cfg.Parsing.Format)
	}
	if cfg.Parsing.AutoTimestamp != false {
		t.Errorf("Load() Parsing.AutoTimestamp = %v, want false", cfg.Parsing.AutoTimestamp)
	}
}

func TestLoad_PartialFile(t *testing.T) {
	// Create a config file with only some fields
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[storage]
retention_size = "500MB"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify overridden value
	if cfg.Storage.RetentionSize != "500MB" {
		t.Errorf("Load() Storage.RetentionSize = %v, want 500MB", cfg.Storage.RetentionSize)
	}

	// Verify default values for unspecified fields
	defaultCfg := DefaultConfig()
	if cfg.Storage.RetentionDays != defaultCfg.Storage.RetentionDays {
		t.Errorf("Load() Storage.RetentionDays = %v, want default %v", cfg.Storage.RetentionDays, defaultCfg.Storage.RetentionDays)
	}
	if cfg.Server.Port != defaultCfg.Server.Port {
		t.Errorf("Load() Server.Port = %v, want default %v", cfg.Server.Port, defaultCfg.Server.Port)
	}
}

func TestLoad_InvalidFile(t *testing.T) {
	// Create an invalid TOML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	invalidContent := `
[storage
retention_size = "1GB"
`

	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Load() with invalid TOML should return error")
	}
}

func TestLoad_HomeDirectoryExpansion(t *testing.T) {
	// Test that ~ is expanded to home directory
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get user home directory")
	}

	// Create a unique temp directory under home so we never collide
	// with pre-existing data or concurrent test runs.
	configDir, err := os.MkdirTemp(home, ".peek-test-config-*")
	if err != nil {
		t.Skip("Cannot create test config directory")
	}
	defer os.RemoveAll(configDir)

	configPath := filepath.Join(configDir, "config.toml")
	configContent := `
[storage]
retention_size = "1GB"
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Skip("Cannot create test config file")
	}

	// Load with ~ path — only the directory basename varies
	dirName := filepath.Base(configDir)
	tildeConfigPath := "~/" + dirName + "/config.toml"
	cfg, err := Load(tildeConfigPath)
	if err != nil {
		t.Fatalf("Load() with ~ path error = %v", err)
	}

	if cfg.Storage.RetentionSize != "1GB" {
		t.Errorf("Load() with ~ path failed to load config correctly")
	}
}

func TestParseSize(t *testing.T) {
	tests := []struct {
		name    string
		sizeStr string
		want    int64
		wantErr bool
	}{
		{
			name:    "1GB",
			sizeStr: "1GB",
			want:    1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "500MB",
			sizeStr: "500MB",
			want:    500 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "100KB",
			sizeStr: "100KB",
			want:    100 * 1024,
			wantErr: false,
		},
		{
			name:    "lowercase gb",
			sizeStr: "2gb",
			want:    2 * 1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "with spaces",
			sizeStr: "  1GB  ",
			want:    1024 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "decimal value",
			sizeStr: "1.5GB",
			want:    int64(1.5 * 1024 * 1024 * 1024),
			wantErr: false,
		},
		{
			name:    "0.5GB",
			sizeStr: "0.5GB",
			want:    512 * 1024 * 1024,
			wantErr: false,
		},
		{
			name:    "invalid format - no unit",
			sizeStr: "1000",
			wantErr: true,
		},
		{
			name:    "invalid format - unknown unit",
			sizeStr: "1TB",
			wantErr: true,
		},
		{
			name:    "invalid format - non-numeric",
			sizeStr: "XGB",
			wantErr: true,
		},
		{
			name:    "empty string",
			sizeStr: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSize(tt.sizeStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_GetRetentionSizeBytes(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
		want int64
	}{
		{
			name: "valid 1GB",
			cfg: &Config{
				Storage: StorageConfig{
					RetentionSize: "1GB",
				},
			},
			want: 1024 * 1024 * 1024,
		},
		{
			name: "valid 500MB",
			cfg: &Config{
				Storage: StorageConfig{
					RetentionSize: "500MB",
				},
			},
			want: 500 * 1024 * 1024,
		},
		{
			name: "invalid format - defaults to 1GB",
			cfg: &Config{
				Storage: StorageConfig{
					RetentionSize: "invalid",
				},
			},
			want: 1024 * 1024 * 1024, // Default
		},
		{
			name: "empty string - defaults to 1GB",
			cfg: &Config{
				Storage: StorageConfig{
					RetentionSize: "",
				},
			},
			want: 1024 * 1024 * 1024, // Default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.GetRetentionSizeBytes(); got != tt.want {
				t.Errorf("Config.GetRetentionSizeBytes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	// Create an empty config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	err := os.WriteFile(configPath, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() empty file error = %v", err)
	}

	// Should return default config
	defaultCfg := DefaultConfig()
	if cfg.Storage.RetentionSize != defaultCfg.Storage.RetentionSize {
		t.Errorf("Load() empty file should return defaults")
	}
}
