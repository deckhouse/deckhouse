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

package fencingstate

import (
	"fmt"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "fencing-agent/api/node-manager.deckhouse.io/v1alpha1"
	"fencing-agent/internal/domain"
)

// NewClient returns a cacheless typed client for FencingFailedNodeState objects.
// Writes and the read-modify-write of a status update use it: a cached read
// would carry a stale resourceVersion into every retry.
func NewClient(cfg *rest.Config) (client.Client, error) {
	scheme, err := newAPIScheme()
	if err != nil {
		return nil, err
	}

	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	return c, nil
}

// NewCache watches the incident objects of one NodeGroup. A healthy NodeGroup has
// none, so this costs a watch connection and an empty store; in exchange the
// agent can tell whether an object exists without asking the API, and still finds
// the ones it did not create itself.
func NewCache(cfg *rest.Config, nodeGroup string) (cache.Cache, error) {
	scheme, err := newAPIScheme()
	if err != nil {
		return nil, err
	}

	c, err := cache.New(cfg, cache.Options{
		Scheme: scheme,
		ByObject: map[client.Object]cache.ByObject{
			&v1alpha1.FencingFailedNodeState{}: {
				Label: labels.SelectorFromSet(labels.Set{domain.NodeGroupLabel: nodeGroup}),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create cache: %w", err)
	}

	return c, nil
}

func newAPIScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()

	if err := v1alpha1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("register %s scheme: %w", v1alpha1.GroupVersion, err)
	}

	return scheme, nil
}
