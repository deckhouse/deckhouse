/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"report-updater/internal/cache"
	"report-updater/internal/web/hook"
)

const (
	// Webhook tls certificates
	sslWebhookPath = "/etc/ssl/report-updater/"
	sslListenCert  = sslWebhookPath + "tls.crt"
	sslListenKey   = sslWebhookPath + "tls.key"

	// CA to verify kube-apiserver client certificate
	authClientCA = sslWebhookPath + "ca.crt"

	listenAddr = "0.0.0.0:40443"
)

func buildTLSConfig() (*tls.Config, error) {
	clientCertPool := x509.NewCertPool()

	{ // kube-apiserver requests
		clientCertBytes, err := os.ReadFile(authClientCA)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", authClientCA, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}
	{ // kubelet liveness probe requests
		clientCertBytes, err := os.ReadFile(sslListenCert)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", sslListenCert, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}

	return &tls.Config{
		ClientAuth: tls.NoClientCert,
		ClientCAs:  clientCertPool,
	}, nil
}

type Server struct {
	handler *hook.Handler
	logger  *log.Logger
}

type ServerConfig struct {
	Logger          *log.Logger
	HandlerSettings hook.HandlerSettings
}

func NewServer(config *ServerConfig) (*Server, error) {
	c, err := cache.New(context.Background(), config.Logger)
	if err != nil {
		return nil, err
	}

	h, err := hook.NewHandler(config.Logger, c, &config.HandlerSettings)
	if err != nil {
		return nil, err
	}

	return &Server{logger: config.Logger, handler: h}, nil
}

func (s *Server) prepareHTTPServer() (*http.Server, error) {
	router := http.NewServeMux()

	router.Handle("/mutate-vulnerability-report", s.handler)
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		err := s.handler.CheckBDU()
		if err == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	})

	tlsCfg, err := buildTLSConfig()
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:         listenAddr,
		TLSConfig:    tlsCfg,
		Handler:      router,
		ErrorLog:     s.logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return srv, nil
}

// Run starts webhook server and its configuration renewal. It exits only if the webserver stops listening.
func (s *Server) Run(ctx context.Context) error {
	httpServer, err := s.prepareHTTPServer()
	if err != nil {
		return err
	}

	s.logger.Printf("server is starting to listen on '%s' ...\n", listenAddr)

	go s.handler.StartRenewCacheLoop(ctx)

	if err = httpServer.ListenAndServeTLS(sslListenCert, sslListenKey); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %w", listenAddr, err)
	}

	return nil
}
