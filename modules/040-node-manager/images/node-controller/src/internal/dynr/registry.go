/*
Copyright 2025 Flant JSC

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

package dynr

import (
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/node-controller/internal/rcname"
)

type reconcilerEntity struct {
	name        rcname.ReconcilerName
	obj         client.Object
	reconcilers []Reconciler
	isGroup     bool
}

var (
	registryMu sync.Mutex
	registry   []reconcilerEntity
)

func RegisterReconciler(name rcname.ReconcilerName, obj client.Object, r Reconciler) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry = append(registry, reconcilerEntity{name: name, obj: obj, reconcilers: []Reconciler{r}, isGroup: false})
}

func RegisterGroup(name rcname.ReconcilerName, obj client.Object, reconcilers ...Reconciler) {
	registryMu.Lock()
	defer registryMu.Unlock()

	registry = append(registry, reconcilerEntity{name: name, obj: obj, reconcilers: reconcilers, isGroup: true})
}
