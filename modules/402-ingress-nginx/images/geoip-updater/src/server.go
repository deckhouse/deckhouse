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

package main

import (
	"context"
	"log"
	"net/http"
)

type Server struct {
	server *http.Server
}

func NewHealthcheckServer(addr string) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", Health)

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &Server{
		server: server,
	}
}

func (s *Server) Start() error {
	log.Printf("Healthcheck server is listening on %s\n", s.server.Addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Beginning graceful shutdown...")
	return s.server.Shutdown(ctx)
}

func Health(w http.ResponseWriter, r *http.Request) {
	log.Printf("Health check requested from %s", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}
