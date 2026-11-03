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

// ipv4-options-guard runs as a sidecar of the CNI agent and mitigates CVE-2026-98432
// in the host and in every pod network namespace. It never exits on its own: a failing
// sidecar would make the CNI agent pod NotReady, so problems are reported via metrics.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ipv4-options-guard/internal/guard"
)

func main() {
	var (
		settings     = flag.String("settings", "/etc/ipv4-options-guard/settings.json", "settings file from the CNI module ConfigMap")
		netnsDir     = flag.String("netns-dir", "/run/netns", "directory with pod netns bind mounts")
		hostNetNS    = flag.String("host-netns", "/proc/self/ns/net", "host netns path")
		hostSysctl   = flag.String("host-sysctl-file", "/host/etc/sysctl.d/90-d8-ipv4-options-guard.conf", "host sysctl.d file; empty to skip")
		sbin         = flag.String("sbin", "/sbin", "iptables-wrapper directory (shared with iptables-wrapper-init where the pod has it)")
		iptablesDist = flag.String("iptables-dist", "/iptables-dist", "iptables-wrapper and xtables binaries to install into -sbin if it is empty")
		resync       = flag.Duration("resync", time.Minute, "full resync interval")
		listen       = flag.String("listen", "127.0.0.1:9741", "metrics address")
	)
	flag.Parse()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	reg := prometheus.NewRegistry()
	met := guard.NewMetrics(reg)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	go func() {
		for {
			if err := http.ListenAndServe(*listen, mux); err != nil {
				log.Error("metrics server", "error", err)
			}
			time.Sleep(10 * time.Second)
		}
	}()

	cfg := guard.Config{
		SettingsFile:   *settings,
		NetNSDir:       *netnsDir,
		HostNetNSPath:  *hostNetNS,
		HostSysctlFile: *hostSysctl,
		Iptables:       guard.Iptables{Sbin: *sbin, Dist: *iptablesDist},
		Resync:         *resync,
	}
	if node := os.Getenv("NODE_NAME"); node != "" {
		if nl, err := guard.NewNodeLabels(node); err != nil {
			log.Warn("node opt-out label is not available", "error", err)
		} else {
			cfg.NodeDisabled = nl.Disabled
		}
	}

	if err := cfg.Iptables.Install(); err != nil {
		// Reported as an unprotected node: the iptables mode cannot be detected.
		log.Error("install iptables-wrapper", "error", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	guard.New(cfg, log, met).Run(ctx)
}
