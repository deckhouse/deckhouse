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

// Command registry-syncer keeps one storage replica in step with the desired
// state: it configures the process that serves images, fills the storage when it
// is the leader, and reports what it holds.
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
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-syncer/internal/distribution"
	"github.com/deckhouse/registry-syncer/internal/fill"
	"github.com/deckhouse/registry-syncer/internal/gc"
	"github.com/deckhouse/registry-syncer/internal/metrics"
	"github.com/deckhouse/registry-syncer/internal/report"
	"github.com/deckhouse/registry-syncer/internal/run"
)

// loopbackRegistry is the sentinel meaning "nobody asked for a particular address".
//
// A value rather than an empty string so that `--local-address=127.0.0.1:5001` stays a thing
// somebody can ask for on purpose, and only the untouched default is replaced.
const loopbackRegistry = "127.0.0.1:5001"

// resolveLocalAddress decides where this replica's own registry is to be found.
//
// It is the address the replica SERVES on, not the loopback, and the difference is not academic.
// The pod runs with host networking, so the loopback it shares is the node's — and on every node
// of a Managed cluster something else already answers there: the registry agent, on the very same
// port. A syncer dialling 127.0.0.1:5001 therefore reaches the agent rather than the registry
// beside it, is served a certificate from the agent's authority, and rejects it:
//
//	listing the repositories: Get "https://127.0.0.1:5001/v2/": tls: failed to verify
//	certificate: x509: certificate signed by unknown authority
//
// Measured on a cluster. It stayed invisible for a reason worth remembering: while an upstream is
// configured every node falls back to it, so a leader that can neither read nor write its own
// store breaks nothing anybody would notice. The air-gap transition is where it surfaces, because
// there the count of what the store holds is the only evidence there is.
//
// An address given explicitly is left alone, including the loopback itself: a deployment that is
// not host-networked would rightly ask for it.
// pushIdleWindow is how long an untouched upload still counts as a push in progress.
//
// Short, because it only has to cover the gap between two chunks of one upload; and non-zero, because
// an abandoned upload keeps its directory for days until distribution purges it, and treating that as
// activity would mean a store that is never reclaimed again.
const pushIdleWindow = 2 * time.Minute

func resolveLocalAddress(localAddress, listenAddress string) string {
	if localAddress != loopbackRegistry || listenAddress == "" {
		return localAddress
	}
	return fmt.Sprintf("%s:%d", listenAddress, constant.Port)
}

// leadership tracks whether this replica holds the lease.
//
// Only the leader is filled from the upstream, so that several replicas do not
// hammer it with the same credentials, and so that exactly one replica's
// completeness gates the air-gap transition.
type leadership struct {
	leader atomic.Bool
}

func (l *leadership) IsLeader() bool { return l.leader.Load() }

