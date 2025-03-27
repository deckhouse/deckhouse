/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package staticpod

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const (
	healthListenAddr = ":8097"
)

// runHealthServer starts a health server that provides readiness and liveness probes
func runHealthServer(ctx context.Context) error {
	okHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", okHandler)
	mux.HandleFunc("/readyz", okHandler)

	srv := &http.Server{
		Addr:    healthListenAddr,
		Handler: mux,
	}

	ctxListenStop := context.AfterFunc(ctx, func() {
		ctx, ctxDone := context.WithTimeout(context.Background(), 10*time.Second)
		defer ctxDone()

		srv.Shutdown(ctx)
	})

	defer ctxListenStop()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen and serve error: %w", err)
	}

	return nil
}
