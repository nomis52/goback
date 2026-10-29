package sshclient

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name     string
		setupFn  func(t *testing.T) (addr, privateKey string, cleanup func())
		wantErr  string
		verifyFn func(t *testing.T, client *SSHClient)
	}{
		{
			name: "successful connection",
			setupFn: func(t *testing.T) (string, string, func()) {
				server, addr, clientKey := newTestServer(t, func(ch ssh.Channel, req *ssh.Request) {
					req.Reply(true, nil)
					ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
				})
				return addr, clientKey, server.close
			},
			verifyFn: func(t *testing.T, client *SSHClient) {
				assert.NotNil(t, client)
			},
		},
		{
			name: "invalid private key",
			setupFn: func(t *testing.T) (string, string, func()) {
				return "localhost:22", "invalid-key", func() {}
			},
			wantErr: "failed to parse private key",
		},
		{
			name: "connection refused",
			setupFn: func(t *testing.T) (string, string, func()) {
				// Use a port that's definitely not listening
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				require.NoError(t, err)
				addr := listener.Addr().String()
				listener.Close()

				clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
				require.NoError(t, err)
				clientKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type:  "RSA PRIVATE KEY",
					Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
				})
				return addr, string(clientKeyPEM), func() {}
			},
			wantErr: "failed to dial SSH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, privateKey, cleanup := tt.setupFn(t)
			defer cleanup()

			client, err := New(addr, "testuser", privateKey)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, client)
			} else {
				require.NoError(t, err)
				defer client.Close()
				if tt.verifyFn != nil {
					tt.verifyFn(t, client)
				}
			}
		})
	}
}

func TestSSHClient_Run(t *testing.T) {
	tests := []struct {
		name       string
		handler    func(ch ssh.Channel, req *ssh.Request)
		command    string
		stdin      io.Reader
		useStdout  bool
		useStderr  bool
		wantErr    string
		wantStdout string
		wantStderr string
	}{
		{
			name: "captures stdout",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Write([]byte("hello world\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command:    "echo hello",
			useStdout:  true,
			useStderr:  true,
			wantStdout: "hello world\n",
		},
		{
			name: "captures stderr and reports non-zero exit",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Stderr().Write([]byte("error output\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
			},
			command:    "failing command",
			useStdout:  true,
			useStderr:  true,
			wantErr:    "failed to run command",
			wantStderr: "error output\n",
		},
		{
			name: "captures both stdout and stderr",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Write([]byte("stdout line\n"))
				ch.Stderr().Write([]byte("stderr line\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command:    "mixed output",
			useStdout:  true,
			useStderr:  true,
			wantStdout: "stdout line\n",
			wantStderr: "stderr line\n",
		},
		{
			name: "stdout discarded without WithStdout",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Write([]byte("discarded\n"))
				ch.Stderr().Write([]byte("captured\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command:    "test",
			useStderr:  true,
			wantStderr: "captured\n",
		},
		{
			name: "stderr discarded without WithStderr",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Write([]byte("captured\n"))
				ch.Stderr().Write([]byte("discarded\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command:    "test",
			useStdout:  true,
			wantStdout: "captured\n",
		},
		{
			name: "no options discards all output",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.Write([]byte("discarded\n"))
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command: "test",
		},
		{
			name: "stdin is delivered to the remote command",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				input, _ := io.ReadAll(ch)
				ch.Write([]byte("got: "))
				ch.Write(input)
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
			},
			command:    "cat",
			stdin:      strings.NewReader("secret\n"),
			useStdout:  true,
			wantStdout: "got: secret\n",
		},
		{
			name: "command failure without output",
			handler: func(ch ssh.Channel, req *ssh.Request) {
				req.Reply(true, nil)
				ch.SendRequest("exit-status", false, []byte{0, 0, 0, 1})
			},
			command:   "exit 1",
			useStdout: true,
			useStderr: true,
			wantErr:   "failed to run command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, addr, clientKey := newTestServer(t, tt.handler)
			defer server.close()

			client, err := New(addr, "testuser", clientKey)
			require.NoError(t, err)
			defer client.Close()

			var stdoutBuf, stderrBuf bytes.Buffer
			var opts []RunOption
			if tt.stdin != nil {
				opts = append(opts, WithStdin(tt.stdin))
			}
			if tt.useStdout {
				opts = append(opts, WithStdout(&stdoutBuf))
			}
			if tt.useStderr {
				opts = append(opts, WithStderr(&stderrBuf))
			}

			err = client.Run(tt.command, opts...)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantStdout, stdoutBuf.String())
			assert.Equal(t, tt.wantStderr, stderrBuf.String())
		})
	}
}

func TestSSHClient_Close(t *testing.T) {
	server, addr, clientKey := newTestServer(t, func(ch ssh.Channel, req *ssh.Request) {
		req.Reply(true, nil)
		ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
	})
	defer server.close()

	client, err := New(addr, "testuser", clientKey)
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)

	// Verify connection is actually closed by trying to use it
	err = client.Run("test")
	require.Error(t, err)
}

func TestSSHClient_MultipleCommands(t *testing.T) {
	commandCount := 0
	server, addr, clientKey := newTestServer(t, func(ch ssh.Channel, req *ssh.Request) {
		commandCount++
		req.Reply(true, nil)
		fmt.Fprintf(ch, "command %d\n", commandCount)
		ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
	})
	defer server.close()

	client, err := New(addr, "testuser", clientKey)
	require.NoError(t, err)
	defer client.Close()

	var stdout1 bytes.Buffer
	err = client.Run("first", WithStdout(&stdout1))
	require.NoError(t, err)
	assert.Equal(t, "command 1\n", stdout1.String())

	var stdout2 bytes.Buffer
	err = client.Run("second", WithStdout(&stdout2))
	require.NoError(t, err)
	assert.Equal(t, "command 2\n", stdout2.String())
}

// Test helpers

// testServer is a minimal SSH server for testing.
type testServer struct {
	listener   net.Listener
	config     *ssh.ServerConfig
	handler    func(ch ssh.Channel, req *ssh.Request)
	acceptDone chan struct{}
}

func newTestServer(t *testing.T, handler func(ch ssh.Channel, req *ssh.Request)) (*testServer, string, string) {
	t.Helper()

	// Generate server host key
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	require.NoError(t, err)

	// Generate client key pair
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	clientSigner, err := ssh.NewSignerFromKey(clientKey)
	require.NoError(t, err)

	// Encode client private key to PEM
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})

	config := &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if string(pubKey.Marshal()) == string(clientSigner.PublicKey().Marshal()) {
				return nil, nil
			}
			return nil, fmt.Errorf("unknown public key")
		},
	}
	config.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	s := &testServer{
		listener:   listener,
		config:     config,
		handler:    handler,
		acceptDone: make(chan struct{}),
	}

	go s.serve(t)

	return s, listener.Addr().String(), string(clientKeyPEM)
}

func (s *testServer) serve(t *testing.T) {
	defer close(s.acceptDone)

	conn, err := s.listener.Accept()
	if err != nil {
		return // listener closed
	}

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.config)
	if err != nil {
		t.Logf("SSH handshake failed: %v", err)
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}

		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer ch.Close()
			for req := range reqs {
				if req.Type == "exec" {
					s.handler(ch, req)
					return
				}
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
		}(channel, requests)
	}
}

func (s *testServer) close() {
	s.listener.Close()
	<-s.acceptDone
}
