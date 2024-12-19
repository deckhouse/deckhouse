/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package web

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"user-authz-webhook/cache"
	"user-authz-webhook/web/hook"
)

const (
	// Webhook tls certificates
	sslWebhookPath = "/etc/ssl/user-authz-webhook/"
	sslListenCert  = sslWebhookPath + "tls.crt"
	sslListenKey   = sslWebhookPath + "tls.key"

	// CA to verify kube-apiserver client certificate
	// https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#authenticate-apiservers
	authClientCA = "/etc/ssl/apiserver-authentication-requestheader-client-ca/ca.crt"

	ListenAddr = "127.0.0.1:40443"
)

func buildTLSConfig() (*tls.Config, error) {
	clientCertPool := x509.NewCertPool()

	{ // kube-apiserver requests
		clientCertBytes, err := os.ReadFile(authClientCA)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %v", authClientCA, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}
	{ // kubelet liveness probe requests
		clientCertBytes, err := os.ReadFile(sslListenCert)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %v", sslListenCert, err)
		}
		clientCertPool.AppendCertsFromPEM(clientCertBytes)
	}

	return &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
	}, nil
}

type Server struct {
	cache   cache.Cache
	handler *hook.Handler
	logger  *log.Logger
}

func NewServer(l *log.Logger) (*Server, error) {
	c := cache.NewNamespacedDiscoveryCache(l)
	h, err := hook.NewHandler(l, c)
	if err != nil {
		return nil, err
	}
	return &Server{logger: l, cache: c, handler: h}, nil
}

func (s *Server) prepareHTTPServer() (*http.Server, error) {
	router := http.NewServeMux()

	router.Handle("/", s.handler)
	router.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		err := s.cache.Check()
		if err == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ok."))
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
		Addr:         ListenAddr,
		TLSConfig:    tlsCfg,
		Handler:      router,
		ErrorLog:     s.logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	return srv, nil
}

// Run starts webhook server and its configuration renewal. It will exit only if the webserver stops listening.
func (s *Server) Run() error {
	httpServer, err := s.prepareHTTPServer()
	if err != nil {
		return err
	}

	s.logger.Println("server is starting to listen on ", ListenAddr, "...")

	// Register and stop config updater
	stopCh := make(chan struct{})
	go s.handler.StartRenewConfigLoop(stopCh)

	httpServer.RegisterOnShutdown(func() {
		stopCh <- struct{}{}
	})

	if err := httpServer.ListenAndServeTLS(sslListenCert, sslListenKey); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", ListenAddr, err)
	}

	return nil
}