// buildScheme registers every kind this process reads or writes.
//
// Core types included, and that is not incidental: the storage spec names its credentials
// instead of carrying them, so a pass has to read a Secret. A scheme without core types does
// not fail at startup — it fails on the first read, with "no kind is registered for the type
// v1.Secret", which leaves this replica unable to render a configuration and the registry
// beside it crash-looping on a config file that never appears. Nothing about that error
// mentions a scheme.
//
// A function so it can be asserted. Tests build their own schemes and would never have caught
// this one being short.
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
		configPath      string
		writeConfigPath string
		upstreamCAPath  string
		listenAddress   string
		metricsAddress  string
		localAddress    string
		nodeName        string
		namespace       string
		leaseName       string
		gcLeaseName     string
		registryBinary  string
		processName     string
		interval        time.Duration
		logLevel        string
	)

	flag.StringVar(&configPath, "config-path", "/config/config.yaml",
		"Where the registry configuration is written.")
	// Empty by default, so a syncer running beside an older pod template — one without the second
	// container — configures nothing that has no reader.
	flag.StringVar(&writeConfigPath, "write-config-path", "",
		"Where the configuration of the write endpoint instance is written. Empty means there is none.")
	flag.StringVar(&upstreamCAPath, "upstream-ca-path", distribution.UpstreamCAFile,
		"Where the certificate authority of the upstream is written.")
	flag.StringVar(&gcLeaseName, "gc-lease-name", "registry-storage-gc",
		"The lease that serialises garbage collection, so at most one replica is read-only at a time.")
	flag.StringVar(&registryBinary, "registry-binary", "/registry",
		"The registry binary, used to reclaim blobs the deleted manifests left unreachable.")
	flag.StringVar(&metricsAddress, "metrics-address", "127.0.0.1:4284",
		"Where to serve metrics. Loopback, because a kube-rbac-proxy beside it is what the cluster scrapes.")
	flag.StringVar(&listenAddress, "listen-address", "",
		"The address the registry serves on. Defaults to the node address from the environment.")
	flag.StringVar(&localAddress, "local-address", loopbackRegistry,
		"Where this replica's own registry answers, used to read its catalogue and as the "+
			"destination of a fill. Defaults to the address this replica serves on, because the "+
			"pod is host-networked and the node's loopback belongs to the registry agent.")
	flag.StringVar(&nodeName, "node-name", os.Getenv("NODE_NAME"),
		"The node this replica runs on, and the key of its status entry.")
	flag.StringVar(&namespace, "namespace", os.Getenv("POD_NAMESPACE"),
		"The namespace the lease lives in.")
	flag.StringVar(&leaseName, "lease-name", "registry-storage",
		"The lease that elects the replica filled from the upstream.")
	flag.StringVar(&processName, "registry-process-name", "registry",
		"The executable of the serving process, signalled when the configuration changes.")
	flag.DurationVar(&interval, "interval", 30*time.Second,
		"How often the desired state is re-read.")
	flag.StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn or error.")
	flag.Parse()

	log := newLogger(logLevel)

	if nodeName == "" {
		log.Error("no node name; set --node-name or NODE_NAME")
		os.Exit(1)
	}
	if listenAddress == "" {
		listenAddress = os.Getenv("NODE_IP")
	}
	if listenAddress == "" {
		log.Error("no listen address; set --listen-address or NODE_IP")
		os.Exit(1)
	}

	localAddress = resolveLocalAddress(localAddress, listenAddress)

	ctx := signalContext()

	if err := serve(ctx, log, options{
		configPath:      configPath,
		writeConfigPath: writeConfigPath,
		upstreamCAPath:  upstreamCAPath,
		listenAddress:   listenAddress,
		metricsAddress:  metricsAddress,
		localAddress:    localAddress,
		nodeName:        nodeName,
		namespace:       namespace,
		leaseName:       leaseName,
		gcLeaseName:     gcLeaseName,
		registryBinary:  registryBinary,
		processName:     processName,
		interval:        interval,
	}); err != nil {
		log.Error("the syncer stopped", "error", err.Error())
		os.Exit(1)
	}
}

type options struct {
	configPath      string
	writeConfigPath string
	upstreamCAPath  string
	listenAddress   string
	metricsAddress  string
	localAddress    string
	nodeName        string
	namespace       string
	leaseName       string
	gcLeaseName     string
	registryBinary  string
	processName     string
	interval        time.Duration
}

