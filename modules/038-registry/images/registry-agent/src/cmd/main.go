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

// Command registry-agent lets a node pull images.
//
// It is what the container runtime asks about every registry, so it has to work when
// almost nothing else does: before the cluster network is up, while the API server is
// unreachable, and on a node whose root filesystem is read-only. Everything below is
// arranged around that.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-agent/internal/agent"
	"github.com/deckhouse/registry-agent/internal/containerd"
	"github.com/deckhouse/registry-agent/internal/layout"
	agentmetrics "github.com/deckhouse/registry-agent/internal/metrics"
	"github.com/deckhouse/registry-agent/internal/pki"
	"github.com/deckhouse/registry-agent/internal/proxy"
	"github.com/deckhouse/registry-agent/internal/status"
)

// DefaultKubeconfig is the kubelet's own credentials, which every node already has.
//
// Used rather than a service account token so that nothing has to be distributed to the
// node for the agent to read its layout: the kubelet's identity is in the
// `system:nodes` group, and the layout objects are readable by that group precisely
// because they carry no per-node secrets.
const DefaultKubeconfig = "/etc/kubernetes/kubelet.conf"

// buildScheme registers every kind this agent reads or writes.
//
// Core types included, because a layout names its credentials rather than carrying them and
// resolving one means reading a Secret. Left out, the read fails with "no kind is registered
// for the type v1.Secret" — and the agent treats a failed resolution as an unreachable API,
// so it quietly falls back to the layout it was installed with and reports success. The node
// keeps pulling, nothing looks broken, and the layout the cluster is actually trying to apply
// never takes effect.
//
// A function so it can be asserted, because the failure above is invisible from the outside.
// buildSource assembles where the agent gets its layout from.
//
// Lifted out of main so the wiring can be asserted. Every test of this package builds a Source
// with whatever it is testing, so none of them could notice that the real one was missing its
// resolver — and a missing resolver does not fail, it degrades: the layout from the API is
// refused, the one the node was installed with is used instead, and the agent reports success.
func buildSource(log *slog.Logger, kubeClient client.Client, opts options) *layout.Source {
	return &layout.Source{
		Log:       log,
		Client:    kubeClient,
		Node:      opts.nodeName,
		Cache:     &layout.Cache{Path: opts.cachePath},
		Bootstrap: &layout.Bootstrap{Path: opts.bootstrap},
		// The layouts name their credentials rather than carrying them, so reading one means
		// reading a Secret. Without this the agent can never apply what the cluster gives it.
		Resolver: &layout.Resolver{Namespace: moduleNamespace},
	}
}

// moduleNamespace is where the Secret holding the resolved credentials lives.
const moduleNamespace = "d8-system"

func buildScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := registryv1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("building the scheme: %w", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("adding the core types to the scheme: %w", err)
	}
	return scheme, nil
}

func main() {
	var (
		kubeconfig    string
		nodeName      string
		listen        string
		advertise     string
		dropInRoot    string
		pkiDir        string
		cachePath     string
		storePath     string
		bootstrap     string
		interval      time.Duration
		forwardLimit  time.Duration
		metricsListen string
		logLevel      string
	)

	flag.StringVar(&kubeconfig, "kubeconfig", DefaultKubeconfig,
		"Credentials for reading this node's layout. Defaults to the kubelet's own.")
	flag.StringVar(&nodeName, "node-name", os.Getenv("NODE_NAME"),
		"This node's name, which is also the name of its layout object.")
	flag.StringVar(&listen, "listen-address", constant.ProxyHost,
		"Where to serve the container runtime.")
	flag.StringVar(&advertise, "advertise-address", "",
		"How the runtime is told to reach the agent. Defaults to the listen address.")
	flag.StringVar(&dropInRoot, "containerd-registry-dir", containerd.DefaultRoot,
		"The registry drop-in directory of the container runtime.")
	flag.StringVar(&pkiDir, "pki-dir", pki.DefaultDir,
		"Where the agent's own certificate material was written at bootstrap.")
	flag.StringVar(&cachePath, "layout-cache", layout.DefaultCachePath,
		"Where to keep the copy of the layout used when the API server is unreachable.")
	flag.StringVar(&storePath, "store-path", constant.StorePath,
		"Where the in-cluster cache keeps its blobs on this node, measured while no cache is configured.")
	flag.StringVar(&bootstrap, "bootstrap-layout", layout.DefaultBootstrapPath,
		"Where the layout the node was installed with was written. Used only until the API server answers once.")
	flag.DurationVar(&interval, "interval", 30*time.Second, "How often the layout is re-read.")
	flag.DurationVar(&forwardLimit, "forward-timeout", 5*time.Minute,
		"How long one attempt at one registry may take.")
	flag.StringVar(&metricsListen, "metrics-address", "127.0.0.1:4286",
		"Where to serve the agent's own metrics. Loopback: nothing scrapes a static pod "+
			"that has to work while the API server does not.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn or error.")
	flag.Parse()

	log := newLogger(logLevel)

	if nodeName == "" {
		log.Error("no node name; set --node-name or NODE_NAME")
		os.Exit(1)
	}
	if advertise == "" {
		advertise = listen
	}

	if err := run(signalContext(), log, options{
		kubeconfig:    kubeconfig,
		nodeName:      nodeName,
		listen:        listen,
		advertise:     advertise,
		dropInRoot:    dropInRoot,
		pkiDir:        pkiDir,
		cachePath:     cachePath,
		storePath:     storePath,
		bootstrap:     bootstrap,
		interval:      interval,
		forwardLimit:  forwardLimit,
		metricsListen: metricsListen,
	}); err != nil {
		log.Error("the agent stopped", "error", err.Error())
		os.Exit(1)
	}
}

