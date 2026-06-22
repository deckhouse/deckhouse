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

// Binary agent is the registry-agent entrypoint. It wires together the proxy
// handler, TLS server, and controller-runtime manager that reconciles
// RegistryConfig custom resources.
package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"registry-agent/internal/auth"
	"registry-agent/internal/config"
	"registry-agent/internal/controller"
	"registry-agent/internal/proxy"
	"registry-agent/internal/server"
)

func main() {
	var (
		pkiDir            string
		usersFile         string
		registryDir       string
		listen            string
		agentURL          string
		healthListen      string
		bootstrapSeedFile string
	)

	flag.StringVar(&pkiDir, "pki-dir", "/etc/kubernetes/registry-agent/pki", "directory containing ca.crt, tls.crt and tls.key")
	flag.StringVar(&usersFile, "users-file", "/etc/kubernetes/registry-agent/users.yaml", "path to the users.yaml Basic-auth credentials file")
	flag.StringVar(&registryDir, "registry-dir", "/etc/containerd/registry.d", "containerd registry.d directory managed by the agent")
	flag.StringVar(&listen, "listen", ":5001", "address the proxy server listens on (host:port)")
	flag.StringVar(&agentURL, "agent-url", "https://127.0.0.1:5001", "public HTTPS URL of this agent (injected into containerd config)")
	flag.StringVar(&healthListen, "health-listen", "127.0.0.1:5051", "address the health/readiness server listens on (host:port)")
	flag.StringVar(&bootstrapSeedFile, "bootstrap-seed-file", "/etc/kubernetes/registry-agent-bootstrap/bootstrap-seed.yaml", "optional path to the bootstrap-seed YAML (host/scheme/ca); when present the agent appends the seed as a lowest-priority containerd mirror")
	flag.Parse()

	log := slog.Default().With("component", "main")

	// Read the module CA certificate from the PKI directory.
	caPath := filepath.Join(pkiDir, "ca.crt")
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		log.Error("failed to read CA certificate", "path", caPath, "error", err)
		os.Exit(1)
	}

	opts := config.Options{
		AgentURL: agentURL,
		ModuleCA: string(caBytes),
	}

	seed, err := config.LoadSeedMirror(bootstrapSeedFile)
	if err != nil {
		log.Error("failed to load bootstrap-seed file", "path", bootstrapSeedFile, "error", err)
		os.Exit(1)
	}
	opts.Seed = seed

	// Load the local Basic-auth users. Fail fast so misconfiguration is caught
	// at startup rather than silently serving all requests unauthenticated.
	users, err := auth.LoadUsers(usersFile)
	if err != nil {
		log.Error("failed to load users file", "path", usersFile, "error", err)
		os.Exit(1)
	}
	authenticator := auth.New(users)

	// RouterHolder starts empty; the controller fills it on first reconcile.
	holder := &controller.RouterHolder{}

	// Build a proxy handler that resolves the active Router per request from
	// the holder, so the controller can hot-swap routes without a restart.
	proxyHandler, err := proxy.NewHandlerFunc(func() *proxy.Router { return holder.Get() }, authenticator)
	if err != nil {
		log.Error("failed to build proxy handler", "error", err)
		os.Exit(1)
	}

	// Build the TLS server.
	certFile := filepath.Join(pkiDir, "tls.crt")
	keyFile := filepath.Join(pkiDir, "tls.key")
	srv := server.New(listen, certFile, keyFile, proxyHandler)

	// Build the controller-runtime manager.
	// Disable the default metrics server (binds :8080) — unsafe under hostNetwork.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: healthListen,
	})
	if err != nil {
		log.Error("failed to create controller manager", "error", err)
		os.Exit(1)
	}

	// Register the RegistryConfig reconciler.
	reconciler := &controller.Reconciler{
		Client:      mgr.GetClient(),
		RegistryDir: registryDir,
		Opts:        opts,
		Routers:     holder,
	}
	readyFlag := &atomic.Bool{}
	reconciler.Ready = readyFlag
	if err := reconciler.SetupWithManager(mgr); err != nil {
		log.Error("failed to set up reconciler", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("reconciled", controller.ReadyzCheck(readyFlag)); err != nil {
		log.Error("failed to add readyz check", "error", err)
		os.Exit(1)
	}

	// errgroup cancels the shared context when either goroutine returns an error,
	// so a proxy server bind/serve failure immediately stops the manager (and
	// vice-versa), causing the DaemonSet pod to restart.
	g, ctx := errgroup.WithContext(ctrl.SetupSignalHandler())

	g.Go(func() error {
		log.Info("starting proxy server", "listen", listen)
		return srv.Start(ctx)
	})

	g.Go(func() error {
		log.Info("starting controller manager")
		return mgr.Start(ctx)
	})

	if err := g.Wait(); err != nil {
		log.Error("registry-agent stopped with error", "error", err)
		os.Exit(1)
	}

	log.Info("registry-agent stopped")
}
