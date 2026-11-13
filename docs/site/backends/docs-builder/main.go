// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	v1 "github.com/flant/docs-builder/internal/http/v1"
	"github.com/flant/docs-builder/internal/local"
	"github.com/flant/docs-builder/internal/metrics"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/pprof"
	"github.com/flant/docs-builder/internal/registry"
	"github.com/flant/docs-builder/internal/s3cache"
)

// flags
var (
	listenAddress     string
	metricsAddress    string
	onDemandCache     string
	onDemandLogAddr   string
	onDemandS3        string
	onDemandS3Size    string
	onDemandCacheSize string
	onDemandS3Prefix  string
	// The public site: modules come from these registries instead of the
	// Module and ModuleSource objects of a cluster.
	onDemandRegistries    string
	onDemandRegistryAuths string
	// onDemandFollowEvery is how often a follower reads the targets the
	// leader recorded in S3.
	onDemandFollowEvery time.Duration
	// A developer's machine: modules come from the module repositories
	// under this directory, without a cluster or a registry.
	onDemandLocalDir     string
	onDemandLocalVersion string
)

// On-demand rendering of module versions. Measured on 2026-11-02: 70 modules
// had documentation, a rendered version took 1.34 MB on average (0.9 MB p50,
// 8.3 MB max, both languages), so 2 GiB holds about 1600 versions: 20
// versions of 80 modules, or 5 versions of 300 modules. Keep it below the
// sizeLimit of the cache volume in the documentation module, which also holds
// the renders in progress.
const (
	onDemandMaxSize       = 2 << 30 // bytes
	onDemandMaxIdle       = 7 * 24 * time.Hour
	onDemandMaxRenders    = 2
	onDemandMaxQueue      = 20
	onDemandRenderTimeout = 5 * time.Minute
	onDemandRetryAfter    = time.Minute
	// A version that does not exist or has no documentation is not looked up
	// again for an hour.
	onDemandNotFoundRetryAfter = time.Hour
	// The list of the versions of a module (tags/list) is cached this long.
	// A cluster lists rarely: deckhouse-controller scans its ModuleSources
	// every 30 minutes by default. The public site lists more often: new
	// versions should show up soon there. A version missing from the list
	// lists the registry earlier (ondemand.Versions.Has).
	onDemandVersionsTTL     = 30 * time.Minute
	onDemandSiteVersionsTTL = 5 * time.Minute
	onDemandEvictEvery      = time.Minute
	// The version of every module the search index and llms.txt are built
	// from is looked up this often. A version that does not exist, has no
	// documentation, fails the Hugo build or the AI export is tried again after
	// onDemandPinRetryAfter (or when the module moves to another version);
	// other failures on the next pass.
	onDemandPinEvery      = 15 * time.Minute
	onDemandPinRetryAfter = 24 * time.Hour
	// Archives are evicted from 80% of the bucket, or of its part under the
	// prefix (--ondemand-s3-size), down to 60%, once an hour. STS issues keys
	// for the SystemBucket "documentation".
	onDemandS3High       = 0.8
	onDemandS3Low        = 0.6
	onDemandS3EvictEvery = time.Hour
	// A pass renders anew the archives an older template rendered, one at a
	// time; the next pass picks up those a failure of the registry left.
	onDemandRerenderEvery = 10 * time.Minute
	// A pod waits for S3 this long at most before it takes traffic: a slow
	// S3 must not keep every replica unready during a rolling update.
	onDemandReadyWait   = 5 * time.Minute
	onDemandS3Namespace = "d8-system"
	onDemandS3Bucket    = "documentation"
	// The files of the local modules are checked for changes this often.
	onDemandLocalWatchEvery = time.Second
)