type options struct {
	kubeconfig    string
	nodeName      string
	listen        string
	advertise     string
	dropInRoot    string
	pkiDir        string
	cachePath     string
	storePath     string
	bootstrap     string
	interval      time.Duration
	forwardLimit  time.Duration
	metricsListen string
}

func run(ctx context.Context, log *slog.Logger, opts options) error {
	material := &pki.OnDisk{Dir: opts.pkiDir}
	if err := material.Ready(); err != nil {
		// Refused up front rather than at the first handshake: without this material the
		// runtime cannot verify the agent, and every pull on the node would fail for a
		// reason nobody would connect to a missing file.
		return fmt.Errorf("the agent certificate material is not usable: %w", err)
	}

	writer := &containerd.Writer{Root: opts.dropInRoot}
	if err := writer.Writable(); err != nil {
		// An immutable root filesystem is a supported target, and it is better to say so
		// than to fail on the first write with a bare permission error.
		return fmt.Errorf("the container runtime drop-in directory is not usable: %w", err)
	}

	// Deliberately not required. A node has no kubeconfig until it has completed its TLS
	// bootstrap, and the agent has to be answering the container runtime before that: the
	// images it serves include the ones the node needs to join at all. So this is
	// attempted, retried by the loop, and never a reason to refuse to start.
	connect := func() (client.Client, error) { return newClient(opts.kubeconfig) }

	kubeClient, err := connect()
	if err != nil {
		log.Warn("no API credentials yet; routing from what the node has on disk until there are",
			"error", err.Error())
	}

	source := buildSource(log, kubeClient, opts)
	if err := source.Validate(); err != nil {
		return fmt.Errorf("the layout source is unusable: %w", err)
	}

	loop := &agent.Loop{
		Log:           log,
		Source:        source,
		Writer:        writer,
		PKI:           material,
		Publisher:     &status.Publisher{Client: kubeClient, Node: opts.nodeName},
		Connect:       connect,
		ProxyEndpoint: opts.advertise,
		StorePath:     opts.storePath,
		Interval:      opts.interval,
	}

	// The agent's own view of the pulls passing through it. Read from the node rather
	// than scraped, for the reason the agent is a static pod at all: a kube-rbac-proxy
	// beside it would authenticate against the API server this component exists to
	// outlive.
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	collected := agentmetrics.New(registry)
	loop.Metrics = collected

	go func() {
		if err := serveMetrics(ctx, opts.metricsListen, registry); err != nil {
			// Never fatal. Every pull on this node goes through the agent, and none of
			// them may depend on whether anyone is watching.
			log.Error("the metrics listener stopped", "error", err.Error())
		}
	}()

	server := &proxy.Server{
		Log:            log,
		Layout:         proxy.LayoutFunc(loop.Current),
		Self:           opts.advertise,
		ForwardTimeout: opts.forwardLimit,
		Metrics:        collected,
	}
	loop.Serving = server.Serving
	loop.Usable = server.Usable

	// The loop runs first so that the runtime is not pointed at a proxy with no routing
	// rules yet.
	go func() {
		if err := loop.Run(ctx); err != nil {
			log.Error("the reconciliation loop stopped", "error", err.Error())
		}
	}()

	log.Info("serving the container runtime", "address", opts.listen, "advertised", opts.advertise)
	return server.Listen(ctx, opts.listen, material.TLSConfig())
}

// newClient builds a client from a kubeconfig, falling back to the in-cluster
// configuration.
//
// The fallback exists for the service account path: the kubelet's credentials are the
// default, but a node may be running the agent before the kubelet has any.
func newClient(kubeconfig string) (client.Client, error) {
	scheme, err := buildScheme()
	if err != nil {
		return nil, err
	}

	var config *rest.Config

	if _, statErr := os.Stat(kubeconfig); statErr == nil {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", kubeconfig, err)
		}
	} else {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("no usable credentials: %s is absent and there is no service account: %w",
				kubeconfig, err)
		}
	}

	// No caching client: the agent reads one object, and an informer would make its
	// correctness depend on a watch staying alive — which is exactly what it must
	// survive losing.
	built, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("building the API client: %w", err)
	}
	return built, nil
}

// serveMetrics answers whoever is on the node, and nobody else.
func serveMetrics(ctx context.Context, address string, registry *prometheus.Registry) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func newLogger(level string) *slog.Logger {
	var parsed slog.Level
	if err := parsed.UnmarshalText([]byte(level)); err != nil {
		parsed = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parsed}))
}

func signalContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		cancel()
		<-signals
		os.Exit(1)
	}()

	return ctx
}
