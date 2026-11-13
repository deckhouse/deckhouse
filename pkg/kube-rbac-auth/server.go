// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kuberbacauth

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	certutil "k8s.io/client-go/util/cert"
)

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// ServerConfig configures an HTTPS server that takes the place of the kube-rbac-proxy listener.
type ServerConfig struct {
	// Address to listen on, for example ":8443" or "$(POD_IP):8443".
	Address string
	// Handler is usually Middleware.Handler wrapped around the metrics handler.
	Handler http.Handler
	// CertFile and KeyFile are PEM files, reread when they change. Without them the server uses a
	// self-signed certificate, as kube-rbac-proxy does, so Prometheus scrapes with insecureSkipVerify.
	CertFile string
	KeyFile  string
	// ShutdownTimeout bounds the graceful shutdown after ctx is done. Zero means 5 seconds.
	ShutdownTimeout time.Duration
	// Logger defaults to slog.Default().
	Logger *slog.Logger
}

// ListenAndServeTLS serves until ctx is done and then shuts the server down gracefully.
func ListenAndServeTLS(ctx context.Context, cfg ServerConfig) error {
	if cfg.Handler == nil {
		return errors.New("handler is required")
	}
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("certFile and keyFile must be set together")
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CertFile == "" {
		cert, err := selfSignedCertificate()
		if err != nil {
			return err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		log.Info("serving with a self-signed certificate")
	} else {
		loader := &certLoader{certFile: cfg.CertFile, keyFile: cfg.KeyFile}
		if _, err := loader.GetCertificate(nil); err != nil {
			return err
		}
		tlsConfig.GetCertificate = loader.GetCertificate
	}

	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Address, err)
	}
	return serveTLS(ctx, ln, cfg.Handler, tlsConfig, cfg.ShutdownTimeout, log)
}

func serveTLS(ctx context.Context, ln net.Listener, h http.Handler, tlsConfig *tls.Config, shutdownTimeout time.Duration, log *slog.Logger) error {
	if shutdownTimeout == 0 {
		shutdownTimeout = 5 * time.Second
	}
	srv := &http.Server{
		Handler:           h,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelDebug),
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ServeTLS(ln, "", "") }()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down the server: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func selfSignedCertificate() (tls.Certificate, error) {
	host, err := os.Hostname()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("get the hostname for a self-signed certificate: %w", err)
	}
	certPEM, keyPEM, err := certutil.GenerateSelfSignedCertKey(host, nil, nil)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate a self-signed certificate: %w", err)
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// certLoader rereads the key pair when either file changes, so a renewed certificate
// is picked up without a restart.
type certLoader struct {
	certFile, keyFile string

	mu       sync.Mutex
	cert     *tls.Certificate
	certTime time.Time
	keyTime  time.Time
}

func (l *certLoader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	certInfo, err := os.Stat(l.certFile)
	if err != nil {
		return l.fallback(err)
	}
	keyInfo, err := os.Stat(l.keyFile)
	if err != nil {
		return l.fallback(err)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cert != nil && certInfo.ModTime().Equal(l.certTime) && keyInfo.ModTime().Equal(l.keyTime) {
		return l.cert, nil
	}
	cert, err := tls.LoadX509KeyPair(l.certFile, l.keyFile)
	if err != nil {
		if l.cert != nil {
			// The files may be caught halfway through a rotation; keep serving the previous pair.
			return l.cert, nil
		}
		return nil, fmt.Errorf("load the certificate: %w", err)
	}
	l.cert, l.certTime, l.keyTime = &cert, certInfo.ModTime(), keyInfo.ModTime()
	return l.cert, nil
}

func (l *certLoader) fallback(err error) (*tls.Certificate, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cert != nil {
		return l.cert, nil
	}
	return nil, fmt.Errorf("read the certificate: %w", err)
}
