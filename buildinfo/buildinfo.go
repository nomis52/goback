// Package buildinfo provides build-time properties injected via ldflags.
package buildinfo

// Properties holds build-time properties injected via ldflags.
type Properties struct {
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
}

// Package-level variables for ldflags injection (unexported).
var (
	buildTime = "unknown"
	gitCommit = "unknown"
)

// Get returns the current build properties.
func Get() Properties {
	return Properties{
		BuildTime: buildTime,
		GitCommit: gitCommit,
	}
}