func init() {
	flag.StringVar(&listenAddress, "address", ":8081", "Address to listen on")
	flag.StringVar(&metricsAddress, "metrics-address", ":9090", "Address to listen on metrics")
	flag.StringVar(&onDemandCache, "ondemand-cache-dir", "", "Cache volume shared with nginx for module versions rendered on demand (required)")
	flag.StringVar(&onDemandLogAddr, "ondemand-access-log-address", "127.0.0.1:5514", "UDP address to receive the nginx syslog access log of on-demand pages on")
	flag.StringVar(&onDemandS3, "ondemand-s3-dir", "", "Directory with the connection of the S3 bucket that keeps rendered module versions (connection/: the ConfigMap of the SystemBucket or the Secret of the user) and the STS token (token/) when the connection has no static keys; empty keeps them in the cache volume only")
	flag.StringVar(&onDemandCacheSize, "ondemand-cache-size", "", "Size of the cache volume given in --ondemand-cache-dir (its sizeLimit), for example 4Gi; the pinned versions may take it less the room of the versions readers open, and a metric reports that budget")
	flag.StringVar(&onDemandS3Size, "ondemand-s3-size", "", "Size of the S3 bucket given in --ondemand-s3-dir, or of its part under --ondemand-s3-prefix, for example 5Gi; required with --ondemand-s3-dir")
	flag.StringVar(&onDemandS3Prefix, "ondemand-s3-prefix", "", "Prefix of every key in the S3 bucket, ending with \"/\"; objects outside of it are never read, written or deleted")
	flag.StringVar(&onDemandRegistries, "ondemand-registries", "", "Comma-separated module registries (<host>/<path>, whose tags are the modules) of the public site; when set, modules come from them instead of the Module and ModuleSource objects of the cluster, and the first registry that has a module wins")
	flag.StringVar(&onDemandRegistryAuths, "ondemand-registry-auths", "", "File with the docker config JSON of --ondemand-registries; empty means anonymous access")
	flag.DurationVar(&onDemandFollowEvery, "ondemand-follow-interval", time.Minute, "How often a replica that does not hold the leader Lease reads the versions the leader recorded in S3")
	flag.StringVar(&onDemandLocalDir, "ondemand-local-modules-dir", "", "Directory with module repositories (each with module.yaml, docs/, openapi/, crds/), or of directories of them, served as the modules of the public site without a cluster or a registry: a developer's machine. A module is rendered anew once its files change")
	flag.StringVar(&onDemandLocalVersion, "ondemand-local-version", "v0.0.0-local", "The version every module of --ondemand-local-modules-dir is served as")
}

