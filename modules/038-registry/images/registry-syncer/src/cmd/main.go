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
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/utils/ptr"
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
// A value rather than an empty string so that `--local-address=127.0.0.1:4281` stays a thing
// somebody can ask for on purpose, and only the untouched default is replaced. The loopback form
// of the store's own port, constant.StoragePort, which is what such a request would name.
const loopbackRegistry = "127.0.0.1:4281"

// pushIdleWindow is how long an untouched upload still counts as a push in progress.
//
// Short, because it only has to cover the gap between two chunks of one upload; and non-zero, because
// an abandoned upload keeps its directory for days until distribution purges it, and treating that as
// activity would mean a store that is never reclaimed again.
const pushIdleWindow = 2 * time.Minute

// writeEndpointAddress is the listener of this replica's registry that does NOT proxy, and the only
// address anything that modifies the store may be pointed at.
//
// The serving listener is a pull-through cache. Filling through it fills nothing — it answers "I
// already hold that blob" by fetching the blob from the upstream, so the layer is never uploaded —
// and collecting through it collects nothing, because the proxy store implements no deletion and
// answers every DELETE with 405. Both failures are quiet: a fill that reports what it wrote and a
// collection that reports what it kept, over a store that does not change.
func writeEndpointAddress(listenAddress string) string {
	return fmt.Sprintf("%s:%d", listenAddress, distribution.WriteEndpointPort)
}

// resolveLocalAddress decides where this replica's own registry is to be found: the address the
// replica SERVES on, not the loopback.
//
// The pod runs with host networking, so the loopback it shares is the node's, and the registry beside
// it binds only the node address. A syncer dialling its own port on the loopback reaches nothing.
// It used to be worse: the store and the registry agent shared a port number, so the loopback reached
// the agent, whose certificate comes from the agent's own authority and was rejected.
//
// The failure is invisible while an upstream is configured, because every node falls back to it and
// a leader that can neither read nor write its own store breaks nothing anybody notices. The air-gap
// transition is where it surfaces: there the count of what the store holds is the only evidence.
//
// An address given explicitly is left alone, the loopback included: a deployment that is not
// host-networked would rightly ask for it.
func resolveLocalAddress(localAddress, listenAddress string) string {
	if localAddress != loopbackRegistry || listenAddress == "" {
		return localAddress
	}
	return fmt.Sprintf("%s:%d", listenAddress, constant.StoragePort)
}

// leadership tracks whether this replica holds the lease.
//
// Only the leader is filled from the upstream, so that several replicas do not
// hammer it with the same credentials, and so that exactly one replica's
// completeness gates the air-gap transition.
//
// It also owns d8_registry_storage_leader, which D8RegistryStorageLeaderAbsent sums across the
// replicas. The gauge is set where the lease is won and lost, not by the loop: a pass reads the
// lease once at its start and may then run for an hour, filling or replicating, and a gauge set
// from that reading kept saying what the lease was an hour ago. A follower that took the lease
// from a dead leader in the middle of such a pass went on reporting 0 to the end of it — long
// enough for the alert to fire on a cluster that had a leader — and a leader that missed a
// renewal mid-fill went on reporting 1, which is the value that hides a cluster with none.
type leadership struct {
	// mu orders started against stopped; IsLeader reads without it.
	mu     sync.Mutex
	leader atomic.Bool
	// gauge is d8_registry_storage_leader; nil leaves the lease untracked by metrics.
	gauge prometheus.Gauge
}

func (l *leadership) IsLeader() bool { return l.leader.Load() }

// started records winning the lease, unless it has already been lost again, and reports whether it
// did.
//
// The elector calls OnStartedLeading in a goroutine of its own and OnStoppedLeading on its way
// out, after cancelling the context it handed the first one. A renewal that fails at once can
// therefore run the two in the opposite order, and a replica that took "started" at its word
// would believe it led until the next election. The cancelled context is what tells: checked
// under the same lock "stopped" takes, a "started" that arrives after "stopped" finds it done.
func (l *leadership) started(ctx context.Context) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if ctx.Err() != nil {
		return false
	}
	l.set(true)
	return true
}

// stopped records losing the lease, or never having won it, and reports whether there was a term
// to end.
func (l *leadership) stopped() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	held := l.leader.Load()
	l.set(false)
	return held
}

// callbacks is what the elector is handed: winning and losing the lease, recorded here.
//
// Built here rather than inline in serve so that the test drives the same functions the elector
// calls, and a callback that stops recording fails it.
//
// Each is said only when it took effect. client-go calls OnStoppedLeading in a defer of Run, for an
// attempt that never won as much as for a term that ended: on a stand an empty replica that stood
// for a quarter of a minute and stepped aside logged that it no longer filled the storage, having
// never filled it.
func (l *leadership) callbacks(log *slog.Logger, node string) leaderelection.LeaderCallbacks {
	return leaderelection.LeaderCallbacks{
		OnStartedLeading: func(leading context.Context) {
			if l.started(leading) {
				log.Info("this replica now fills the storage", "node", node)
			}
		},
		OnStoppedLeading: func() {
			if l.stopped() {
				log.Info("this replica no longer fills the storage; it keeps serving images", "node", node)
			}
		},
	}
}