func serve(ctx context.Context, log *slog.Logger, opts options) error {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("reading the cluster configuration: %w", err)
	}

	scheme, err := buildScheme()
	if err != nil {
		return err
	}

	kubeClient, err := client.New(restConfig, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("building the API client: %w", err)
	}

	// Credentials of this replica's own registry. Read from the environment rather
	// than from the custom resource: they are the pod's own identity, and the
	// storage has to stay reachable even when the desired state cannot be read.
	loop := &run.Loop{
		Log:       log,
		Client:    kubeClient,
		Namespace: opts.namespace,
		Node:      opts.nodeName,
		Applier: &distribution.Applier{
			ConfigPath:      opts.configPath,
			WriteConfigPath: opts.writeConfigPath,
			UpstreamCAPath:  opts.upstreamCAPath,
			Restarter:       &distribution.SignalRestarter{ProcessName: opts.processName},
			Options: distribution.Options{
				ListenAddress: opts.listenAddress,
				HTTPSecret:    os.Getenv("REGISTRY_HTTP_SECRET"),
				// Derived from this replica's own address rather than read from a
				// secret. The token service runs beside it in the same pod, so no
				// other value could be right, and one supplied from outside could
				// only ever be wrong — as it was: a literal 127.0.0.1 would be the
				// client's own loopback, not the storage's.
				AuthRealm:   fmt.Sprintf("https://%s:5051/auth", opts.listenAddress),
				TokenIssuer: os.Getenv("REGISTRY_TOKEN_ISSUER"),
			},
		},
		Publisher: &report.Publisher{Client: kubeClient},
		// The loopback address for talking to itself, and the node address for the
		// neighbours: a replica replicating from this one comes in over the host port.
		LocalAddress: opts.localAddress,
		// And where to WRITE, which is a different instance of the same registry over the same data
		// directory: the serving one is a pull-through cache, and filling through a cache fills
		// nothing — it answers "already have that blob" by fetching it from the upstream, so the
		// layers are never uploaded. See Loop.writeRegistry.
		WriteAddress: fmt.Sprintf("%s:%d", opts.listenAddress, distribution.WriteEndpointPort),
		// The store the replica serves, counted from disk when completeness has to be read
		// rather than derived from what a fill wrote.
		DataDir:         distribution.DataDir,
		ReportedAddress: fmt.Sprintf("%s:%d", opts.listenAddress, constant.Port),
		LocalCA:         os.Getenv("REGISTRY_LOCAL_CA"),
		LocalUser:       os.Getenv("REGISTRY_LOCAL_USER"),
		LocalPassword:   os.Getenv("REGISTRY_LOCAL_PASSWORD"),
		Interval:        opts.interval,
	}

	tracker := &leadership{}
	loop.Leadership = tracker

	// Metrics about this replica's own work. Served on loopback: what the cluster scrapes
	// is the kube-rbac-proxy beside it, so this listener never needs to authenticate
	// anyone and never has to be reachable from off the node.
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	collected := metrics.New(registry)
	loop.Metrics = collected

	go func() {
		if err := serveMetrics(ctx, opts.metricsAddress, registry); err != nil {
			// Never fatal. Filling a cache must not depend on whether anyone is watching.
			log.Error("the metrics listener stopped", "error", err.Error())
		}
	}()

	// The loop runs regardless of the election. Configuring the serving process is
	// every replica's job, and it must not wait for a lease: a replica that cannot
	// win an election still has to pick up a credential change.
	go func() {
		if err := loop.Run(ctx); err != nil {
			log.Error("the loop stopped", "error", err.Error())
		}
	}()

	if opts.namespace == "" {
		log.Warn("no namespace, so no leader election; this replica will never fill from the upstream")
		<-ctx.Done()
		return nil
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("building the clientset for the lease: %w", err)
	}

	// One elector per attempt: an elector cannot be reused once its context is done, and
	// this replica may leave and re-enter the election several times over its life.
	elect := func(attempt context.Context) {
		elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
			Lock: &resourcelock.LeaseLock{
				LeaseMeta: metav1.ObjectMeta{Namespace: opts.namespace, Name: opts.leaseName},
				Client:    clientset.CoordinationV1(),
				LockConfig: resourcelock.ResourceLockConfig{
					Identity: opts.nodeName,
				},
			},
			LeaseDuration: 30 * time.Second,
			RenewDeadline: 20 * time.Second,
			RetryPeriod:   5 * time.Second,
			// The replica keeps serving images after losing the lease; it only stops
			// filling. Releasing on cancel hands the lease over promptly, which matters
			// most when the reason for the cancel is that another replica has the images
			// and everything else is waiting on it.
			ReleaseOnCancel: true,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(context.Context) {
					log.Info("this replica now fills the storage", "node", opts.nodeName)
					tracker.leader.Store(true)
				},
				OnStoppedLeading: func() {
					log.Info("this replica no longer fills the storage; it keeps serving images",
						"node", opts.nodeName)
					tracker.leader.Store(false)
				},
			},
		})
		if err != nil {
			log.Error("cannot build the leader elector", "error", err.Error())
			return
		}
		elector.Run(attempt)
	}

	// Whether this replica should hold the lease at all, read from what every replica
	// reports about itself. The leader is the replication source and the one whose
	// completeness gates going air-gap, so an empty replica must not take the lease from
	// a full one — see run.MayLead.
	eligible := func(ctx context.Context) (bool, error) {
		storage := &registryv1alpha1.RegistryStorage{}
		key := types.NamespacedName{Name: registryv1alpha1.SingletonName}
		if err := kubeClient.Get(ctx, key, storage); err != nil {
			if apierrors.IsNotFound(err) {
				// No storage object yet, so nothing is known to be full and somebody has
				// to lead in order to begin filling.
				return true, nil
			}
			return false, err
		}
		return run.MayLead(opts.nodeName, storage.Status.Replicas), nil
	}

	// Reclaiming the disk, on its own schedule and its own lease.
	//
	// A second lease, separate from the one above: the replica filling the cache is the last
	// one that should stop accepting writes, so "who fills" and "whose turn is it to go
	// read-only" must be free to be different replicas.
	go startCollector(ctx, log, opts, kubeClient, clientset, loop, collected)

	candidate := &run.Candidate{Log: log, Eligible: eligible, Elect: elect}
	candidate.Run(ctx)
	return nil
}

