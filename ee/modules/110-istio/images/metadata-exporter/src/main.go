/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var logger = log.New(os.Stdout, "http: ", log.LstdFlags)

func httpHandlerPublicJSON(exp *Exporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		publicMetaData := exp.RenderPublicMetadataJSON()
		fmt.Fprint(w, publicMetaData)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}
}

func httpHandlerFederationPrivateJSON(exp *Exporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := exp.CheckAuthn(r.Header, "private-federation")
		if err != nil {
			http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, err)
			return
		}

		privateMetadataJSON := exp.RenderFederationPrivateMetadataJSON()
		fmt.Fprint(w, privateMetadataJSON)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}

}

func httpHandlerMulticlusterPrivateJSON(exp *Exporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := exp.CheckAuthn(r.Header, "private-multicluster")
		if err != nil {
			http.Error(w, "Authentication error: "+err.Error(), http.StatusUnauthorized)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, err)
			return
		}
		privateMetadataJSON := exp.RenderMulticlusterPrivateMetadataJSON()
		fmt.Fprint(w, privateMetadataJSON)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}

}

func httpHandlerSpiffeBundleEndpoint(exp *Exporter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		spiffeBundleJSON, err := exp.SpiffeBundleJSON()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, err)
			return
		}
		fmt.Fprint(w, spiffeBundleJSON)
		logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
	}
}

//goland:noinspection SpellCheckingInspection
func httpHandlerHealthz(w http.ResponseWriter, r *http.Request) {
	clusterUUID := os.Getenv("CLUSTER_UUID")
	if clusterUUID == "" {
		clusterUUID = "unknown"
	}
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, fmt.Sprintf("ClusterUUID: %s", clusterUUID))
}

func main() {
	var exp, err = New("d8-istio", "ingressgateway")

	if err != nil {
		logger.Fatalf("Failed to create Exporter: %v", err)
	}

	var ctx, cancel = context.WithCancel(context.Background())

	var wg sync.WaitGroup // for wait all go routine

	listenAddr := "0.0.0.0:8080"
	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(httpHandlerHealthz))
	router.Handle("/metadata/public/spiffe-bundle-endpoint", httpHandlerSpiffeBundleEndpoint(exp))
	router.Handle("/metadata/public/public.json", httpHandlerPublicJSON(exp))

	if os.Getenv("FEDERATION_ENABLED") == "true" {
		router.Handle("/metadata/private/federation.json", httpHandlerFederationPrivateJSON(exp))
	}
	if os.Getenv("MULTICLUSTER_ENABLED") == "true" {
		router.Handle("/metadata/private/multicluster.json", httpHandlerMulticlusterPrivateJSON(exp))
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		exp.watchIngressGateways(ctx)
	}()

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      router,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	// create signal for graceful shutdown server

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Println("Could not listen on", listenAddr, ":", err)
			select {
			case stop <- syscall.SIGTERM:
			default:
			}
		}
	}()

	// Wait signal stop
	<-stop
	logger.Println("Shutting down server...")

	// Wait all goroutine stopped
	cancel()

	// Wait requests for server for gracefully shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatalf("Could not gracefully shutdown server: %v\n", err)
	}

	wg.Wait()
	logger.Println("Server gracefully stopped.")
}
