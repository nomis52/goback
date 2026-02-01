// Package handlers provides HTTP handlers for the goback server.
//
// Each handler is in its own file and implements http.Handler.
// Handlers use interfaces to access server dependencies, avoiding
// circular imports.
package handlers

import (
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/server/runner"
)

// ConfigProvider provides access to the current configuration.
type ConfigProvider interface {
	Config() *config.Config
}

// Reloader can reload its configuration.
type Reloader interface {
	Reload() error
}

// BackupRunner can start backup runs.
type BackupRunner interface {
	Run(workflows []string) error
}

// HistoryProvider provides access to run history.
type HistoryProvider interface {
	History() []runner.RunSummary
	GetLogs(string) ([]runner.ActivityExecution, error)
}
