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

package resources

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type resourceIsReadyCheckerFactory struct {
	watcher *Watcher
	kubeCl  *client.KubernetesClient
}

func newKubedogResourceIsReadyCheckerFactory(kubeCl *client.KubernetesClient) (*resourceIsReadyCheckerFactory, error) {
	return &resourceIsReadyCheckerFactory{
		watcher: NewWatcher(kubeCl.KubeClient),
	}, nil
}

func (f *resourceIsReadyCheckerFactory) getChecker(resource *template.Resource) (Checker, error) {
	resourceID := ResourceID{
		Name:             resource.Object.GetName(),
		Namespace:        resource.Object.GetNamespace(),
		GroupVersionKind: resource.GVK,
	}

	_, ok := resourceMap[resource.GVK]
	if !ok {
		log.DebugF("resourceIsReadyChecker: skip %s", resourceID.String())

		return nil, nil
	}

	checker := &resourceIsReadyChecker{}

	err := f.watcher.Watch(context.TODO(), resourceID, func(_ *unstructured.Unstructured, isReady bool) {
		checker.setIsReady(isReady)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to watch resource %s/%s: %w", resource.Object.GetNamespace(), resource.Object.GetName(), err)
	}

	return checker, nil
}

type resourceIsReadyChecker struct {
	mutex   sync.RWMutex
	isReady bool
}

func (k *resourceIsReadyChecker) IsReady() (bool, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	return k.isReady, nil
}

func (k *resourceIsReadyChecker) setIsReady(isReady bool) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	k.isReady = isReady
}

func (k *resourceIsReadyChecker) Name() string {
	return "resourceIsReadyChecker"
}
