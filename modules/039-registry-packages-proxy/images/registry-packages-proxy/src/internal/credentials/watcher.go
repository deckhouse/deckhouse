// Copyright 2024 Flant JSC
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

package credentials

import (
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	toolsWatch "k8s.io/client-go/tools/watch"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type Watcher struct {
	k8sClient                     *kubernetes.Clientset
	k8sDynamicClient              dynamic.Interface
	packageRepositoryClient       ctrlclient.Client
	registrySecretDiscoveryPeriod time.Duration
	sync.RWMutex
	registryClientConfigs map[string]*registry.ClientConfig
	logger                *log.Logger

	// The cluster's own registry, held as an input rather than written straight into the map above,
	// so that the keys it owns can be withdrawn when it changes. See applyClusterRegistry.
	fromRegistrySecret *registry.ClientConfig

	// How to read the cluster's own STORE, from the secret the module renders only where a store runs.
	// Used while this node has no agent to fetch through — see storeAuthority for why that window
	// exists and why the installer's secret cannot serve in it.
	fromStoreSecret *registry.ClientConfig

	// clusterRegistryKeys are the keys the derivation currently owns, so that they can be withdrawn
	// when the authority changes. Without this, switching from the installer's registry to the store
	// would leave the previous repository behind as a key nothing serves.
	clusterRegistryKeys []string
}

func NewWatcher(
	k8sClient *kubernetes.Clientset,
	k8sDynamicClient dynamic.Interface,
	packageRepositoryClient ctrlclient.Client,
	registrySecretDiscoveryPeriod time.Duration,
	logger *log.Logger,
) *Watcher {
	return &Watcher{
		k8sClient:                     k8sClient,
		k8sDynamicClient:              k8sDynamicClient,
		packageRepositoryClient:       packageRepositoryClient,
		registrySecretDiscoveryPeriod: registrySecretDiscoveryPeriod,
		registryClientConfigs:         make(map[string]*registry.ClientConfig),
		logger:                        logger,
	}
}

// Get is the configuration for fetching from a repository — as dialled, not as recorded.
//
// The translation happens here, at the one place that has to dial, and nowhere else. Everything the
// cluster writes down names `registry.d8-system.svc:5001`, because a recorded address is read by more
// than one party and the agent's loopback belongs in none of them. But this process fetches, and on a
// node where the module manages the pull path the way to fetch is through the agent: it holds the
// credentials for whatever is behind it — the in-cluster store, an upstream — and asks the caller for
// none.
//
// This replaces reading the store's own access secret. That worked, and it was the wrong layer: it
// gave this client credentials for one particular backend, when the point of the agent is that a
// client on a node needs no credentials and no knowledge of which backend answers.
func (w *Watcher) Get(repository string) (*registry.ClientConfig, error) {
	w.RLock()
	defer w.RUnlock()

	clientConfig, ok := w.registryClientConfigs[repository]
	if !ok {
		return nil, fmt.Errorf("registry client config for repository '%s' not found", repository)
	}

	dialled := throughTheAgent(clientConfig, readAgentCA)
	if dialled != clientConfig {
		return dialled, nil
	}

	// No agent on this node yet — the window the agent's own installation happens in, since the agent is
	// installed from a package fetched through here. The cluster's own store is how that window is
	// crossed: its address, its authority, its read-only account, and nothing that leaves the cluster.
	//
	// Only for the repository the store serves. A ModuleSource brought its own address and its own
	// credentials, and the store's account authorizes nothing there.
	if w.fromStoreSecret != nil && servedByTheAgent(clientConfig.Repository) {
		return w.fromStoreSecret, nil
	}

	// And nothing after those two, which is the design rather than a gap.
	//
	// There was a third path here — a seed read out of `registry-bashible-config`, fetching from the
	// UPSTREAM with the credentials nodes are told. It existed because on a cache-less cluster there is
	// no store to read and the agent is what this very fetch installs, so a worker could not be given
	// the package that would have made it a node. That circle is now cut where it is actually closed:
	// the installer puts the agent on the first master over its own tunnel, and every node after that
	// fetches through the proxy beside that agent.
	//
	// Removing it also removes the one place credentials reached an upstream from here, which is the
	// rule this implementation is built on: an upstream is the agent's business, and a client on a node
	// needs no credentials at all.
	return clientConfig, nil
}

// repositoryKeys are the map keys one authority for the cluster's own registry installs.
//
// Three of them, matching what the proxy is asked for: the default repository, which is what a node
// bootstrapping asks for by name, the full repository, and the bare host.
func repositoryKeys(config *registry.ClientConfig) []string {
	return []string{
		registry.DefaultRepository,
		config.Repository,
		strings.Split(config.Repository, "/")[0],
	}
}

func (w *Watcher) Watch(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(4)

	go func() {
		defer wg.Done()

		w.watchSecret(ctx)
	}()

	go func() {
		defer wg.Done()

		w.watchStore(ctx)
	}()

	go func() {
		defer wg.Done()

		w.watchModuleSources(ctx)
	}()

	wg.Wait()
}

// watchStore follows how to read the cluster's own store, for the window in which this node has no
// agent to fetch through. See storeAuthority.
func (w *Watcher) watchStore(ctx context.Context) {
	watchFunc := func(_ metav1.ListOptions) (watch.Interface, error) {
		return w.k8sClient.CoreV1().Secrets("d8-system").Watch(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", storeAccessSecret).String(),
		})
	}

	storeWatcher, err := toolsWatch.NewRetryWatcherWithContext(ctx, "1", &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		w.logger.Error("Watch the store access secret: %v", err)
		return
	}
	defer storeWatcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-storeWatcher.Done():
			return
		case event, ok := <-storeWatcher.ResultChan():
			if !ok {
				return
			}
			w.processStoreEvent(event)
		}
	}
}