func (l *leadership) set(held bool) {
	l.leader.Store(held)
	if l.gauge == nil {
		return
	}
	if held {
		l.gauge.Set(1)
		return
	}
	l.gauge.Set(0)
}

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
		configPath     string
		upstreamCAPath string
		listenAddress  string
		metricsAddress string
		localAddress   string
		nodeName       string
		namespace      string
		leaseName      string
		gcLeaseName    string
		registryBinary string
		processName    string
		interval       time.Duration
		copyRateMiB    int
		storeRecheck   time.Duration
		storeHostPath  string
		logLevel       string
	)

	flag.StringVar(&configPath, "config-path", "/config/config.yaml",
		"Where the registry configuration is written.")
	flag.StringVar(&upstreamCAPath, "upstream-ca-path", distribution.UpstreamCAFile,
		"Where the certificate authority of the upstream is written.")
	flag.StringVar(&gcLeaseName, "gc-lease-name", "registry-storage-gc",
		"The lease that serialises garbage collection, so at most one replica collects at a time.")
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
	flag.IntVar(&copyRateMiB, "copy-rate-mib", defaultCopyRateMiB,
		"How fast, in MiB/s, the copy of pending updates may read from its source; 0 for no limit. "+
			"Copying the set the cluster runs — the leader's fill and the followers' replication — and pulls "+
			"by the nodes are not limited.")
	flag.DurationVar(&storeRecheck, "store-recheck", 6*time.Hour,
		"How often an idle replica reads its own store again, to find what it may have lost.")
	flag.StringVar(&storeHostPath, "store-host-path", os.Getenv("REGISTRY_STORE_HOST_PATH"),
		"The store's directory on the node: which directory a disk for it is mounted at or above, and "+
			"what to change the owner of when the registry cannot write to it.")
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
		configPath:     configPath,
		upstreamCAPath: upstreamCAPath,
		listenAddress:  listenAddress,
		metricsAddress: metricsAddress,
		localAddress:   localAddress,
		nodeName:       nodeName,
		namespace:      namespace,
		leaseName:      leaseName,
		gcLeaseName:    gcLeaseName,
		registryBinary: registryBinary,
		processName:    processName,
		interval:       interval,
		copyRateMiB:    copyRateMiB,
		storeRecheck:   storeRecheck,
		storeHostPath:  storeHostPath,
	}); err != nil {
		log.Error("the syncer stopped", "error", err.Error())
		os.Exit(1)
	}
}

type options struct {
	configPath     string
	upstreamCAPath string
	listenAddress  string
	metricsAddress string
	localAddress   string
	nodeName       string
	namespace      string
	leaseName      string
	gcLeaseName    string
	registryBinary string
	processName    string
	interval       time.Duration
	copyRateMiB    int
	storeRecheck   time.Duration
	storeHostPath  string
}

