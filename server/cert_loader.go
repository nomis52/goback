package server

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// CertLoader handles dynamic loading of TLS certificates.
// It checks the file modification time to reload the certificate when it changes.
type CertLoader struct {
	certFile string
	keyFile  string
	logger   *slog.Logger

	mu        sync.RWMutex
	cert      *tls.Certificate
	loadedAt  time.Time
	lastCheck time.Time
}

// NewCertLoader creates a new CertLoader.
func NewCertLoader(certFile, keyFile string, logger *slog.Logger) (*CertLoader, error) {
	loader := &CertLoader{
		certFile: certFile,
		keyFile:  keyFile,
		logger:   logger,
	}

	// Initial load
	if err := loader.reload(); err != nil {
		return nil, err
	}

	return loader, nil
}

// GetCertificate is a callback for tls.Config.GetCertificate.
func (l *CertLoader) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	l.mu.RLock()
	// Check if we need to reload (limit checks to every 1 minute to avoid excessive syscalls)
	// For dev/testing, 1 minute is fine. For production, maybe longer or use fsnotify.
	// Given the requirement is just "picked up", polling start of request is simple and robust.
	if time.Since(l.lastCheck) < 1*time.Minute {
		defer l.mu.RUnlock()
		return l.cert, nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()

	// Double check after lock
	if time.Since(l.lastCheck) < 1*time.Minute {
		return l.cert, nil
	}
	l.lastCheck = time.Now()

	// Check file mod times
	certStat, err := os.Stat(l.certFile)
	if err != nil {
		l.logger.Error("failed to stat cert file", "error", err)
		return l.cert, nil // Return old cert on error
	}
	keyStat, err := os.Stat(l.keyFile)
	if err != nil {
		l.logger.Error("failed to stat key file", "error", err)
		return l.cert, nil
	}

	if certStat.ModTime().After(l.loadedAt) || keyStat.ModTime().After(l.loadedAt) {
		if err := l.reload(); err != nil {
			l.logger.Error("failed to reload certificate", "error", err)
			return l.cert, nil // Return old cert on error
		}
	}

	return l.cert, nil
}

func (l *CertLoader) reload() error {
	cert, err := tls.LoadX509KeyPair(l.certFile, l.keyFile)
	if err != nil {
		return fmt.Errorf("failed to load key pair: %w", err)
	}

	l.cert = &cert
	l.loadedAt = time.Now()
	l.logger.Info("loaded tls certificate", "cert", l.certFile, "key", l.keyFile)
	return nil
}