// startCollector runs the garbage collection schedule for this replica's own store.
//
// Never fatal, and never in the way: a replica that cannot reclaim its disk still serves every
// image it holds, and the filling and reporting share this process.
func startCollector(
	ctx context.Context, log *slog.Logger, opts options,
	kubeClient client.Client, clientset *kubernetes.Clientset, loop *run.Loop,
	collected *metrics.Metrics,
) {
	if opts.namespace == "" {
		log.Warn("no namespace, so no garbage collection; the store will grow without bound")
		return
	}

	scheduler := &gc.Scheduler{
		Log: log,
		Plan: func() gc.Plan {
			// Read from the desired state at each firing, so changing the schedule does not
			// need a restart.
			spec := loop.Desired()
			if spec == nil {
				// Nothing has been read yet, which is not the same as a collection that has
				// been turned off — and saying "disabled" here used to cost a fresh replica
				// its first hour, because that is how long the disabled path waits.
				return gc.Plan{}
			}
			// A fill the cluster has asked for and this replica has not finished is a reason to
			// collect nothing: the store cannot be compared with anything while the set is still
			// going in, and the reclamation would refuse the writes that fill is made of.
			pending := spec.NeedSync && !loop.Complete()

			if spec.GarbageCollection == nil {
				// Read, and it says nothing about collecting.
				return gc.Plan{Known: true, FillPending: pending}
			}
			return gc.Plan{
				Known:       true,
				Enabled:     spec.GarbageCollection.Enabled,
				Schedule:    spec.GarbageCollection.Schedule,
				FillPending: pending,
			}
		},
		// Filling has priority: a collection reclaims what nothing declares, and a fill is putting
		// the declarations in.
		Filling: loop.Filling,
		// And so has a push from outside, which is the one writer this process cannot take turns
		// with. Asked about activity rather than about the endpoint being open: it may stay open for
		// the life of the cluster while pushes happen twice a year.
		Pushing: func() bool {
			active, err := fill.UploadInProgress(distribution.DataDir, pushIdleWindow, time.Now())
			if err != nil {
				// Unreadable means unknown, and unknown is not permission to delete: an upload that
				// cannot be seen is exactly the one that must not be swept from under.
				log.Warn("cannot tell whether a push is in progress, so the collection waits",
					"error", err.Error())
				return true
			}
			return active
		},
		Lease: &gc.ClusterLease{
			Log:       log,
			Client:    clientset.CoordinationV1(),
			Namespace: opts.namespace,
			Name:      opts.gcLeaseName,
			Identity:  opts.nodeName,
		},
		Collect: func(ctx context.Context) (gc.Report, error) {
			// The syncer stops writing to the cache for the duration, because it is the only thing
			// in this pod that writes to it. Nothing else has to change: the reclamation deletes
			// what no kept release declares, and a fill putting images in at the same moment is the
			// one activity that could contradict it.
			done := loop.PauseWrites()
			defer done()

			releases, err := gc.FromCluster(ctx, kubeClient)
			if err != nil {
				return gc.Report{}, err
			}

			// The same modules the fill and the count enumerate: what the collector protects has to
			// be exactly what completeness is judged on, or one of them is wrong about the cluster.
			modules, err := gc.ModulesFromCluster(ctx, kubeClient)
			if err != nil {
				return gc.Report{}, err
			}

			collector := &gc.Collector{
				Log: log,
				Registry: gc.Registry{
					Address:  opts.localAddress,
					Insecure: false,
					Scope:    strings.Trim(constant.Path, "/"),
					Options:  loop.LocalOptions(),
				},
				// What the store holds is read from its files, not asked of the registry: a
				// pull-through cache answers that question with its upstream's contents.
				DataDir:  distribution.DataDir,
				Releases: releases,
				Modules:  modules,
				// The registry is not touched at all, and that is deliberate: inside this pod the
				// only writer to the cache is this syncer, so "stop writing while collecting" is
				// something it does to itself. Serving goes on exactly as before — a collection is
				// not an outage, and making it one by restarting the serving process is what it used
				// to be.
				Sweep: &gc.Binary{
					Log:        log,
					Path:       opts.registryBinary,
					ConfigPath: opts.configPath,
				},
			}
			report, err := collector.Run(ctx)
			collected.Reclaimed.Add(float64(len(report.Deleted)))
			return report, err
		},
		Publish: func(ctx context.Context, finished time.Time, err error) {
			result := metrics.ResultSuccess
			if err != nil {
				result = metrics.ResultFailure
			}
			collected.Collections.WithLabelValues(result).Inc()
			if err == nil {
				// Only on success. A timestamp that moved on a failed run would report a
				// store as recently reclaimed when nothing was.
				collected.LastCollection.Set(float64(finished.Unix()))
			}

			if publishErr := loop.PublishCollection(ctx, finished, err); publishErr != nil {
				log.Warn("cannot record the outcome of the garbage collection",
					"error", publishErr.Error())
			}
		},
	}

	scheduler.Run(ctx)
}

// serveMetrics answers the scraper beside it, and nothing else.
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
		// A second signal means someone is in a hurry.
		os.Exit(1)
	}()

	return ctx
}
