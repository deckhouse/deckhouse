/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	logger = log.New(os.Stdout, "http: ", log.LstdFlags)
)

func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func httpHandlerReady(p *Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := p.probeClient.Get("https://kubernetes.default.svc." + os.Getenv("CLUSTER_DOMAIN") + "/version")
		if err != nil {
			logger.Printf("[api-proxy] Readiness probe error: %v\n", err)
			http.Error(w, "Error", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, "Ok.")
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}
}

func httpHandlerApiProxy(p *Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// check if request was passed by ingress
		if len(r.TLS.PeerCertificates) == 0 {
			errstring := "[api-proxy] Only requests with client certificate are allowed."
			http.Error(w, errstring, http.StatusUnauthorized)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
			return
		}

		if r.TLS.PeerCertificates[0].Subject.Organization[0] != "ingress-nginx:auth" {
			errstring := "[api-proxy] Only requests from ingress are allowed."
			http.Error(w, errstring, http.StatusUnauthorized)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
			return
		}

		err := p.CheckAuthn(r.Header, "api")
		if err != nil {
			errstring := "[api-proxy] Authentication error: " + err.Error()
			http.Error(w, errstring, http.StatusUnauthorized)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, http.StatusUnauthorized, errstring)
			return
		}

		p.reverseProxy.ServeHTTP(w, r)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}
}

func main() {
	listenAddr := "0.0.0.0:4443"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg := &sync.WaitGroup{}

	proxy, err := NewProxy("d8-istio")
	if err != nil {
		logger.Println("[api-proxy] Error creating proxy:", err)
		return
	}

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/ready", http.HandlerFunc(httpHandlerReady(proxy)))
	router.Handle("/", http.HandlerFunc(httpHandlerApiProxy(proxy)))

	kubeCA, err := os.ReadFile("/etc/ssl/kube-rbac-proxy-ca.crt")
	if err != nil {
		logger.Printf("[api-proxy] Could not read CA certificate: %v\n", err)
		return
	}
	kubeCertPool := x509.NewCertPool()

	if !kubeCertPool.AppendCertsFromPEM(kubeCA) {
		logger.Println("[api-proxy] Could not parse CA certificate")
		return
	}

	server := &http.Server{
		Addr:     listenAddr,
		Handler:  router,
		ErrorLog: logger,
		TLSConfig: &tls.Config{
			// Allow unauthenticated requests to probes, but check certificates from ingress.
			// Additional subject check is in httpHandlerApiProxy func.
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    kubeCertPool,
			Certificates: []tls.Certificate{*proxy.serverCert},
		},
	}

	stop := make(chan os.Signal, 2)
	errChan := make(chan error, 2)

	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := proxy.Watch(ctx); err != nil {
			logger.Println("[api-proxy] Error watching proxy:", err)
			errChan <- err
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Println("Server is starting to listen on", listenAddr, "...")
		if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			logger.Printf("Could not listen on %s: %v\n", listenAddr, err)
			errChan <- err
		}
	}()

	// wait stop
	select {
	case <-stop:
		logger.Println("Server is shutting down...")
	case err := <-errChan:
		logger.Println("[api-proxy] Error watching proxy:", err)
	}

	logger.Println("Shutdown signal received. Shutting down server...")

	// graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Printf("Server forced to shutdown: %v", err)
	}

	wg.Wait()
	close(errChan)

	logger.Println("Server gracefully stopped")
}
