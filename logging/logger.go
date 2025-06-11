// Package logging provides structured logging for goback.
// It uses Go's standard library slog package for high-performance structured logging
// with support for different output formats and log levels.
//
// Example usage:
//
//	logger := logging.New(logging.Config{
//		Level:  "info",
//		Format: "json",
//	})
//	logger.Info("backup started", "vm_id", 100, "vm_name", "web-server")
//	logger.Error("backup failed", "vm_id", 100, "error", err)
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"
)

// Config holds the configuration for the logger.
type Config struct {
	// Level sets the minimum log level. Valid values: debug, info, warn, error
	Level string `yaml:"level"`
	// Format sets the output format. Valid values: json, text
	Format string `yaml:"format"`
	// Output sets the output destination. Valid values: stdout, stderr, or a file path
	Output string `yaml:"output"`
	// AddSource adds source code position to log records
	AddSource bool `yaml:"add_source"`
}

// Logger wraps slog.Logger
type Logger struct {
	*slog.Logger
	config Config
}

// New creates a new logger with the given configuration.
func New(cfg Config) (*Logger, error) {
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid logging config: %w", err)
	}

	// Set defaults
	cfg.setDefaults()

	// Parse log level
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %q: %w", cfg.Level, err)
	}

	// Get output writer
	writer, err := getWriter(cfg.Output)
	if err != nil {
		return nil, fmt.Errorf("failed to get output writer: %w", err)
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Customize timestamp format
			if a.Key == slog.TimeKey {
				return slog.String(slog.TimeKey, a.Value.Time().Format(time.RFC3339))
			}
			return a
		},
	}

	// Create handler based on format
	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(writer, opts)
	case "text":
		handler = slog.NewTextHandler(writer, opts)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", cfg.Format)
	}

	logger := slog.New(handler)

	return &Logger{
		Logger: logger,
		config: cfg,
	}, nil
}

// validate checks if the configuration is valid.
func (cfg *Config) validate() error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if cfg.Level != "" && !slices.Contains(validLevels, cfg.Level) {
		return fmt.Errorf("level must be one of: %s", strings.Join(validLevels, ", "))
	}

	validFormats := []string{"json", "text"}
	if cfg.Format != "" && !slices.Contains(validFormats, cfg.Format) {
		return fmt.Errorf("format must be one of: %s", strings.Join(validFormats, ", "))
	}

	return nil
}

// setDefaults sets default values for unset configuration fields.
func (cfg *Config) setDefaults() {
	if cfg.Level == "" {
		cfg.Level = "info"
	}
	if cfg.Format == "" {
		cfg.Format = "json"
	}
	if cfg.Output == "" {
		cfg.Output = "stdout"
	}
}

// parseLevel converts a string level to slog.Level.
func parseLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown level: %s", level)
	}
}

// getWriter returns an io.Writer for the given output configuration.
func getWriter(output string) (io.Writer, error) {
	switch output {
	case "stdout":
		return os.Stdout, nil
	case "stderr":
		return os.Stderr, nil
	default:
		// Assume it's a file path
		file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %q: %w", output, err)
		}
		return file, nil
	}
}