func main() {
	flag.Parse()
	tuneGC()

	ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopNotify()

	logger := log.NewLogger(
		log.WithLevel(log.LogLevelFromStr(os.Getenv("LOG_LEVEL")).Level()),
		log.WithHandlerType(log.TextHandlerType),
	)

	mStorage := metricsstorage.NewMetricStorage()

	if err := metrics.RegisterMetrics(mStorage); err != nil {
		logger.Fatal("failed to register metrics", log.Err(err))
	}

	if err := checkFlags(); err != nil {
		logger.Fatal("flags", log.Err(err))
	}

	h := v1.NewHandler(logger.Named("v1"))

	// With S3 the pod is ready once it took from there the versions it serves
	// first, at most onDemandReadyWait after the start.
	var ready atomic.Bool
	markReady := sync.OnceFunc(func() { ready.Store(true) })
	parts, err := newOnDemand(onDemandCache, onDemandS3, onDemandS3Size, onDemandS3Prefix, onDemandLocalDir, logger.Named("ondemand"), mStorage)
	if err != nil {
		logger.Fatal("on-demand rendering", log.Err(err))
	}

	if parts.evictCold == nil {
		markReady()
	} else {
		time.AfterFunc(onDemandReadyWait, func() {
			if !ready.Load() {
				logger.Warn("ready before the versions came from S3: it answers too slowly")
				markReady()
			}
		})
	}
	parts.Ready = ready.Load
	h.EnableOnDemand(parts.OnDemand)
	store := parts.Store

	srv := &http.Server{
		Addr:    listenAddress,
		Handler: h,
	}

	metricsSrv := &http.Server{
		Addr:    metricsAddress,
		Handler: mStorage.Handler(),
	}

	// Only in a build with the pprof tag, see internal/pprof.
	pprofSrv := pprof.NewServer(logger)

	eg, ctx := errgroup.WithContext(ctx)

	logger.Info("starting application")

	eg.Go(srv.ListenAndServe)
	eg.Go(metricsSrv.ListenAndServe)
	if pprofSrv != nil {
		eg.Go(pprofSrv.ListenAndServe)
	}
	eg.Go(func() error {
		store.Run(ctx, onDemandEvictEvery)
		return nil
	})
	eg.Go(func() error {
		return ondemand.ListenAccessLog(ctx, onDemandLogAddr, store, logger.Named("ondemand"))
	})
	if parts.evictCold == nil {
		pinner := parts.newPinner()
		eg.Go(func() error {
			pinner.Run(ctx, onDemandPinEvery)
			return nil
		})
		if parts.watch != nil {
			eg.Go(func() error {
				parts.watch(ctx, pinner.Kick)
				return nil
			})
		}
	} else {
		runLeaderAndFollower(ctx, eg, parts, markReady, logger.Named("ondemand"))
	}

	logger.Info("application started")

	<-ctx.Done()

	logger.Info("stopping application")

	// Create a new context with timeout for graceful shutdown.
	// The signal context (ctx) may already be canceled (e.g., if one of the goroutines failed).
	// In that case, shutdown would immediately exit — which is not what we want.
	// A separate context allows all shutdown goroutines to complete correctly,
	// even if one of them fails — the error won't cancel the context for others.
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// New errgroup for shutdown — the main eg may have already exited with an error
	// (and consequently canceled its context). Reusing it for shutdown is not allowed.
	shutdownEg, shutdownCtx := errgroup.WithContext(timeoutCtx)

	shutdownEg.Go(func() error {
		return srv.Shutdown(shutdownCtx)
	})
	shutdownEg.Go(func() error {
		return metricsSrv.Shutdown(shutdownCtx)
	})
	if pprofSrv != nil {
		shutdownEg.Go(func() error {
			return pprofSrv.Shutdown(shutdownCtx)
		})
	}

	waitErr := shutdownEg.Wait()
	if waitErr != nil && !errors.Is(waitErr, context.Canceled) {
		logger.Error("goroutine error", log.Err(waitErr))
	}

	// The background work ends with ctx. Exiting before it does would leave
	// the leader Lease held until it expires, with no leader meanwhile, and
	// cut the uploads to S3 short.
	if err := waitStopped(timeoutCtx, eg); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		// A Hugo build takes no context: it may outlast the wait.
		logger.Warn("stop the background work", log.Err(err))
	}

	logger.Info("application stopped")
}

// checkFlags refuses the flags that would be ignored without a word.
func checkFlags() error {
	switch {
	case onDemandCache == "":
		return errors.New("--ondemand-cache-dir is required")
	case onDemandRegistries != "" && onDemandLocalDir != "":
		return errors.New("--ondemand-registries does not go with --ondemand-local-modules-dir")
	case onDemandS3 == "" && (onDemandS3Size != "" || onDemandS3Prefix != ""):
		return errors.New("--ondemand-s3-size and --ondemand-s3-prefix need --ondemand-s3-dir")
	}
	return nil
}

// waitStopped waits until ctx is done for the goroutines of eg, which end
// with its context, and returns the error of the group.
func waitStopped(ctx context.Context, eg *errgroup.Group) error {
	done := make(chan error, 1)
	go func() { done <- eg.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return errors.New("the background work is still running")
	}
}

// onDemandParts are what the on-demand rendering runs.
type onDemandParts struct {
	v1.OnDemand
	newPinner func() *ondemand.Pinner
	// newFallbackPinner reads the registry for a follower S3 does not answer.
	newFallbackPinner func() *ondemand.Pinner
	// evictCold is nil without S3: every replica then reads the registry
	// itself, there is no leader.
	evictCold func(context.Context)
	kube      kubernetes.Interface
	// watch, with local modules, kicks the pinner when their files change.
	watch func(ctx context.Context, kick func())
}