func (w *Watcher) processStoreEvent(secretEvent watch.Event) {
	secret, ok := secretEvent.Object.(*v1.Secret)
	if !ok {
		return
	}

	w.Lock()
	defer w.Unlock()

	switch secretEvent.Type {
	case watch.Added, watch.Modified:
		// Keyed to the repository the store serves, which is the one address every image reference in
		// the cluster is built from.
		// Joined with a slash: `storePath` carries none of its own, and concatenating them plainly
		// produced `registry.d8-system.svc:5001system/deckhouse`, which matches no repository and would
		// have failed as a missing key rather than as a bad address.
		authority := storeAuthority(secret.Data, storeHost+"/"+storePath)
		w.fromStoreSecret = authority
		if authority != nil {
			w.logger.Info("the in-cluster store can be read directly while a node has no agent",
				slog.String("repo", authority.Repository))
		} else {
			// Said out loud: a secret that exists but cannot be used is indistinguishable from an absent
			// one in behaviour, and only one of the two is worth investigating.
			w.logger.Info("the store access secret is present but incomplete, so it will not be used")
		}

	case watch.Deleted:
		w.fromStoreSecret = nil
	}
}

func (w *Watcher) watchSecret(ctx context.Context) {
	watchFunc := func(_ metav1.ListOptions) (watch.Interface, error) {
		// Get the deckhouse-registry secret
		return w.k8sClient.CoreV1().Secrets("d8-system").Watch(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", "deckhouse-registry").String(),
		})
	}

	secretWatcher, err := toolsWatch.NewRetryWatcherWithContext(ctx, "1", &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		w.logger.Error("Watch secrets: %v", err)
		return
	}
	defer secretWatcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-secretWatcher.Done():
			return
		case event, ok := <-secretWatcher.ResultChan():
			if !ok {
				return
			}

			err = w.processSecretEvent(event)
			if err != nil {
				w.logger.Error("Process secret event: %v", err)
			}
		}
	}
}