// defaultCopyRateMiB bounds the background copy to what a control-plane node can spare while it
// runs etcd and the API server: measured at 0.02-0.05 of a core per MiB/s, so about half a core.
// A whole set of 11.8 GB took some 12 minutes at this rate (see Loop.copier), and a pending release
// copies only the layers the store does not hold yet.
//
// Background is the copy ahead, not the copy of the set the cluster runs — the leader's fill or a
// follower's replication: until a store holds that, a node pulls through the upstream, and a follower
// without it is a replica the cluster cannot fall back on.
const defaultCopyRateMiB = 16

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

	// The registry keeps a reserve free on the filesystem under the store, and how much depends on
	// whether that filesystem is the store's own. Found here, once: the container's mounts do not
	// change while it runs. The size the reserve is taken of is not — the registry measures it at
	// every write.
	storeDirectory := distribution.StoreDirectory
	if opts.storeHostPath != "" {
		storeDirectory = filepath.Base(opts.storeHostPath)
	}
	storeFilesystem := distribution.DetectFilesystem(distribution.MountInfo, distribution.DataDir, storeDirectory)
	log.Info("the filesystem under the store", "dedicated", storeFilesystem.Dedicated,
		"why", storeFilesystem.Why, "root", storeFilesystem.Root, "type", storeFilesystem.Type,
		"device", storeFilesystem.Device)

	// Credentials of this replica's own registry. Read from the environment rather
	// than from the custom resource: they are the pod's own identity, and the
	// storage has to stay reachable even when the desired state cannot be read.
	loop := &run.Loop{
		Log:       log,
		Client:    kubeClient,
		Namespace: opts.namespace,
		Node:      opts.nodeName,
		Applier: &distribution.Applier{
			ConfigPath:     opts.configPath,
			UpstreamCAPath: opts.upstreamCAPath,
			Restarter:      &distribution.SignalRestarter{ProcessName: opts.processName},
			Log:            log,
			Options: distribution.Options{
				ListenAddress: opts.listenAddress,
				HTTPSecret:    os.Getenv("REGISTRY_HTTP_SECRET"),
				// Derived from this replica's own address rather than read from a
				// secret. The token service runs beside it in the same pod, so no
				// other value could be right, and one supplied from outside could
				// only ever be wrong — as it was: a literal 127.0.0.1 would be the
				// client's own loopback, not the storage's.
				// This registry's own token path, not the token service's address. The service
				// listens on the loopback and no client can reach it; the registry forwards the
				// request instead. The challenge a client actually receives names the address that
				// client used — the registry builds it per request — so this value is only the
				// fallback, and a fallback pointing at an unreachable address is a lie waiting to
				// be believed.
				AuthRealm: fmt.Sprintf("https://%s:%d%s",
					opts.listenAddress, constant.StoragePort, distribution.AuthTokenPath),
				TokenIssuer:     os.Getenv("REGISTRY_TOKEN_ISSUER"),
				StoreFilesystem: storeFilesystem,
			},
		},
		Publisher: &report.Publisher{Client: kubeClient},
		// The loopback address for talking to itself, and the node address for the
		// neighbours: a replica replicating from this one comes in over the host port.
		LocalAddress: opts.localAddress,
		// And where to WRITE, which is another port of the same registry: the serving one is a
		// pull-through cache, and filling through a cache fills nothing — it answers "already have
		// that blob" by fetching it from the upstream, so the layers are never uploaded. See
		// Loop.writeRegistry.
		WriteAddress: writeEndpointAddress(opts.listenAddress),
		// The store the replica serves, counted from disk when completeness has to be read
		// rather than derived from what a fill wrote.
		DataDir:         distribution.DataDir,
		StoreHostPath:   opts.storeHostPath,
		ReportedAddress: fmt.Sprintf("%s:%d", opts.listenAddress, constant.StoragePort),
		LocalCA:         os.Getenv("REGISTRY_LOCAL_CA"),
		LocalUser:       os.Getenv("REGISTRY_LOCAL_USER"),
		LocalPassword:   os.Getenv("REGISTRY_LOCAL_PASSWORD"),
		Interval:        opts.interval,
		CopyPacer:       fill.NewPacer(opts.copyRateMiB << 20),
		StoreRecheck:    opts.storeRecheck,
		// What the write endpoint admits this syncer by: it refuses a handshake without a client
		// certificate. The registry's own serving pair, from the PKI mount the two containers
		// share — see distribution.CertificateFile.
		LocalClientCertificate: &fill.ClientCertificate{
			CertFile: distribution.CertificateFile,
			KeyFile:  distribution.KeyFile,
		},
	}

	// Built before the loop starts, which reads the lease through it — see followTheLease.
	var clientset kubernetes.Interface
	if opts.namespace != "" {
		if clientset, err = kubernetes.NewForConfig(restConfig); err != nil {
			return fmt.Errorf("building the clientset for the lease: %w", err)
		}
		followTheLease(loop, clientset, opts.namespace, opts.nodeName, opts.leaseName)
	}

	// Metrics about this replica's own work. Served on loopback: what the cluster scrapes
	// is the kube-rbac-proxy beside it, so this listener never needs to authenticate
	// anyone and never has to be reachable from off the node.
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	collected := metrics.New(registry)
	loop.Metrics = collected

	tracker := &leadership{gauge: collected.Leader}
	loop.Leadership = tracker

	go func() {
		if err := serveMetrics(ctx, opts.metricsAddress, registry); err != nil {
			// Never fatal. Filling a cache must not depend on whether anyone is watching.
			log.Error("the metrics listener stopped", "error", err.Error())
		}
	}()

	var candidacy *election
	if clientset != nil {
		candidacy = &election{
			log:        log,
			kubeClient: kubeClient,
			clientset:  clientset,
			namespace:  opts.namespace,
			leaseName:  opts.leaseName,
			self:       opts.nodeName,
			loop:       loop,
			tracker:    tracker,
			waiting:    &run.Waiting{For: passOverAfter, Log: log},
			seen:       &renewals{},
		}
		// Before the loop's first pass, which writes the registry's configuration: a leader recreated over
		// a lost store takes its report back before its registry serves anything, and as soon as this
		// process can — the controller reads the report without waiting for the pod.
		candidacy.stepDownAtStart(ctx)
	}

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
			// Long enough to renew through an API server that does not answer for half a minute.
			// A renewal that misses RenewDeadline releases the lease, and every full replica is then
			// equally entitled to it: on a three-master cluster an API stall of twenty-odd seconds,
			// minutes after bootstrap, moved the lease off a full leader to another full replica.
			//
			// The cost is the failover after a leader dies. About a minute in air-gap, where every
			// full replica already stands in the election and sees the renewals stop. With an upstream
			// the others stand only once the lease has expired, and a fresh elector waits a whole
			// LeaseDuration from its first look, so two to two and a half minutes — up to about three
			// and a half while nobody is full, see leaseGrace and nodeLostAfter, and two minutes more when
			// the replica holding the most does not take it, see passOverAfter. A leader stopped
			// gracefully releases the lease, and the handover takes one review period. The other
			// replicas keep what they hold, and one of them takes over without filling anew; but the
			// nodes pull from the leader alone, and reach the cache again only once their layout names
			// the new one — until then from the upstream, if there is one, and in air-gap not at all.
			LeaseDuration: leaseDuration,
			RenewDeadline: 40 * time.Second,
			RetryPeriod:   5 * time.Second,
			// The replica keeps serving images after losing the lease; it stops filling within
			// run.Loop.ArrangementEvery rather than at the end of the pass. A fill that ran on would
			// spend the upstream account the new leader fills with, and end by reporting this replica
			// as the leader, which takes the role from the one holding the lease. Releasing on cancel
			// hands the lease over promptly, which matters most when the reason for the cancel is that
			// another replica has the images and everything else is waiting on it.
			ReleaseOnCancel: true,
			Callbacks:       tracker.callbacks(log, opts.nodeName),
		})
		if err != nil {
			log.Error("cannot build the leader elector", "error", err.Error())
			return
		}
		elector.Run(attempt)
	}

	// Reclaiming the disk, when a RegistryOperation asks for it, on its own lease.
	//
	// A second lease, separate from the one above: the replica filling the cache is the last
	// one that should pause its writes, so "who fills" and "whose turn is it to collect" must be
	// free to be different replicas.
	go startCollector(ctx, log, opts, kubeClient, clientset, loop, collected)

	candidate := &run.Candidate{Log: log, Eligible: candidacy.eligible, Elect: elect}
	candidate.Run(ctx)
	return nil
}

// election is what this replica's candidacy reads and keeps between reviews.
type election struct {
	log        *slog.Logger
	kubeClient client.Client
	clientset  kubernetes.Interface
	namespace  string
	leaseName  string
	self       string
	loop       *run.Loop
	tracker    *leadership
	waiting    *run.Waiting
	seen       *renewals
}

