// Copyright 2026 Flant JSC
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

// Package grants resolves per-project cluster resource grants
// (multitenancy-manager AvailableClusterResource) so that Application package
// settings fields tagged with x-deckhouse-grantable-resource can be defaulted
// and validated against what a project may use.
package grants

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	availableGroup   = "multitenancy.deckhouse.io"
	availableVersion = "v1alpha1"
	availableKind    = "AvailableClusterResource"
)

// Catalog is the resolved per-project catalog for one grantable resource.
type Catalog struct {
	// Default is the per-project default granted name. May be empty.
	Default string
	// Available is the set of granted names available to the project.
	Available []string
	// Found reports whether an AvailableClusterResource was actually located.
	// When false, the feature is treated as inactive (no defaulting, no
	// validation) for this resource.
	Found bool
}

// IsAvailable reports whether name is in the available set.
func (c Catalog) IsAvailable(name string) bool {
	for _, n := range c.Available {
		if n == name {
			return true
		}
	}

	return false
}

// Resolver resolves a grantable resource catalog for a namespace.
type Resolver interface {
	// Resolve returns the per-project catalog for the AvailableClusterResource
	// named resource in namespace. A missing CRD (multitenancy disabled) or a
	// missing object yields Catalog{Found: false} with a nil error; only
	// unexpected API errors are returned.
	Resolve(ctx context.Context, namespace, resource string) (Catalog, error)
}

// clientResolver reads AvailableClusterResource objects via a controller-runtime
// client using unstructured access, so the multitenancy types do not need to be
// compiled into the deckhouse-controller scheme.
type clientResolver struct {
	client kclient.Client
}

// NewResolver returns a Resolver backed by the given client.
func NewResolver(client kclient.Client) Resolver {
	return &clientResolver{client: client}
}

func (r *clientResolver) Resolve(ctx context.Context, namespace, resource string) (Catalog, error) {
	obj := new(unstructured.Unstructured)
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   availableGroup,
		Version: availableVersion,
		Kind:    availableKind,
	})

	key := kclient.ObjectKey{Namespace: namespace, Name: resource}
	if err := r.client.Get(ctx, key, obj); err != nil {
		// CRD not installed (multitenancy disabled) or object absent: feature
		// is inactive for this resource, degrade silently.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return Catalog{Found: false}, nil
		}

		return Catalog{}, fmt.Errorf("get %s/%s: %w", namespace, resource, err)
	}

	def, _, _ := unstructured.NestedString(obj.Object, "status", "default")

	available := make([]string, 0)
	items, _, _ := unstructured.NestedSlice(obj.Object, "status", "available")
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if name, ok, _ := unstructured.NestedString(m, "name"); ok && name != "" {
			available = append(available, name)
		}
	}

	return Catalog{
		Default:   def,
		Available: available,
		Found:     true,
	}, nil
}

// NoopResolver is a Resolver that always reports the feature as inactive.
// Useful for tests and contexts where grant resolution is not wired.
type NoopResolver struct{}

// Resolve always returns an inactive catalog.
func (NoopResolver) Resolve(_ context.Context, _, _ string) (Catalog, error) {
	return Catalog{Found: false}, nil
}
