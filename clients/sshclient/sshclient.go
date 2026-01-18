package sshclient

import (
	"bytes"
	"fmt"
	"io"

	"golang.org/x/crypto/ssh"
)

// SSHClient manages a persistent SSH connection for running multiple commands.
type SSHClient struct {
	client *ssh.Client
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
func (c *SSHClient) Run(command string) (string, string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	if err := session.Run(command); err != nil {
		return stdoutBuf.String(), stderrBuf.String(), fmt.Errorf("failed to run command: %w", err)
	}

	return stdoutBuf.String(), stderrBuf.String(), nil
}

// RunWithWriter executes a command on the remote host and streams stdout/stderr to the provided writers.
// If stdoutWriter or stderrWriter is nil, that stream will be discarded.
// Returns any error from command execution.
func (c *SSHClient) RunWithWriter(command string, stdoutWriter, stderrWriter io.Writer) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	if stdoutWriter != nil {
		session.Stdout = stdoutWriter
	}
	if stderrWriter != nil {
		session.Stderr = stderrWriter
	}

	if err := session.Run(command); err != nil {
		return fmt.Errorf("failed to run command: %w", err)
	}

	return nil
}

// Close closes the underlying SSH connection.
func (c *SSHClient) Close() error {
	return c.client.Close()
}
