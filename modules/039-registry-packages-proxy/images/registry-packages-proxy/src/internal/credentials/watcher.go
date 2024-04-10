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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	toolsWatch "k8s.io/client-go/tools/watch"

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

const (
	// defaultRepository is the repository address from deckhouse-registry secret.
	defaultRepository = ""
)

type Watcher struct {
	k8sClient                     *kubernetes.Clientset
	k8sDynamicClient              dynamic.Interface
	registrySecretDiscoveryPeriod time.Duration
	registryClientConfigsMutex    sync.RWMutex
	registryClientConfigs         map[string]*registry.ClientConfig
	logger                        *log.Entry
}

func NewWatcher(k8sClient *kubernetes.Clientset, k8sDynamicClient dynamic.Interface, registrySecretDiscoveryPeriod time.Duration, logger *log.Entry) *Watcher {
	return &Watcher{
		k8sClient:                     k8sClient,
		k8sDynamicClient:              k8sDynamicClient,
		registrySecretDiscoveryPeriod: registrySecretDiscoveryPeriod,
		registryClientConfigs:         make(map[string]*registry.ClientConfig),
		logger:                        logger,
	}
}

func (w *Watcher) Get(repository string) (*registry.ClientConfig, error) {
	w.registryClientConfigsMutex.RLock()
	defer w.registryClientConfigsMutex.RUnlock()

	clientConfig, ok := w.registryClientConfigs[repository]
	if !ok {
		return nil, fmt.Errorf("registry client config for repository '%s' not found", repository)
	}

	return clientConfig, nil
}

func (w *Watcher) Watch(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()

		w.watchSecret(ctx)
	}()

	go func() {
		defer wg.Done()

		w.watchModuleSources(ctx)
	}()

	wg.Wait()
}

func (w *Watcher) watchSecret(ctx context.Context) {
	err := w.fetchSecret(ctx)
	if err != nil {
		w.logger.Errorf("Fetch secret: %v", err)
		return
	}

	ticker := time.NewTicker(w.registrySecretDiscoveryPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := w.fetchSecret(ctx)
			if err != nil {
				w.logger.Errorf("Fetch secret: %v", err)
				return
			}
		}
	}
}

func (w *Watcher) fetchSecret(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Get the secret with the registry credentials
	secret, err := w.k8sClient.CoreV1().Secrets("d8-system").Get(ctx, "deckhouse-registry", metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get secret")
	}

	var input registrySecretData

	input.FromSecretData(secret.Data)

	registryConfig, err := input.toClientConfig()
	if err != nil {
		return errors.Wrap(err, "failed to convert secret data to registry config")
	}

	w.registryClientConfigsMutex.Lock()
	w.registryClientConfigs[defaultRepository] = registryConfig
	w.registryClientConfigsMutex.Unlock()

	return nil
}

func (w *Watcher) watchModuleSources(ctx context.Context) {
	watchFunc := func(_ metav1.ListOptions) (watch.Interface, error) {
		timeout := int64((30 * time.Second).Seconds())

		// Get the module sources and their registry credentials
		return w.k8sDynamicClient.Resource(moduleSourceGVR).Watch(ctx, metav1.ListOptions{TimeoutSeconds: &timeout})
	}

	moduleSourcesWatcher, err := toolsWatch.NewRetryWatcher("1", &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		w.logger.Errorf("Watch module sources: %v", err)
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
				w.logger.Errorf("Process module source event: %v", err)
			}
		}
	}
}

func (w *Watcher) processModuleSourceEvent(moduleSourceEvent watch.Event) error {
	switch moduleSourceEvent.Type {
	case watch.Added, watch.Modified:
		var moduleSource ModuleSource

		err := runtime.DefaultUnstructuredConverter.FromUnstructured(moduleSourceEvent.Object.(*unstructured.Unstructured).Object, &moduleSource)
		if err != nil {
			return errors.Wrap(err, "failed to convert unstructured object to module source")
		}

		var auth string

		if len(moduleSource.Spec.Registry.DockerCFG) > 0 {
			var err error
			auth, err = dockerConfigToAuth(moduleSource.Spec.Registry.DockerCFG, strings.Split(moduleSource.Spec.Registry.Repo, "/")[0])
			if err != nil {
				return errors.Wrap(err, "failed to convert docker config to auth")
			}
		}

		clientConfig := &registry.ClientConfig{
			Repository: moduleSource.Spec.Registry.Repo,
			Scheme:     moduleSource.Spec.Registry.Scheme,
			CA:         moduleSource.Spec.Registry.CA,
			Auth:       auth,
		}

		w.registryClientConfigsMutex.Lock()
		w.registryClientConfigs[moduleSource.Spec.Registry.Repo] = clientConfig
		w.registryClientConfigsMutex.Unlock()
	case watch.Deleted:
		var moduleSource ModuleSource

		err := runtime.DefaultUnstructuredConverter.FromUnstructured(moduleSourceEvent.Object.(*unstructured.Unstructured).Object, &moduleSource)
		if err != nil {
			return errors.Wrap(err, "failed to convert unstructured object to module source")
		}

		w.registryClientConfigsMutex.Lock()
		delete(w.registryClientConfigs, moduleSource.Spec.Registry.Repo)
		w.registryClientConfigsMutex.Unlock()
	}

	return nil
}