// eligible says whether this replica should hold the lease at all, read from what every replica
// reports about itself, and why not when it should not. The leader is the replication
// source and the one whose completeness gates going air-gap, so an empty replica must
// not take the lease from a full one — see run.WhyNotLead. While nobody is full and nobody
// leads, a replica chosen to begin that does not take the lease is passed over — see
// run.Waiting.
//
// The lease is read as the loop reads it, with the renewals this replica saw happen — see renewals —
// on a tracker of its own, the loop's reads keeping their own pace. By the clocks alone a holder whose
// clock runs two minutes behind this one's holds nothing, and while nobody is full it is the choice:
// filling, and passed over for not taking a lease it holds.
//
// A replica the rule lets stand looks at its own store while another replica holds the whole set, and
// hands the lease over when the store does not hold it — see run.Loop.WhyStepAside. With nobody else
// full, the holder looks at it while its report says it holds the set, and found short takes the report
// back and stays eligible — see run.Loop.TakeBackFull.
func (e *election) eligible(ctx context.Context) (bool, string, error) {
	storage, currentSet, deployed, err := e.readStorage(ctx)
	if err != nil {
		return false, "", err
	}
	if storage == nil {
		// No storage object yet, so nothing is known to be full and somebody has
		// to lead in order to begin filling.
		return true, "", nil
	}
	now := time.Now()
	lease := readRenewedLease(ctx, e.clientset, e.namespace, e.leaseName, e.seen, now)
	lease.CanFill = storage.Spec.Upstream != nil && storage.Spec.Upstream.Host != ""
	replicas := presentReplicas(ctx, e.clientset, e.namespace, e.self, storage.Status.Replicas, now)
	// This replica's own report as it stands now, not as the status last heard it: a registry that
	// does not answer, or a disk that refuses writes, makes it not full at once — see WithOwnHealth;
	// a store it stepped down for, or took its report back for, not full and holding nothing — see
	// WithOwnLoss.
	replicas = run.WithOwnHealth(e.self, replicas, e.loop.HealthError())
	replicas = run.WithOwnLoss(e.self, replicas, e.loop.Lost())
	why := e.waiting.WhyNotLead(e.self, replicas, currentSet, lease, now)
	// A leader that took its report back and kept the lease owes a step-down once another replica holds
	// the whole set, whichever rule says so first: in air-gap it is the rule itself — no incumbent keeps a
	// leader that is not full beside one that is — and the step-down it owed would otherwise be left to
	// the elector's release and to its next pass. The rule's reason stays the one given.
	if why == "" || e.loop.TookBack() {
		if aside := e.stepAside(ctx, replicas, currentSet, deployed); why == "" {
			why = aside
		}
	}
	return why == "", why, nil
}

// readStorage reads the storage object, nil when there is none, and the fingerprint of the set the
// cluster needs now: a report of full measured for the previous one is not complete for this one.
// Unknown, the fingerprint is empty, and every report is taken at its word, as it always was. Beside
// it, the release deployed now, from the same reading of the releases — see run.CurrentSet.
func (e *election) readStorage(ctx context.Context) (*registryv1alpha1.RegistryStorage, string, string, error) {
	storage := &registryv1alpha1.RegistryStorage{}
	key := types.NamespacedName{Name: registryv1alpha1.SingletonName}
	if err := e.kubeClient.Get(ctx, key, storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, "", "", nil
		}
		return nil, "", "", err
	}
	currentSet, deployed, err := run.CurrentSet(ctx, e.kubeClient)
	if err != nil {
		currentSet, deployed = "", ""
	}
	return storage, currentSet, deployed, nil
}

// stepAside is run.Loop.WhyStepAside, with the lease given up where the elector does not: when this
// replica's name is on it and no term of this process is running. An elector that leads releases it as
// the attempt is cancelled — see ReleaseOnCancel.
func (e *election) stepAside(
	ctx context.Context, replicas []registryv1alpha1.StorageReplicaStatus, currentSet, deployed string,
) string {
	why, handOver := e.loop.WhyStepAside(ctx, replicas, currentSet, deployed, func() bool { return e.holdsLease(ctx) })
	if handOver && !e.tracker.IsLeader() {
		if err := releaseLease(ctx, e.clientset, e.namespace, e.leaseName, e.self); err != nil {
			e.log.Warn("cannot give up the lease this replica's name is on; it runs out in its own time",
				"error", err.Error())
		}
	}
	return why
}

// holdsLease says whether this replica holds the lease: a term of this process is running, or the lease
// carries its name — by the record, not the clocks, because the controller reads the record to tell whose
// report gates the air-gap transition, whether the holder still renews or not.
func (e *election) holdsLease(ctx context.Context) bool {
	if e.tracker.IsLeader() {
		return true
	}
	_, lease := readLeaseObject(ctx, e.clientset, e.namespace, e.leaseName, time.Now())
	return lease != nil && lease.Spec.HolderIdentity != nil && *lease.Spec.HolderIdentity == e.self
}

