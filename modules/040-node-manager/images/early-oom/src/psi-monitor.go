/*
Copyright 2021 Flant JSC

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
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/procfs"
)

const (
	pollInterval     = 5 * time.Second
	recoveryInterval = 15 * time.Second

	sysrqTriggerFile  = "/proc/sysrq-trigger"
	sysrqOOMCharacter = "f"
)

var psiUnavailable = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "early_oom_psi_unavailable",
	Help: "Whether PSI subsystem is unavailable on a given system",
})

func init() {
	prometheus.MustRegister(psiUnavailable)
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	if len(os.Args) < 2 {
		log.Fatal("failed to get memory threshold argument")
	}
	memoryThreshold := flag.Float64("memory-threshold", 0, "Memory threshold for PSI memory avg10")

	flag.Parse()

	if *memoryThreshold <= 0 {
		log.Fatalf("Please, provide positive value in memory-threshold flag: %v", *memoryThreshold)
	}

	server := &http.Server{Addr: "127.0.0.1:8080"}
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

	psiSupportErr := probePSISupport()
	if psiSupportErr != nil {
		log.Println(psiSupportErr)
		psiUnavailable.Set(1)

		sig := <-c
		shutdown(sig, server)
	}

	t := time.NewTimer(0)
	for {
		select {
		case sig := <-c:
			shutdown(sig, server)
		case <-t.C:
			if iteration(*memoryThreshold) {
				t.Reset(recoveryInterval)
			} else {
				t.Reset(pollInterval)
			}
		}
	}
}

func probePSISupport() error {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		return err
	}

	_, err = fs.PSIStatsForResource("memory")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("pressure information is unavailable, you need a Linux kernel >= 4.20 and/or CONFIG_PSI enabled for your kernel")
		}
		if errors.Is(err, syscall.ENOTSUP) {
			return errors.New("pressure information is disabled, add psi=1 kernel command line to enable it")
		}
	}

	return nil
}

func iteration(memoryThreshold float64) bool {
	fs, err := procfs.NewDefaultFS()
	if err != nil {
		log.Fatal(err)
	}

	stats, err := fs.PSIStatsForResource("memory")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Println("Pressure information is unavailable, you need a Linux kernel >= 4.20 and/or CONFIG_PSI enabled for your kernel")
		}
		if errors.Is(err, syscall.ENOTSUP) {
			log.Println("pressure information is disabled, add psi=1 kernel command line to enable it")
		}
	}

	if stats.Full.Avg10 > memoryThreshold {
		log.Printf("full avg10 value %f, threshold %f, triggering system OOM killer...", stats.Full.Avg10, memoryThreshold)
		err := triggerSystemOOM(sysrqTriggerFile)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Waiting for recovery for %s", recoveryInterval.String())

		return true
	}

	return false
}

func triggerSystemOOM(sysrqTriggerFile string) error {
	return os.WriteFile(sysrqTriggerFile, []byte(sysrqOOMCharacter), 0666)
}

func shutdown(sig os.Signal, server *http.Server) {
	log.Printf("Caught signal %s, exiting", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Failed to gracefully shutdown http server: %s", err)
	}

	os.Exit(0)
}
