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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"github.com/flant/docs-builder/internal/docs"
	"github.com/flant/docs-builder/internal/local"
	"github.com/flant/docs-builder/internal/ondemand"
	"github.com/flant/docs-builder/internal/ondemand/cache"
	"github.com/flant/docs-builder/internal/ondemand/versions"
	"github.com/flant/docs-builder/internal/registry"
	"github.com/flant/docs-builder/internal/registrydocs"
)

// modeParts are what a mode gives the on-demand rendering: where the modules
// come from, how their versions are listed and rendered, and how the target
// of each is picked. Every mode gives the same parts.
type modeParts struct {
	versions *versions.Versions
	render   ondemand.RenderFunc
	resolver cache.Resolver
	// isBranch tells the version a ModulePullOverride pulls: a cluster only,
	// the public site has none.
	isBranch func(ctx context.Context, module, version string) (bool, error)
	// site is the public site: a page per release channel, the search
	// leading to the module root and links without the language. One
	// switch, so the parts never disagree.
	site bool
	// embedded is the history of the embedded modules: the public site only.
	embedded *cache.EmbeddedVersions
	// watch, with local modules, kicks the pinner when their files change.
	watch func(ctx context.Context, kick func())
	// kube is the cluster the leader Lease is kept in; none with local
	// modules.
	kube kubernetes.Interface
}

// newMode makes the parts of the mode the flags pick: the local modules of
// a developer's machine, the registries of the public site or a cluster.
func newMode(cfg config, logger *log.Logger, ms *metricsstorage.MetricStorage) (modeParts, error) {
	switch {
	case cfg.localDir != "":
		return localMode(cfg, logger, ms)
	case cfg.registries != "":
		return siteMode(cfg, logger, ms)
	}

	return clusterMode(logger, ms)
}

// clusterMode reads the modules from the Module and ModuleSource objects of
// the cluster and their registries.
func clusterMode(logger *log.Logger, ms *metricsstorage.MetricStorage) (modeParts, error) {
	restConfig, err := inClusterConfig()
	if err != nil {
		return modeParts{}, err
	}
	client, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return modeParts{}, fmt.Errorf("dynamic client: %w", err)
	}
	kube, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return modeParts{}, fmt.Errorf("kubernetes client: %w", err)
	}

	clients := registryClients(ms)
	sources := registry.NewSources(client, logger)
	published := newVersions(registrydocs.Tags(sources, clients, logger), onDemandVersionsTTL, logger, ms)

	return modeParts{
		versions: published,
		render:   registrydocs.Render(sources, clients, docs.HugoInitDir, logger, ms),
		resolver: cache.Resolver{
			Modules: sources.Modules,
			Policy:  cache.ClusterPolicy{ChannelVersion: registrydocs.ChannelVersion(sources, clients), Latest: published.Latest, Logger: logger},
			// A cluster renders ahead the versions it is about to update to.
			Upcoming: sources.PendingReleases,
			Logger:   logger,
		},
		isBranch: func(ctx context.Context, module, version string) (bool, error) {
			tag, err := sources.OverrideTag(ctx, module)
			return tag == version, err
		},
		kube: kube,
	}, nil
}

// siteMode reads the modules of the public site from the registries of the
// flags: every release channel has pages of its own.
func siteMode(cfg config, logger *log.Logger, ms *metricsstorage.MetricStorage) (modeParts, error) {
	restConfig, err := inClusterConfig()
	if err != nil {
		return modeParts{}, err
	}
	kube, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return modeParts{}, fmt.Errorf("kubernetes client: %w", err)
	}

	var auths []byte
	if cfg.registryAuths != "" {
		if auths, err = os.ReadFile(cfg.registryAuths); err != nil {
			return modeParts{}, fmt.Errorf("--ondemand-registry-auths: %w", err)
		}
	}
	clients := registryClients(ms)
	sources, err := registry.NewStaticSources(strings.Split(cfg.registries, ","), auths, clients, logger)
	if err != nil {
		return modeParts{}, fmt.Errorf("--ondemand-registries: %w", err)
	}
	published := newVersions(registrydocs.Tags(sources, clients, logger), onDemandSiteVersionsTTL, logger, ms)

	return modeParts{
		versions: published,
		render:   registrydocs.Render(sources, clients, docs.HugoInitDir, logger, ms),
		resolver: cache.Resolver{
			Modules: sources.Modules,
			Policy:  cache.SitePolicy{Channels: registrydocs.Channels(sources, clients), Latest: published.Latest},
			Logger:  logger,
		},
		site: true,
		// The history of the embedded modules, mounted from a ConfigMap.
		embedded: cache.NewEmbeddedVersions(filepath.Join(docs.HugoInitDir, "data", "embedded_modules_list.json")),
		kube:     kube,
	}, nil
}

// localMode serves the module repositories under the directory of the flags
// as the modules of the public site, without a cluster or a registry: a
// developer's machine. Every module has one version, served as its stable
// channel.
func localMode(cfg config, logger *log.Logger, ms *metricsstorage.MetricStorage) (modeParts, error) {
	modules, err := local.New(cfg.localDir, cfg.localVersion, logger)
	if err != nil {
		return modeParts{}, fmt.Errorf("--ondemand-local-modules-dir: %w", err)
	}
	published := newVersions(modules.Tags, onDemandSiteVersionsTTL, logger, ms)

	return modeParts{
		versions: published,
		render:   modules.Render(docs.HugoInitDir),
		resolver: cache.Resolver{
			Modules: modules.Modules,
			// Each module is the branch of its files, as a ModulePullOverride
			// pulls one.
			Policy: cache.ClusterPolicy{ChannelVersion: modules.Version, Latest: published.Latest, Logger: logger},
			Logger: logger,
		},
		site:  true,
		watch: func(ctx context.Context, kick func()) { modules.Watch(ctx, onDemandLocalWatchEvery, kick) },
	}, nil
}

// inClusterConfig is the API server of the pod. A pass of the pinner reads
// the Module and the ModuleSources of every module: about 300 requests,
// which take a minute at the default 5 per second.
func inClusterConfig() (*rest.Config, error) {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	restConfig.QPS, restConfig.Burst = 50, 100

	return restConfig, nil
}

// registryClients are the clients of the module registries.
func registryClients(ms *metricsstorage.MetricStorage) *registry.Clients {
	clients := registry.NewClients()
	clients.Metrics = ms

	return clients
}

// newVersions lists the published versions of a module with tags, cached for
// ttl.
func newVersions(tags ondemand.TagLister, ttl time.Duration, logger *log.Logger, ms *metricsstorage.MetricStorage) *versions.Versions {
	published := versions.New(tags, ttl, onDemandRetryAfter, ms)
	published.Logger = logger

	return published
}
