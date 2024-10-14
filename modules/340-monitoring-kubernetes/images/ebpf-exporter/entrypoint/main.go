/*
Copyright 2024 Flant JSC

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
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	shutdownDurationSeconds = 5
)

var btfUnavailable = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "ebpf_exporter_btf_support_unavailable_in_kernel",
	Help: "BTF support is unavailable on a given system",
})

func init() {
	prometheus.MustRegister(btfUnavailable)
}

func main() {
	binPath := os.Getenv("EBPF_EXPORTER_BIN_PATH")
	if binPath == "" {
		binPath = "/usr/local/bin/ebpf_exporter"
	}

	configDir := os.Getenv("EBPF_EXPORTER_CONFIG_DIR")
	if configDir == "" {
		configDir = "/metrics"
	}

	configNames := os.Getenv("EBPF_EXPORTER_CONFIG_NAMES")
	if configNames == "" {
		configNames = "oomkill"
	}

	listenAddress := os.Getenv("EBPF_EXPORTER_LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = "127.0.0.1:9435"
	}

	args := []string{
		fmt.Sprintf("--config.dir=%s", configDir),
		fmt.Sprintf("--config.names=%s", configNames),
		fmt.Sprintf("--web.listen-address=%s", listenAddress),
	}

	cmd := exec.Command(binPath, args...)
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		server := runHTTPServer(listenAddress)
		errorHandling(err, server)
	}
}

func runHTTPServer(addr string) *http.Server {
	server := &http.Server{Addr: addr}
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()
	return server
}

func errorHandling(err error, server *http.Server) {
	log.Println(err)
	btfUnavailable.Set(1)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	sig := <-c
	shutdown(sig, server)
}

func shutdown(sig os.Signal, server *http.Server) {
	log.Printf("Caught signal %s, exiting", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), shutdownDurationSeconds*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Failed to gracefully shutdown http server: %s", err)
	}

	os.Exit(0)
}