func newOnDemand(cacheDir, s3Dir, s3Size, s3Prefix, localDir string, logger *log.Logger, ms *metricsstorage.MetricStorage) (onDemandParts, error) {
	// Local modules need neither a cluster nor S3, whose leader Lease is
	// a cluster object.
	var localModules *local.Source
	var client dynamic.Interface
	var kube kubernetes.Interface
	if localDir != "" {
		if s3Dir != "" {
			return onDemandParts{}, errors.New("--ondemand-local-modules-dir does not go with --ondemand-s3-dir")
		}
		var err error
		if localModules, err = local.New(localDir, onDemandLocalVersion, logger); err != nil {
			return onDemandParts{}, fmt.Errorf("--ondemand-local-modules-dir: %w", err)
		}
	} else {
		config, err := rest.InClusterConfig()
		if err != nil {
			return onDemandParts{}, fmt.Errorf("in-cluster config: %w", err)
		}
		// A pass of the pinner reads the Module and the ModuleSources of every
		// module: about 300 requests, which take a minute at the default 5 per
		// second.
		config.QPS, config.Burst = 50, 100

		if client, err = dynamic.NewForConfig(config); err != nil {
			return onDemandParts{}, fmt.Errorf("dynamic client: %w", err)
		}
		if kube, err = kubernetes.NewForConfig(config); err != nil {
			return onDemandParts{}, fmt.Errorf("kubernetes client: %w", err)
		}
	}

	var bucket *s3cache.Cache
	var quota int64
	var templateRevision string
	if s3Dir != "" {
		var err error
		if bucket, quota, templateRevision, err = newColdBucket(s3Dir, s3Size, s3Prefix, docs.HugoInitDir, logger, ms); err != nil {
			return onDemandParts{}, err
		}
	}
	revision := func() string { return templateRevision }

	// The public site: modules from a list of registries, a page per release
	// channel, the search leading to the module root and links without the
	// language. One switch, so the parts never disagree. The local modules
	// of a developer's machine make up the public site too.
	site := onDemandRegistries != "" || localModules != nil

	clients := registry.NewClients()
	clients.Metrics = ms
	// The modules and how to render them: a cluster, the registries of the
	// public site, or the local modules, which need no registry.
	var sources interface {
		ondemand.ModuleSources
		Modules(ctx context.Context) ([]registry.Module, error)
	}
	// A ModulePullOverride exists in a cluster only; the public site has none.
	var isBranch func(context.Context, string, string) (bool, error)
	var clusterSources *registry.Sources
	switch {
	case localModules != nil:
	case site:
		var auths []byte
		if onDemandRegistryAuths != "" {
			var err error
			if auths, err = os.ReadFile(onDemandRegistryAuths); err != nil {
				return onDemandParts{}, fmt.Errorf("--ondemand-registry-auths: %w", err)
			}
		}
		static, err := registry.NewStaticSources(strings.Split(onDemandRegistries, ","), auths, clients, logger)
		if err != nil {
			return onDemandParts{}, fmt.Errorf("--ondemand-registries: %w", err)
		}
		sources = static
	default:
		clusterSources = registry.NewSources(client, logger)
		sources = clusterSources
		isBranch = func(ctx context.Context, module, version string) (bool, error) {
			tag, err := clusterSources.OverrideTag(ctx, module)
			return tag == version, err
		}
	}

	// The renders in progress take a few megabytes on top: not counted.
	var pinnedBudget int64
	if onDemandCacheSize != "" {
		size, err := resource.ParseQuantity(onDemandCacheSize)
		if err != nil || size.Value() <= onDemandMaxSize {
			return onDemandParts{}, fmt.Errorf("--ondemand-cache-size %q: a size above %d bytes is expected", onDemandCacheSize, int64(onDemandMaxSize))
		}
		pinnedBudget = size.Value() - onDemandMaxSize
	}

	versionsTTL := onDemandVersionsTTL
	if site {
		versionsTTL = onDemandSiteVersionsTTL
	}
	tags := ondemand.RegistryTags(sources, clients, logger)
	render := ondemand.RegistryRender(sources, clients, docs.HugoInitDir, logger, ms)
	if localModules != nil {
		tags, render = localModules.Tags, localModules.Render(docs.HugoInitDir)
	}
	versions := ondemand.NewVersions(tags, versionsTTL, onDemandRetryAfter, ms)
	versions.Logger = logger

	store := ondemand.NewStore(ondemand.Config{
		Dir:                cacheDir,
		MaxSize:            onDemandMaxSize,
		PinnedBudget:       pinnedBudget,
		MaxIdle:            onDemandMaxIdle,
		MaxRenders:         onDemandMaxRenders,
		MaxQueue:           onDemandMaxQueue,
		WaitForPin:         true,
		RenderTimeout:      onDemandRenderTimeout,
		RetryAfter:         onDemandRetryAfter,
		NotFoundRetryAfter: onDemandNotFoundRetryAfter,
		Render:             render,
		RenderList: func(ctx context.Context, data []byte, workDir string) (map[string][]byte, error) {
			return docs.RenderModulesList(ctx, docs.HugoInitDir, workDir, data, logger)
		},
		Relisted: versions.Forget,
		Logger:   logger,
		Metrics:  ms,
		Cold:     coldCache(bucket),
		// A bucket that is not set up yet has no archive either: the version
		// is rendered.
		ColdNotFound: func(err error) bool {
			return errors.Is(err, s3cache.ErrNotFound) || errors.Is(err, s3cache.ErrNotReady)
		},
		Revision:          revision,
		IndexAtModuleRoot: site,
		IsBranch:          isBranch,
	})

	if err := store.Load(); err != nil {
		return onDemandParts{}, fmt.Errorf("load cache: %w", err)
	}

	resolver := ondemand.Resolver{
		Latest: ondemand.LatestVersion(versions),
		Logger: logger,
	}
	switch {
	case localModules != nil:
		// One version per module, served as its stable channel.
		resolver.Modules, resolver.ChannelVersion = localModules.Modules, localModules.Version
	case site:
		// Every release channel has pages of its own on the public site.
		resolver.Modules, resolver.ChannelVersion = sources.Modules, ondemand.RegistryChannelVersion(sources, clients)
		resolver.Channels = ondemand.RegistryChannels(sources, clients)
	default:
		resolver.Modules, resolver.ChannelVersion = sources.Modules, ondemand.RegistryChannelVersion(sources, clients)
		// A cluster renders ahead the versions it is about to update to.
		resolver.Upcoming = clusterSources.PendingReleases
	}
	// A pass of any pinner of the pod stuck past its deadline fails the
	// liveness probe.
	passes := ondemand.NewPassWatch()
	newPinner := func() *ondemand.Pinner {
		p := ondemand.NewPinner(resolver, store, onDemandPinRetryAfter, logger)
		p.Watch = passes
		return p
	}
	newFallbackPinner := func() *ondemand.Pinner {
		p := ondemand.NewFallbackPinner(resolver, store, onDemandPinRetryAfter, logger)
		p.Watch = passes
		return p
	}

	var evictCold func(context.Context)
	if bucket != nil {
		// The first pass right at the start also tells whether the bucket is
		// ready, before any reader asks for a version.
		evictCold = func(ctx context.Context) {
			t := time.NewTicker(onDemandS3EvictEvery)
			defer t.Stop()
			for {
				err := store.EvictCold(ctx, quota, onDemandS3High, onDemandS3Low)
				if err != nil && !errors.Is(err, s3cache.ErrNotReady) {
					logger.Warn("evict versions from S3", log.Err(err))
				}
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
			}
		}
	}

	var embedded *ondemand.EmbeddedVersions
	if site && localModules == nil {
		// The history of the embedded modules, mounted from a ConfigMap.
		embedded = ondemand.NewEmbeddedVersions(filepath.Join(docs.HugoInitDir, "data", "embedded_modules_list.json"))
	}

	var watch func(ctx context.Context, kick func())
	if localModules != nil {
		watch = func(ctx context.Context, kick func()) { localModules.Watch(ctx, onDemandLocalWatchEvery, kick) }
	}

	return onDemandParts{OnDemand: v1.OnDemand{Store: store, Versions: versions, Site: site, Embedded: embedded, Live: passes.Check}, newPinner: newPinner, newFallbackPinner: newFallbackPinner, evictCold: evictCold, kube: kube, watch: watch}, nil
}

