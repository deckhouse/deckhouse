/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/time/rate"

	"user-api/pkg/api"
	"user-api/pkg/auth"
	"user-api/pkg/k8s"
)

type Config struct {
	ListenAddress string
	DexURL        string
	TLSCertFile   string
	TLSKeyFile    string
}

func main() {
	cfg := &Config{}

	rootCmd := &cobra.Command{
		Use:   "user-api",
		Short: "User API service for self-service operations",
		Long:  "User API service provides self-service operations like password reset for Deckhouse users",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cfg)
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfg.ListenAddress, "listen", ":8443", "Listen address and port")
	rootCmd.PersistentFlags().StringVar(&cfg.DexURL, "dex-url", "https://dex.d8-user-authn.svc", "Dex OIDC issuer URL")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSCertFile, "tls-cert-file", "", "TLS certificate file path")
	rootCmd.PersistentFlags().StringVar(&cfg.TLSKeyFile, "tls-key-file", "", "TLS private key file path")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg *Config) error {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting user-api service",
		"listen", cfg.ListenAddress,
		"dex_url", cfg.DexURL,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	oidcVerifier, err := auth.NewOIDCVerifier(ctx, cfg.DexURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC verifier: %w", err)
	}

	k8sClient, err := k8s.NewClient(logger)
	if err != nil {
		return fmt.Errorf("failed to create K8s client: %w", err)
	}

	// Start the password cache informer
	if err := k8sClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start K8s client: %w", err)
	}
	defer k8sClient.Stop()

	handler := api.NewHandler(oidcVerifier, k8sClient, logger)

	// Rate limiter: 10 requests per second with burst of 20
	resetLimiter := rate.NewLimiter(rate.Limit(10), 20)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handler.Healthz)
	mux.HandleFunc("GET /readyz", handler.Readyz)
	mux.HandleFunc("GET /metrics", handler.Metrics)
	mux.HandleFunc("POST /api/v1/password/reset", rateLimitMiddleware(resetLimiter, handler.ResetPassword))

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
			slog.Info("Starting HTTPS server")
			errCh <- server.ListenAndServeTLS(cfg.TLSCertFile, cfg.TLSKeyFile)
		} else {
			slog.Info("Starting HTTP server (no TLS)")
			errCh <- server.ListenAndServe()
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case sig := <-sigCh:
		slog.Info("Received signal, shutting down", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown error: %w", err)
		}
	}

	return nil
}

func rateLimitMiddleware(limiter *rate.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests, please try again later"}`))
			return
		}
		next(w, r)
	}
}