func (w *Watcher) processSecretEvent(secretEvent watch.Event) error {
	secret := secretEvent.Object.(*v1.Secret)

	var input registrySecretData
	input.FromSecretData(secret.Data)

	registryConfig, err := input.toClientConfig()
	if err != nil {
		return err
	}

	switch secretEvent.Type {
	case watch.Added, watch.Modified:
		w.Lock()
		defer w.Unlock()

		w.logger.Info(
			"the cluster registry secret names a repository",
			slog.String("repo", registryConfig.Repository),
		)
		w.fromRegistrySecret = registryConfig
		w.applyClusterRegistry()

	case watch.Deleted:
		w.Lock()
		defer w.Unlock()

		w.logger.Info(
			"the cluster registry secret is gone",
			slog.String("repo", registryConfig.Repository),
		)
		w.fromRegistrySecret = nil
		w.applyClusterRegistry()
	}

	return nil
}

// applyClusterRegistry installs the cluster's own registry under the keys it is asked for.
//
// Kept as a derivation rather than written straight into the map from the watch, because the keys it
// owns have to be withdrawn when the address changes: on a cluster that moves from one registry to
// another, a key left behind answers for a repository nobody pulls from any more — with credentials
// that may since have been revoked.
//
// What is stored is the registry AS RECORDED. Turning that into something dialable happens on the
// way out, in Get, for the same reason the Deckhouse controller does it at the point of dialling: the
// loopback address is node-local, and belongs in nothing that is written down.
//
// Callers hold the write lock.
func (w *Watcher) applyClusterRegistry() {
	for _, key := range w.clusterRegistryKeys {
		delete(w.registryClientConfigs, key)
	}
	w.clusterRegistryKeys = nil

	config := w.fromRegistrySecret
	if config == nil {
		return
	}

	w.clusterRegistryKeys = repositoryKeys(config)
	for _, key := range w.clusterRegistryKeys {
		w.registryClientConfigs[key] = config
	}
}

func (w *Watcher) watchModuleSources(ctx context.Context) {
	watchFunc := func(_ metav1.ListOptions) (watch.Interface, error) {
		// Get the module sources and their registry credentials
		return w.k8sDynamicClient.Resource(ModuleSourceGVR).Watch(ctx, metav1.ListOptions{})
	}

	moduleSourcesWatcher, err := toolsWatch.NewRetryWatcherWithContext(ctx, "1", &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		w.logger.Error("Watch module sources: %v", err)
		return
	}
	defer moduleSourcesWatcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-moduleSourcesWatcher.Done():
			return
		case event, ok := <-moduleSourcesWatcher.ResultChan():
			if !ok {
				return
			}

			err = w.processModuleSourceEvent(event)
			if err != nil {
				w.logger.Error("Process module source event: %v", err)
			}
		}
	}
}

func (w *Watcher) processModuleSourceEvent(moduleSourceEvent watch.Event) error {
	var moduleSource ModuleSource

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(moduleSourceEvent.Object.(*unstructured.Unstructured).Object, &moduleSource)
	if err != nil {
		return fmt.Errorf("unmarshal module source event: %v", err)
	}

	w.logger.Info("event from module source received", slog.String("event", string(moduleSourceEvent.Type)), slog.String("module_source", moduleSource.Name))

	switch moduleSourceEvent.Type {
	case watch.Added, watch.Modified:
		var auth string

		if len(moduleSource.Spec.Registry.DockerCFG) > 0 {
			dc, err := base64.StdEncoding.DecodeString(moduleSource.Spec.Registry.DockerCFG)
			if err != nil {
				return err
			}

			auth, err = dockerConfigToAuth(dc, strings.Split(moduleSource.Spec.Registry.Repo, "/")[0])
			if err != nil {
				return err
			}
		}

		clientConfig := &registry.ClientConfig{
			Repository: moduleSource.Spec.Registry.Repo,
			Scheme:     moduleSource.Spec.Registry.Scheme,
			CA:         moduleSource.Spec.Registry.CA,
			Auth:       auth,
		}

		w.Lock()
		w.logger.Info("added registry config for repo", slog.String("repo", moduleSource.Spec.Registry.Repo))
		w.registryClientConfigs[moduleSource.Spec.Registry.Repo] = clientConfig
		w.Unlock()
	case watch.Deleted:
		w.Lock()
		w.logger.Info("deleted registry config for repo", slog.String("repo", moduleSource.Spec.Registry.Repo))
		delete(w.registryClientConfigs, moduleSource.Spec.Registry.Repo)
		w.Unlock()
	}

	return nil
}