// newColdBucket returns the S3 bucket of the rendered versions, its quota in
// bytes and the revision of the site template in templateDir, which names
// the archives.
func newColdBucket(s3Dir, s3Size, s3Prefix, templateDir string, logger *log.Logger, ms *metricsstorage.MetricStorage) (*s3cache.Cache, int64, string, error) {
	size, err := resource.ParseQuantity(s3Size)
	if err != nil || size.Value() <= 0 {
		return nil, 0, "", fmt.Errorf("--ondemand-s3-size %q: the size of the bucket is expected", s3Size)
	}

	bucket, err := s3cache.New(s3Dir, onDemandS3Namespace, onDemandS3Bucket, s3Prefix, func(ready bool) {
		if ready {
			ms.GaugeSet(metrics.DocsBuilderOnDemandS3Ready, 1, nil)
			logger.Info("the S3 storage is ready")
			return
		}
		ms.GaugeSet(metrics.DocsBuilderOnDemandS3Ready, 0, nil)
		logger.Warn("the S3 storage is not ready: versions are rendered after every restart")
	})
	if err != nil {
		return nil, 0, "", fmt.Errorf("--ondemand-s3-prefix: %w", err)
	}

	// The template is under a megabyte and hashes in milliseconds, so it is
	// hashed at the start. Without a revision nothing is read from S3 or
	// written there, while the replicas still elect a leader and follow it:
	// a template that can not be read stops the start instead.
	revision, err := ondemand.TemplateRevision(templateDir)
	if err != nil {
		return nil, 0, "", fmt.Errorf("template revision: %w", err)
	}
	logger.Info("keeping rendered versions in the S3 storage", slog.String("template_revision", revision))

	return bucket, size.Value(), revision, nil
}

