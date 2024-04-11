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

	log "github.com/sirupsen/logrus"
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

	"github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy/registry"
)

type Watcher struct {
	k8sClient                     *kubernetes.Clientset
	k8sDynamicClient              dynamic.Interface
	registrySecretDiscoveryPeriod time.Duration
	sync.RWMutex
	registryClientConfigs map[string]*registry.ClientConfig
	logger                *log.Entry
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
	w.RLock()
	defer w.RUnlock()

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
	watchFunc := func(_ metav1.ListOptions) (watch.Interface, error) {
		timeout := int64((30 * time.Second).Seconds())

		// Get the module sources and their registry credentials
		return w.k8sClient.CoreV1().Secrets("d8-system").Watch(ctx, metav1.ListOptions{
			TimeoutSeconds: &timeout,
			FieldSelector:  fields.OneTermEqualSelector("metadata.name", "deckhouse-registry").String(),
		})
	}

	secretWatcher, err := toolsWatch.NewRetryWatcher("1", &cache.ListWatch{WatchFunc: watchFunc})
	if err != nil {
		w.logger.Errorf("Watch secrets: %v", err)
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
				w.logger.Errorf("Process secret event: %v", err)
			}
		}
	}
}

func (w *Watcher) processSecretEvent(secretEvent watch.Event) error {
	var secret v1.Secret

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(secretEvent.Object.(*unstructured.Unstructured).Object, &secret)
	if err != nil {
		return err
	}

	switch secretEvent.Type {
	case watch.Added, watch.Modified:

		var input registrySecretData

		input.FromSecretData(secret.Data)

		registryConfig, err := input.toClientConfig()
		if err != nil {
			return err
		}

		w.Lock()
		w.registryClientConfigs[registry.DefaultRepository] = registryConfig
		w.Unlock()

	case watch.Deleted:
		w.Lock()
		delete(w.registryClientConfigs, registry.DefaultRepository)
		w.Unlock()
	}

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
	var moduleSource ModuleSource

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(moduleSourceEvent.Object.(*unstructured.Unstructured).Object, &moduleSource)
	if err != nil {
		return err
	}

	switch moduleSourceEvent.Type {
	case watch.Added, watch.Modified:
		var auth string

		if len(moduleSource.Spec.Registry.DockerCFG) > 0 {
			var err error
			auth, err = dockerConfigToAuth(moduleSource.Spec.Registry.DockerCFG, strings.Split(moduleSource.Spec.Registry.Repo, "/")[0])
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
		w.registryClientConfigs[moduleSource.Spec.Registry.Repo] = clientConfig
		w.Unlock()
	case watch.Deleted:
		w.Lock()
		delete(w.registryClientConfigs, moduleSource.Spec.Registry.Repo)
		w.Unlock()
	}

	return nil
}