// stepDownAtStart looks at this replica's store once before the loop starts, when the lease carries its
// name — see run.Loop.WhyStepAside — so that a leader whose pod was recreated over a lost store takes its
// report back as early as this process can: beside a replica that holds the whole set it hands the lease
// over, and with nobody else full it keeps the lease and only takes the report back — see
// run.Loop.TakeBackFull.
//
// The elector would take the lease straight back — it names this node — and the status still carries
// the report of the previous process, full. The controller does not wait for this pod to turn Ready: it
// reads the record and that report on any reconcile — another replica's report, a Node, the lease, the
// requeue of a held air-gap. It does not drop the upstream on that report: it waits for one the running
// syncer made after reading its store (layout.LeaderReportCurrent). This hands the lease over within the
// pod's downtime and at most stepDownAtStartTimeout, so that the transition waits for a full leader
// rather than for an empty one to find out. Only the release's tag can be looked at — no pass has read
// the set — which is what a lost store loses first.
//
// With nobody else full the guard already holds the upstream on the previous process's report, and the
// take-back adds what the guard cannot: the status says the leader is not full before its registry
// serves, so needSync rises at once, rather than once the first recheck has read the store — after the
// registry has started, and storeRecheckRetry later if that reading fails, the upstream not answering
// through the pull-through. One tag link and the manifest it names, as beside a full replica.
//
// Best effort, and bounded: anything that cannot be read leaves the decision to the election.
func (e *election) stepDownAtStart(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, stepDownAtStartTimeout)
	defer cancel()

	if !e.holdsLease(ctx) {
		return
	}
	storage, currentSet, deployed, err := e.readStorage(ctx)
	if err != nil || storage == nil {
		return
	}
	replicas := presentReplicas(ctx, e.clientset, e.namespace, e.self, storage.Status.Replicas, time.Now())
	e.stepAside(ctx, replicas, currentSet, deployed)
}

// stepDownAtStartTimeout bounds the look at the store before the loop starts — see stepDownAtStart.
const stepDownAtStartTimeout = 10 * time.Second

// releaseLease gives up the lease when it carries this replica's name, as the elector does when it
// releases one: no holder, and a duration of a second, so that the next replica takes it at once.
func releaseLease(ctx context.Context, clientset kubernetes.Interface, namespace, name, self string) error {
	leases := clientset.CoordinationV1().Leases(namespace)
	lease, err := leases.Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != self {
		return nil
	}
	now := metav1.NewMicroTime(time.Now())
	lease.Spec.HolderIdentity = ptr.To("")
	lease.Spec.LeaseDurationSeconds = ptr.To[int32](1)
	lease.Spec.AcquireTime = &now
	lease.Spec.RenewTime = &now
	_, err = leases.Update(ctx, lease, metav1.UpdateOptions{})
	return err
}

// followTheLease makes the loop read who leads as the election does: from the lease, and, when it cannot
// be read, among the replicas whose pod and node are there. A follower then copies from the replica
// holding the lease, and from no replica that is gone — see run.LeaderOf.
//
// The holder counts as live also when this replica saw it renew within the lease's duration, by its own
// clock — see renewals.
func followTheLease(loop *run.Loop, clientset kubernetes.Interface, namespace, self, leaseName string) {
	loop.Present = func(
		ctx context.Context, replicas []registryv1alpha1.StorageReplicaStatus,
	) []registryv1alpha1.StorageReplicaStatus {
		return presentReplicas(ctx, clientset, namespace, self, replicas, time.Now())
	}
	seen := &renewals{}
	loop.ReadLease = func(ctx context.Context) run.Lease {
		return readRenewedLease(ctx, clientset, namespace, leaseName, seen, time.Now())
	}
}

// storagePodSelector picks the storage replicas' pods; see the StatefulSet.
const storagePodSelector = "app=registry-storage"

// presentReplicas drops the reports of replicas whose pod is gone, or whose node is. A report outlives
// its pod, and an election read from reports alone counted a removed replica as the fullest, or as full,
// or as the one leading — and waited for it. This replica's own report always stays. When the pods
// cannot be listed, or a node cannot be read, the reports they would decide stay, as before this
// existed.
func presentReplicas(
	ctx context.Context, clientset kubernetes.Interface, namespace, self string,
	replicas []registryv1alpha1.StorageReplicaStatus, now time.Time,
) []registryv1alpha1.StorageReplicaStatus {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: storagePodSelector})
	if err == nil {
		replicas = replicasOnNodes(self, replicas, pods.Items)
	}
	return replicasOnLiveNodes(self, replicas, func(name string) (*corev1.Node, error) {
		return clientset.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	}, now)
}

// replicasOnNodes keeps the reports of the nodes that run a storage pod which is not being deleted.
func replicasOnNodes(
	self string, replicas []registryv1alpha1.StorageReplicaStatus, pods []corev1.Pod,
) []registryv1alpha1.StorageReplicaStatus {
	running := map[string]bool{self: true}
	for i := range pods {
		pod := &pods[i]
		if pod.DeletionTimestamp == nil && pod.Spec.NodeName != "" &&
			(pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending) {
			running[pod.Spec.NodeName] = true
		}
	}

	present := make([]registryv1alpha1.StorageReplicaStatus, 0, len(replicas))
	for _, replica := range replicas {
		if running[replica.Node] {
			present = append(present, replica)
		}
	}
	return present
}

// nodeLostAfter is how long a node may go unheard from — its Ready condition Unknown — before the
// replica on it is no longer counted in the election.
//
// A pod does not leave a node that dies. The storage tolerates not-ready and unreachable for good, as
// it must to start before the network does, so nothing evicts it; the node lifecycle controller only
// marks it not ready, and its phase stays Running. Its report stood, and with it two dead ends. A
// leader that died while nobody was full stayed the fullest by its last report, and the survivors
// waited for it. A leader that died full kept `someone is full` true by its last report, the survivors
// being incomplete, and none of them would stand — with an upstream, a cluster that never filled again.
//
// Ready=Unknown rather than the pod's own readiness. Unknown is what the node lifecycle controller sets
// when the kubelet stops reporting — forty to fifty seconds after a node dies — and nothing else sets it.
// A pod is not ready for ordinary reasons as well: its registry walks the whole store before it opens a
// port, and after a power loss every pod still carries the not-ready its node was given at the time,
// so a replica would drop out of its neighbours' count exactly while it is starting. Ready=False is
// a kubelet that is alive and says so, and its replica stays.
//
// A minute past that, because a kubelet that misses a few reports — an API server busy for half a
// minute — is not a node that is gone. The lease and its grace run out over the same minutes, so the
// wait adds little to a failover.
const nodeLostAfter = time.Minute

