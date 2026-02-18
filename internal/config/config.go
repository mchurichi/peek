package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config holds the application configuration
type Config struct {
	Storage StorageConfig `toml:"storage"`
	Server  ServerConfig  `toml:"server"`
	Parsing ParsingConfig `toml:"parsing"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	RetentionSize string `toml:"retention_size"` // e.g., "1GB", "500MB"
	RetentionDays int    `toml:"retention_days"`
	DBPath        string `toml:"db_path"`
}

// ServerConfig holds server-related configuration
type ServerConfig struct {
	Port            int  `toml:"port"`
	AutoOpenBrowser bool `toml:"auto_open_browser"`
}

// ParsingConfig holds parsing-related configuration
type ParsingConfig struct {
	Format        string `toml:"format"` // auto, json, logfmt
	AutoTimestamp bool   `toml:"auto_timestamp"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Storage: StorageConfig{
			RetentionSize: "1GB",
			RetentionDays: 7,
			DBPath:        filepath.Join(home, ".peek", "db"),
		},
		Server: ServerConfig{
			Port:            8080,
			AutoOpenBrowser: true,
		},
		Parsing: ParsingConfig{
			Format:        "auto",
			AutoTimestamp: true,
		},
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	cfg := DefaultConfig()
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %w", err)
	}

	return cfg, nil
}

// ParseSize parses a size string like "1GB", "500MB" to bytes
func ParseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	var multiplier int64 = 1
	var numStr string

	if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		numStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		numStr = strings.TrimSuffix(sizeStr, "KB")
	} else {
		return 0, fmt.Errorf("invalid size format: %s (use KB, MB, or GB)", sizeStr)
	}

	var num float64
	_, err := fmt.Sscanf(numStr, "%f", &num)
	if err != nil {
		return 0, fmt.Errorf("invalid size number: %s", numStr)
	}

	return int64(num * float64(multiplier)), nil
}

// GetRetentionSizeBytes returns retention size in bytes
func (c *Config) GetRetentionSizeBytes() int64 {
	size, err := ParseSize(c.Storage.RetentionSize)
	if err != nil {
		return 1024 * 1024 * 1024 // Default to 1GB
	}
	return size
}
