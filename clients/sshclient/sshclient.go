package sshclient

import (
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages a persistent SSH connection for running multiple commands.
type SSHClient struct {
	client *ssh.Client
}

// RunOption configures how Run executes a remote command.
type RunOption func(*runOptions)

// WithStdin connects the reader to the remote command's standard input.
// The remote command sees EOF once the reader is exhausted. By default the
// remote command receives no input.
func WithStdin(r io.Reader) RunOption {
	return func(o *runOptions) {
		o.stdin = r
	}
}

// WithStdout streams the remote command's standard output to the writer.
// By default standard output is discarded.
func WithStdout(w io.Writer) RunOption {
	return func(o *runOptions) {
		o.stdout = w
	}
}

// WithStderr streams the remote command's standard error to the writer.
// By default standard error is discarded.
func WithStderr(w io.Writer) RunOption {
	return func(o *runOptions) {
		o.stderr = w
	}
}

// New creates a new SSHClient connected to the given host with the provided user and private key (PEM format).
func New(host, user, privateKeyPEM string) (*SSHClient, error) {
	signer, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // NOTE: for production, use a proper callback
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	return &SSHClient{client: client}, nil
}

// Run executes a command on the remote host using a new session on the existing connection.
// Standard input, output and error are wired according to the supplied options; output
// streams that are not configured are discarded. Returns an error if the session cannot
// be created or the command exits non-zero.
func (c *SSHClient) Run(command string, opts ...RunOption) error {
	options := runOptions{
		stdout: io.Discard,
		stderr: io.Discard,
	}
	for _, opt := range opts {
		opt(&options)
	}
	if options.stdout == nil {
		options.stdout = io.Discard
	}
	if options.stderr == nil {
		options.stderr = io.Discard
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdin = options.stdin
	session.Stdout = options.stdout
	session.Stderr = options.stderr

	if err := session.Run(command); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

// Close closes the underlying SSH connection.
func (c *SSHClient) Close() error {
	return c.client.Close()
}

// runOptions holds the per-command settings assembled from RunOption values.
type runOptions struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}
