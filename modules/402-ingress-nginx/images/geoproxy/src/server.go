/*
Copyright 2025 Flant JSC

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

package geodownloader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/deckhouse/deckhouse/pkg/log"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start(ctx context.Context, server, metrics string) error {
	log.Info(fmt.Sprintf("Starting server at %s", server))

	mux := http.NewServeMux()
	promMux := http.NewServeMux()

	ok := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
	mux.HandleFunc("/healthz", ok)
	mux.HandleFunc("/readyz", ok)

	reg := prometheus.NewRegistry()
	reg.MustRegister(GeoIPErrors)
	promMux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	// pprof endpoints
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	fs := http.FileServer(http.Dir(PathDb))
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		log.Info(fmt.Sprintf("%s %s", r.Method, r.URL.Path))
		http.StripPrefix("/download/", fs).ServeHTTP(w, r)
	})

	var retErr error
	errCh := make(chan error, 2)

	srv := &http.Server{
		Addr:              server,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	srvProm := &http.Server{
		Addr:              metrics,
		Handler:           promMux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srvProm.ListenAndServe(); err != nil {
			log.Error(fmt.Sprintf("metrics server (%s): %v", metrics, err))
			errCh <- err
			return
		}
	}()

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Error(fmt.Sprintf("http server (%s): %v", server, err))
			errCh <- err
			return
		}
	}()

	// wait go routines
	select {
	case <-ctx.Done():
		retErr = ctx.Err()
	case err := <-errCh:
		retErr = err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srvProm.Shutdown(ctx)
	_ = srv.Shutdown(ctx)

	return retErr
}