// replicasOnLiveNodes drops the reports of replicas whose node is gone, or has been unheard from for
// longer than nodeLostAfter. Read by name, one node per report: there are as many as there are masters,
// and a list of every node in the cluster to answer for three of them would cost more than it saves.
//
// A node that cannot be read is taken as alive, as a list of pods that cannot be read is: the question
// is answered by the API server, and an unreachable one is no evidence about any node.
func replicasOnLiveNodes(
	self string, replicas []registryv1alpha1.StorageReplicaStatus,
	node func(name string) (*corev1.Node, error), now time.Time,
) []registryv1alpha1.StorageReplicaStatus {
	live := make([]registryv1alpha1.StorageReplicaStatus, 0, len(replicas))
	for _, replica := range replicas {
		if replica.Node == self || !nodeLost(node, replica.Node, now) {
			live = append(live, replica)
		}
	}
	return live
}

// nodeLost reports whether a node is gone, or its kubelet has not been heard from for nodeLostAfter.
func nodeLost(get func(name string) (*corev1.Node, error), name string, now time.Time) bool {
	node, err := get(name)
	switch {
	case apierrors.IsNotFound(err):
		return true
	case err != nil:
		return false
	}
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionUnknown &&
				now.Sub(condition.LastTransitionTime.Time) > nodeLostAfter
		}
	}
	return false
}

// leaseGrace is how long past its expiry a lease's holder still counts as the one leading. Long
// enough to ride out an API server that could not be reached for a minute, short enough that a
// leader whose node is gone is replaced in about the time the lease takes to expire twice.
const leaseGrace = time.Minute

// leaseDuration is how long the storage lease lasts past its holder's last renewal — see the elector.
const leaseDuration = 60 * time.Second

// passOverAfter is how long, while nobody is full and nobody leads, the others wait for the replica the
// rule chose to begin before they pass it over — see run.Waiting.
//
// Twice the lease's duration, because that is comfortably more than the chosen replica needs: up to one
// review (15 s) to see that nobody is full or leads any more, then LeaseDuration of client-go watching the
// old record unchanged before it takes it, then a retry jittered up to 11 s — 86 s at most after the
// moment it became the choice, against at least 120 s for the others, whose wait starts no sooner. In
// the ordinary case it leads before anyone gives up on it, and the others, seeing it lead, stop waiting.
//
// The moment is judged by each replica from other clocks — the lease's renewTime, a node's Ready
// lastTransitionTime — so replicas may see it at different times. With skew beyond the 34 s of margin a
// replica may stand while the chosen one is about to take the lease, and still lose on a held record: the
// chosen replica's elector observed that record first, and takes it first. Only a released lease can then
// go to the less full replica, which costs a fill, not correctness.
const passOverAfter = 2 * leaseDuration

// readLeaseObject reads who leads from the storage lease itself, by the clocks, and returns the lease it
// read: nil when there is none, or it cannot be read. Unknown when it cannot be read: the reported roles
// decide then, as they did before this existed.
func readLeaseObject(
	ctx context.Context, clientset kubernetes.Interface, namespace, name string, now time.Time,
) (run.Lease, *coordinationv1.Lease) {
	lease, err := clientset.CoordinationV1().Leases(namespace).Get(ctx, name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		// Never taken: nobody leads, which is a fact rather than an unknown.
		return run.Lease{Known: true}, nil
	case err != nil:
		return run.Lease{}, nil
	}
	return leaseAt(lease, now), lease
}

// readRenewedLease reads who leads from the storage lease, for the election and for a follower choosing
// whom to copy from: the holder is live also when this replica saw it renew within the lease's duration —
// see renewals.
//
// Unknown when this replica saw it renew only across a gap in its reads longer than the lease lasts, and
// the clocks do not show it live: the renewal may be from a second ago or from a minute and more before,
// and nothing here tells which. The roles then decide among the replicas whose node is there, as for a
// lease that cannot be read, and a copy already running goes on — see run.Loop.rearranged. Taken for
// not live, it cut the copy from a holder that never stopped renewing, after a minute without the API
// server, whenever the clocks disagree.
func readRenewedLease(
	ctx context.Context, clientset kubernetes.Interface, namespace, name string, seen *renewals, now time.Time,
) run.Lease {
	view, lease := readLeaseObject(ctx, clientset, namespace, name, now)
	if lease == nil {
		return view
	}
	switch live, unplaced := seen.renewedWithin(lease, now); {
	case live:
		view.Holder, view.Live = *lease.Spec.HolderIdentity, true
	case unplaced && !view.Live:
		return run.Lease{}
	}
	return view
}

