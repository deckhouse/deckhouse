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
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	utilcache "k8s.io/apimachinery/pkg/util/cache"
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
	// ClientCAs returns the CAs of the accepted client certificates, usually Middleware.ClientCAs.
	// The server names them in the handshake, so a client with several certificates sends the right one.
	// Without it, or when it returns nil, clients are asked for any certificate.
	ClientCAs func() *x509.CertPool
	// IdleTimeout closes a keep-alive connection without requests, so abandoned connections do not pile up
	// in the application. Zero means 2 minutes. Behind a proxy that reuses connections it must be longer
	// than the idle timeout of the proxy.
	IdleTimeout time.Duration
	// ShutdownTimeout bounds the graceful shutdown after ctx is done. Zero means 5 seconds.
	ShutdownTimeout time.Duration
	// Logger defaults to slog.Default(), also when it holds a nil pointer.
	Logger Logger
}

// ListenAndServeTLS serves until ctx is done and then shuts the server down gracefully.
func ListenAndServeTLS(ctx context.Context, cfg ServerConfig) error {
	if cfg.Handler == nil {
		return errors.New("handler is required")
	}
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return errors.New("certFile and keyFile must be set together")
	}
	log := loggerOrDefault(cfg.Logger)

	tlsConfig, err := newTLSConfig(cfg, log)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.Address, err)
	}
	return serveTLS(ctx, ln, cfg, tlsConfig, log)
}

func newTLSConfig(cfg ServerConfig, log Logger) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CertFile == "" {
		cert, err := selfSignedCertificate()
		if err != nil {
			return nil, err
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
		log.Info("serving with a self-signed certificate")
	} else {
		pair, err := newServingCertificate(cfg.CertFile, cfg.KeyFile, realClock{}, log)
		if err != nil {
			return nil, err
		}
		tlsConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, _ := pair.get()
			return cert, nil
		}
	}
	// Always ask for a client certificate, as kube-rbac-proxy does, so that Config.ClientCAFile
	// needs no second setting. A client without a certificate is still served.
	// The handshake only proves that the client owns the key.
	// The middleware checks who signed the certificate.
	tlsConfig.ClientAuth = tls.RequestClientCert
	if cfg.ClientCAs != nil {
		// The server adds these protocols to its own configuration only.
		// A configuration returned for a handshake has to name them itself to keep HTTP/2.
		tlsConfig.NextProtos = []string{"h2", "http/1.1"}
		// The CAs are taken for every handshake, so the list follows a reloaded bundle.
		base := tlsConfig.Clone()
		tlsConfig.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			cas := cfg.ClientCAs()
			if cas == nil {
				return nil, nil
			}
			named := base.Clone()
			named.ClientCAs = cas
			// Without HTTP/2, for example with GODEBUG=http2server=0, the server takes h2 out of its own list only.
			// A client that agrees to h2 here gets a connection that nothing serves.
			if srv, ok := hello.Context().Value(http.ServerContextKey).(*http.Server); ok && srv.TLSNextProto["h2"] == nil {
				named.NextProtos = []string{"http/1.1"}
			}
			return named, nil
		}
	}
	return tlsConfig, nil
}

func serveTLS(ctx context.Context, ln net.Listener, cfg ServerConfig, tlsConfig *tls.Config, log Logger) error {
	shutdownTimeout := cfg.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 5 * time.Second
	}
	idleTimeout := cfg.IdleTimeout
	if idleTimeout == 0 {
		idleTimeout = 2 * time.Minute
	}
	srv := &http.Server{
		Handler:           cfg.Handler,
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       idleTimeout,
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

// newServingCertificate loads the key pair of the server and keeps it up to date.
func newServingCertificate(certFile, keyFile string, clock utilcache.Clock, log Logger) (*reloader[tls.Certificate], error) {
	return newReloader("serving certificate", []string{certFile, keyFile}, func(contents [][]byte) (*tls.Certificate, error) {
		cert, err := tls.X509KeyPair(contents[0], contents[1])
		if err != nil {
			return nil, err
		}
		return &cert, nil
	}, clock, log)
}
