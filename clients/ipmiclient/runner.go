package ipmiclient

import "os/exec"

// CommandRunner executes external commands and returns their output
type CommandRunner interface {
	Run(name string, args ...string) ([]byte, error)
}

// execCommandRunner is the default implementation using os/exec
type execCommandRunner struct{}

func (e *execCommandRunner) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