// renewals is when this replica last saw the lease renewed, by its own clock — the way the elector
// judges a lease it does not hold.
//
// leaseAt compares the renewTime the holder wrote by its clock with the time here, with no allowance for
// the two clocks disagreeing. A follower whose clock ran a minute ahead of the leader's saw a lease
// renewed every five seconds as expired, and copied from nobody; in air-gap, where the leader is the
// only source of the set, it never got it. A renewTime that moved between two reads proves a renewal
// whatever the clocks say: one made after the first of the two reads. So a holder is live for the
// lease's duration after the read BEFORE the one that saw it move — the earliest moment the move
// proves, and so never later than the renewal itself.
//
// Not after the read that saw it: that is later than the renewal by up to the time between reads, and
// reads can be minutes apart. On a DVP stand (H2b) a follower cut from the API server for eight minutes
// read the dead leader's last renewal on its return, counted it live for a minute from then, and
// replicated from that leader 343 s after it stopped renewing. Reads come about a pass apart (30 s),
// every 15 s in the election, and every 10 s while a follower copies, so a holder that keeps renewing
// stays live while they succeed. After a gap in them longer than the lease lasts, a move proves nothing
// about the last minute, and the read says so: see readRenewedLease.
//
// The first read of a holder proves nothing: its renewTime may be an hour old. The clocks alone decide
// until it moves.
type renewals struct {
	mu      sync.Mutex
	holder  string
	renewed time.Time
	// read is when this replica last read the lease; zero before the first read.
	read time.Time
	// moved is when this replica read the lease last before it saw renewed change for this holder; zero
	// until it has.
	moved time.Time
}

// renewedWithin records what a lease says now, and reports whether this replica saw its holder renew it
// within its duration, and whether it saw a renewal it cannot place: one that moved renewTime since a
// read longer ago than the lease lasts.
func (r *renewals) renewedWithin(lease *coordinationv1.Lease, now time.Time) (bool, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	previous := r.read
	r.read = now

	spec := lease.Spec
	if spec.HolderIdentity == nil || *spec.HolderIdentity == "" || spec.RenewTime == nil || spec.LeaseDurationSeconds == nil {
		r.holder, r.renewed, r.moved = "", time.Time{}, time.Time{}
		return false, false
	}
	moved := false
	switch holder, renewed := *spec.HolderIdentity, spec.RenewTime.Time; {
	case holder != r.holder:
		r.holder, r.renewed, r.moved = holder, renewed, time.Time{}
	case !renewed.Equal(r.renewed):
		// previous is not zero: the read that set this holder came before.
		r.renewed, r.moved, moved = renewed, previous, true
	}
	live := !r.moved.IsZero() && !now.After(r.moved.Add(time.Duration(*spec.LeaseDurationSeconds)*time.Second))
	return live, moved && !live
}

// leaseAt is what a lease says at a moment: its holder while it is live and for leaseGrace after,
// nobody once it was released or that has passed.
func leaseAt(lease *coordinationv1.Lease, now time.Time) run.Lease {
	view := run.Lease{Known: true}

	spec := lease.Spec
	if spec.HolderIdentity == nil || *spec.HolderIdentity == "" || spec.RenewTime == nil || spec.LeaseDurationSeconds == nil {
		return view
	}
	expires := spec.RenewTime.Add(time.Duration(*spec.LeaseDurationSeconds) * time.Second)
	if now.After(expires.Add(leaseGrace)) {
		return view
	}

	view.Holder = *spec.HolderIdentity
	view.Live = !now.After(expires)
	return view
}

