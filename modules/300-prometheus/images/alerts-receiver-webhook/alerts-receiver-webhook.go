/*
Copyright 2023 Flant JSC

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
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/alertmanager/template"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ListenHost string
	ListenPort string
}

func NewConfig() *Config {
	c := &Config{}
	c.ListenHost = os.Getenv("LISTEN_HOST")
	if c.ListenHost == "" {
		c.ListenHost = "0.0.0.0"
	}

	c.ListenPort = os.Getenv("LISTEN_PORT")
	if c.ListenPort == "" {
		c.ListenPort = "8080"
	}
	return c
}

func main() {
	log.SetFormatter(&log.JSONFormatter{})

	cfg := NewConfig()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		log.Infof("got signal %v", sig)
		cancel()
	}()

	http.HandleFunc("/healthz", readyHandler)
	http.HandleFunc("/", webhookHandler)

	srv := &http.Server{
		Addr: net.JoinHostPort(cfg.ListenHost, cfg.ListenPort),
	}

	go func() {
		err := srv.ListenAndServe()
		cancel()
		if err == nil || err == http.ErrServerClosed {
			return
		}
		log.Error(err)
	}()

	<-ctx.Done()

	err := srv.Shutdown(context.Background())
	if err != nil && err != http.ErrServerClosed {
		log.Error(err)
	}
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	data := template.Data{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Error(err)
		return
	}
	for _, alert := range data.Alerts {
		log.Infof("received alert: %v", alert)
	}
	w.WriteHeader(http.StatusOK)
}
