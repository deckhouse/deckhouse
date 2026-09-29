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

// TODO: Remove/change when cluster-configuration is removed to use only ModuleConfig control-plane-manager

// Package network resolves network parameters and clusterDomain from ModuleConfig
// control-plane-manager; every consumer must resolve them the same way or node state silently diverges.
package network

import (
	"context"
	"fmt"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ModuleConfigName is the ModuleConfig these parameters live in.
const ModuleConfigName = "control-plane-manager"

// Used when neither ModuleConfig nor ClusterConfiguration sets the domain.
const DefaultClusterDomain = "cluster.local"

// ModuleConfigGVK is the GVK used to read it as unstructured.
func ModuleConfigGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
}

// An empty field means "not set here"; callers fall back to ClusterConfiguration themselves.
type Settings struct {
	PodSubnetCIDR           string
	ServiceSubnetCIDR       string
	PodSubnetNodeCIDRPrefix string
	ClusterDomain           string
}

// FromModuleConfig reads spec.settings.network. Everything here is optional, so an absent object,
// kind or field returns the zero value rather than an error.
func FromModuleConfig(ctx context.Context, reader client.Reader) (Settings, error) {
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(ModuleConfigGVK())
	if err := reader.Get(ctx, types.NamespacedName{Name: ModuleConfigName}, mc); err != nil {
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("get %s ModuleConfig: %w", ModuleConfigName, err)
	}

	group, found, err := unstructured.NestedMap(mc.Object, "spec", "settings", "network")
	if err != nil {
		return Settings{}, fmt.Errorf("read spec.settings.network from %s ModuleConfig: %w", ModuleConfigName, err)
	}
	if !found {
		return Settings{}, nil
	}

	// Non-strings are dropped, not coerced: coercing would launder an object admission rejects.
	out := Settings{}
	out.PodSubnetCIDR, _ = group["podSubnetCIDR"].(string)
	out.ServiceSubnetCIDR, _ = group["serviceSubnetCIDR"].(string)
	out.PodSubnetNodeCIDRPrefix, _ = group["podSubnetNodeCIDRPrefix"].(string)
	out.ClusterDomain, _ = group["clusterDomain"].(string)
	return out, nil
}

// NetworkGroupChanged reports whether spec.settings.network differs between two ModuleConfig
// revisions. A "cannot tell" answers true rather than silently dropping the event.
func NetworkGroupChanged(oldObj, newObj client.Object) bool {
	oldU, ok := oldObj.(*unstructured.Unstructured)
	if !ok {
		return true
	}
	newU, ok := newObj.(*unstructured.Unstructured)
	if !ok {
		return true
	}

	path := []string{"spec", "settings", "network"}
	oldValue, _, err := unstructured.NestedFieldNoCopy(oldU.UnstructuredContent(), path...)
	if err != nil {
		return true
	}
	newValue, _, err := unstructured.NestedFieldNoCopy(newU.UnstructuredContent(), path...)
	if err != nil {
		return true
	}
	return !apiequality.Semantic.DeepEqual(oldValue, newValue)
}