// startCollector runs the garbage collections asked of this replica's own store.
//
// Never fatal, and never in the way: a replica that cannot reclaim its disk still serves every
// image it holds, and the filling and reporting share this process.
func startCollector(
	ctx context.Context, log *slog.Logger, opts options,
	kubeClient client.Client, clientset kubernetes.Interface, loop *run.Loop,
	collected *metrics.Metrics,
) {
	if opts.namespace == "" {
		log.Warn("no namespace, so no garbage collection; the store will grow without bound")
		return
	}

	pushing := func() bool {
		active, err := fill.UploadInProgress(distribution.DataDir, pushIdleWindow, time.Now())
		if err != nil {
			// Unreadable means unknown, and unknown is not permission to delete: an upload that
			// cannot be seen is exactly the one that must not be swept from under.
			log.Warn("cannot tell whether a push is in progress, so the collection waits",
				"error", err.Error())
			return true
		}
		return active
	}
	// The same question for the estimate, which only reads and is asked without a collection to hold
	// back: unknown is "not now", and not worth a warning.
	pushingQuiet := func() bool {
		active, err := fill.UploadInProgress(distribution.DataDir, pushIdleWindow, time.Now())
		return err != nil || active
	}

	// What a collection would free now, for whoever decides whether to start one. Kept away from
	// the store while anything writes to it, a collection included — see gc.Estimator.
	var collecting atomic.Bool
	estimator := &gc.Estimator{
		Log:        log,
		Filling:    loop.Filling,
		Pushing:    pushingQuiet,
		Collecting: collecting.Load,
		Set:        loop.CurrentSet,
		Estimate: func(ctx context.Context) (int64, error) {
			collector, err := newCollector(ctx, log, opts, kubeClient, loop)
			if err != nil {
				return 0, err
			}
			collector.DryRun = true
			report, err := collector.Run(ctx)
			if err != nil {
				return 0, err
			}
			// What the collection frees: the blobs only the doomed reference, and those nothing
			// references at all, which it sweeps even with nothing to delete.
			return gc.Reclaimable(distribution.DataDir, report.Doomed)
		},
		Publish: collected.SetReclaimable,
	}

	// When to collect is the controller's decision, made into RegistryOperation objects: on the
	// schedule, after a new image set, or because somebody asked. This replica runs them.
	worker := &gc.Worker{
		Log: log,
		Plan: func() gc.Plan {
			spec := loop.Desired()
			if spec == nil {
				// Nothing has been read yet: the store has not been looked at.
				return gc.Plan{}
			}
			// A fill the cluster has asked for and this replica has not finished is a reason to
			// collect nothing: the store cannot be compared with anything while the set is still
			// going in. Only a copy this replica makes, though: see Loop.OwesCopy. And only one that
			// can go in: a store that refuses writes fails every fill at its first blob, and waiting
			// for it would wait out the one thing that can give the room back.
			return gc.Plan{
				Known:       true,
				FillPending: spec.NeedSync && !loop.Complete() && loop.OwesCopy() && !loop.StoreRefusesWrites(),
			}
		},
		// Filling has priority: a collection reclaims what nothing declares, and a fill is putting
		// the declarations in.
		Filling: loop.Filling,
		// The syncer stops writing to the cache for the duration, because it is the only thing in
		// this pod that writes to it. Nothing else has to change: the reclamation deletes what no
		// kept release declares, and a fill putting images in at the same moment is the one activity
		// that could contradict it. Taken only if it is free: a fill holds it for its whole pass.
		Pause: loop.TryPauseWrites,
		// And so has a push from outside, which is the one writer this process cannot take turns
		// with. Asked about activity rather than about the endpoint being open: it may stay open for
		// the life of the cluster while pushes happen twice a year.
		Pushing: pushing,
		Lease: &gc.ClusterLease{
			Log:       log,
			Client:    clientset.CoordinationV1(),
			Namespace: opts.namespace,
			Name:      opts.gcLeaseName,
			Identity:  opts.nodeName,
		},
		Operations: &gc.ClusterOperations{Client: kubeClient, Node: opts.nodeName},
		Collect: func(ctx context.Context) (gc.Report, error) {
			collecting.Store(true)
			defer func() {
				collecting.Store(false)
				estimator.Collected()
			}()

			// Runs with this replica's writes paused — see Pause above.

			// The store is measured on both sides of the run, inside the pause, so what it gave
			// back is not mixed with what a fill would have added. A cache miss served meanwhile can
			// still add a little, which makes the figure an underestimate and never an invention.
			before, measureErr := distribution.StoreBytes(distribution.DataDir)
			if measureErr != nil {
				log.Warn("cannot measure the store before the collection, so what it gives back is not reported",
					"error", measureErr.Error())
			}

			collector, err := newCollector(ctx, log, opts, kubeClient, loop)
			if err != nil {
				return gc.Report{}, err
			}
			// The registry is not touched at all, and that is deliberate: inside this pod the only
			// writer to the cache is this syncer, so "stop writing while collecting" is something it
			// does to itself. Serving goes on exactly as before — a collection is not an outage, and
			// making it one by restarting the serving process is what it used to be.
			collector.Sweep = &gc.Binary{
				Log:        log,
				Path:       opts.registryBinary,
				ConfigPath: opts.configPath,
			}
			// What an earlier sweep left behind, on a store that needs every byte of it back — see
			// gc.Collector.SweepAnyway.
			collector.SweepAnyway = loop.StoreRefusesWrites()
			collector.Pushing = pushing
			if collector.ReleaseAhead {
				log.Info("the store is short of room, so what is ahead of the set is collected too")
			}
			report, err := collector.Run(ctx)
			if collector.ReleaseAhead && len(report.Deleted) > 0 {
				loop.ForgetAhead()
			}
			if measureErr == nil {
				if after, afterErr := distribution.StoreBytes(distribution.DataDir); afterErr == nil {
					reclaimed := max(before-after, 0)
					report.ReclaimedBytes = &reclaimed
				} else {
					log.Warn("cannot measure the store after the collection, so what it gave back is not reported",
						"error", afterErr.Error())
				}
			}
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

	estimator.Plan = worker.Plan
	go estimator.Run(ctx)
	worker.Run(ctx)
}

// newCollector is a collection of this replica's store as it would run now: against the releases and
// modules the cluster has, and with the room the store has. Without a sweep, which only a collection
// that deletes has use for.
func newCollector(
	ctx context.Context, log *slog.Logger, opts options, kubeClient client.Client, loop *run.Loop,
) (*gc.Collector, error) {
	releases, err := gc.FromCluster(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	// The same modules the fill and the count enumerate: what the collector protects has to
	// be exactly what completeness is judged on, or one of them is wrong about the cluster.
	modules, err := gc.ModulesFromCluster(ctx, kubeClient)
	if err != nil {
		return nil, err
	}

	// The upstream, for the set of a release this store never got to hold — see
	// gc.Collector.Upstream. Unreadable credentials leave it out, and the store is judged by
	// what it holds, as before.
	upstream, err := loop.UpstreamRegistry()
	if err != nil {
		log.Warn("cannot read the upstream for the image set, so only this store is read", "error", err.Error())
		upstream = nil
	}

	return &gc.Collector{
		Log: log,
		Registry: gc.Registry{
			// The non-proxying listener, the same one a fill writes to: deleting through
			// the serving one deletes nothing. Its proxy store implements no deletion, so
			// every DELETE is answered 405 and the run is refused reference by reference
			// while reporting only warnings — see writeEndpointAddress. LocalOptions carry
			// the client certificate that listener requires.
			Address:  writeEndpointAddress(opts.listenAddress),
			Insecure: false,
			Scope:    strings.Trim(constant.Path, "/"),
			Options:  loop.LocalOptions(),
		},
		// What the store holds is read from its files, not asked of the registry: a
		// pull-through cache answers that question with its upstream's contents.
		DataDir:  distribution.DataDir,
		Releases: releases,
		Modules:  modules,
		Upstream: upstream,
		// What is ahead of the set is given back when the store is short of room and the upstream
		// holds it again — see gc.Collector.ReleaseAhead.
		ReleaseAhead: hasUpstream(loop.Desired()) && !loop.RoomAhead(),
	}, nil
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

// hasUpstream reports whether the storage is configured with an upstream to fetch from again.
func hasUpstream(spec *registryv1alpha1.RegistryStorageSpec) bool {
	return spec != nil && spec.Upstream != nil
}