// runLeaderAndFollower runs the replicas that share S3: the one holding the
// Lease reads the registry, renders the pinned versions and the archives of
// older templates and evicts from S3; the others follow what it recorded
// there. A follower that becomes the leader starts from what it followed.
func runLeaderAndFollower(ctx context.Context, eg *errgroup.Group, parts onDemandParts, markReady func(), logger *log.Logger) {
	identity := os.Getenv("POD_NAME")
	if identity == "" {
		identity, _ = os.Hostname()
	}

	term := leaderTerm{
		setLeader: func(v float64) { parts.Store.SetGauge(metrics.DocsBuilderOnDemandLeader, v) },
		pin: func(ctx context.Context, onRestored func()) {
			pinner := parts.newPinner()
			pinner.OnRestored = onRestored
			pinner.Run(ctx, onDemandPinEvery)
		},
		evictCold: parts.evictCold,
		rerender:  func(ctx context.Context) { parts.Store.Rerender(ctx, onDemandRerenderEvery) },
		markReady: markReady,
	}
	// Every replica reports it, so the alert on no leader sees the replicas.
	term.setLeader(0)
	eg.Go(func() error {
		ondemand.RunLeaderElection(ctx, parts.kube, os.Getenv("POD_NAMESPACE"), identity, term.run, logger)
		return nil
	})
	eg.Go(func() error {
		// A bucket that is not set up yet is S3 not answering, unlike for
		// the store: the follower then reads the registry itself.
		missing := func(err error) bool { return errors.Is(err, s3cache.ErrNotFound) }
		ondemand.NewFollower(parts.Store, missing, onDemandPinRetryAfter, logger).Run(ctx, onDemandFollowEvery, term.leading.Load, markReady, func(fctx context.Context) {
			parts.newFallbackPinner().Run(fctx, onDemandPinEvery)
		})
		return nil
	})
}

// leaderTerm is what the replica holding the Lease does.
type leaderTerm struct {
	// leading tells the follower of this replica to stand by.
	leading atomic.Bool
	// setLeader sets the leader gauge.
	setLeader func(v float64)
	// pin renders the pinned versions; onRestored is called once it took
	// from S3 what is there.
	pin                 func(ctx context.Context, onRestored func())
	evictCold, rerender func(context.Context)
	markReady           func()
}

// run is one term: it returns once ctx, which ends with the Lease, is done
// and the work has stopped.
func (t *leaderTerm) run(ctx context.Context) {
	t.leading.Store(true)
	t.setLeader(1)
	defer func() {
		t.leading.Store(false)
		t.setLeader(0)
	}()

	var g errgroup.Group
	g.Go(func() error { t.pin(ctx, t.markReady); return nil })
	g.Go(func() error { t.evictCold(ctx); return nil })
	g.Go(func() error { t.rerender(ctx); return nil })
	_ = g.Wait()
}

// coldCache keeps a nil bucket a nil interface, so the store sees no cold
// cache at all.
func coldCache(bucket *s3cache.Cache) ondemand.ColdCache {
	if bucket == nil {
		return nil
	}
	return bucket
}
