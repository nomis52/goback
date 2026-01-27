// Package types provides shared types for the server package and its subpackages.
package types

import (
	"time"

	"github.com/nomis52/goback/buildinfo"
)

// ServerProperties holds metadata about the running server instance.
type ServerProperties struct {
	Build     buildinfo.Properties `json:"build"`
	StartedAt time.Time            `json:"started_at"`
	Hostname  string               `json:"hostname"`
}
