// Package handlers provides HTTP handlers for the goback server.
//
// Each handler is in its own file and implements http.Handler.
// Handlers use interfaces to access server dependencies, avoiding
// circular imports.
package handlers

import (
	"github.com/nomis52/goback/config"
	"github.com/nomis52/goback/orchestrator"
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
	Run() error
}

// RunStatusProvider provides access to run status.
type RunStatusProvider interface {
	Status() runner.RunStatus
}

// HistoryProvider provides access to run history.
type HistoryProvider interface {
	History() []runner.RunStatus
}

// ResultsProvider provides access to activity results.
type ResultsProvider interface {
	GetResults() map[orchestrator.ActivityID]*orchestrator.Result
}
