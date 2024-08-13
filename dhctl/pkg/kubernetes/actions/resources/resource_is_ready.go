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
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

type resourceIsReadyChecker struct {
	mutex            sync.Mutex
	kubeCl           *client.KubernetesClient
	resourceID       ResourceID
	watcherIsRunning bool
	isReady          bool
}

func newResourceIsReadyChecker(kubeCl *client.KubernetesClient, resource *template.Resource) (Checker, error) {
	resourceID := ResourceID{
		Name:             resource.Object.GetName(),
		Namespace:        resource.Object.GetNamespace(),
		GroupVersionKind: resource.GVK,
	}

	checker := &resourceIsReadyChecker{
		kubeCl:     kubeCl,
		resourceID: resourceID,
	}

	return checker, nil
}

func (c *resourceIsReadyChecker) IsReady(ctx context.Context) (bool, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.watcherIsRunning {
		watcher := NewWatcher(c.kubeCl)

		err := watcher.Watch(ctx, c.resourceID, func(_ *unstructured.Unstructured, isReady bool) {
			c.setIsReady(isReady)
		})
		if err != nil {
			return false, fmt.Errorf("failed to watch resource '%s': %w", c.resourceID.String(), err)
		}

		c.watcherIsRunning = true
	}

	return c.isReady, nil
}

func (c *resourceIsReadyChecker) setIsReady(isReady bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.isReady = isReady
}

func (c *resourceIsReadyChecker) Name() string {
	return "resourceIsReadyChecker"
}
