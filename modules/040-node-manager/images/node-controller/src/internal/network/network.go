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

// Package network resolves the three cluster network parameters
// (podSubnetCIDR, serviceSubnetCIDR, podSubnetNodeCIDRPrefix), which are being
// migrated from the deprecated ClusterConfiguration into the network group of
// ModuleConfig control-plane-manager. Every consumer (the NodeGroup webhook,
// the CAPI controllers, the bashible context, the NodeConfig renderer) must
// resolve "ModuleConfig, otherwise ClusterConfiguration" the same way, or the
// node-side pod-per-node limit and the cluster/service CIDRs handed to the
// cloud provider would silently diverge from what the control plane runs
// with. This package resolves only the ModuleConfig side: each consumer
// already reads ClusterConfiguration its own way, so it overlays this result
// on top of that existing read rather than duplicating it here.
package network

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ModuleConfigName is the ModuleConfig these parameters are being migrated into.
const ModuleConfigName = "control-plane-manager"

// ModuleConfigGVK is the GVK used to read it as unstructured.
func ModuleConfigGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig"}
}

// Settings is the network group of ModuleConfig control-plane-manager. An
// empty field means "not set there" — callers fall back to the deprecated
// same-named ClusterConfiguration field themselves.
type Settings struct {
	PodSubnetCIDR           string
	ServiceSubnetCIDR       string
	PodSubnetNodeCIDRPrefix string
}

// FromModuleConfig reads spec.settings.network off the control-plane-manager
// ModuleConfig. The ModuleConfig, the group and each field are all optional,
// so an absent object, kind or field returns the zero value rather than an
// error — the caller resolves the rest from ClusterConfiguration.
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

	out := Settings{}
	// Non-string values are dropped rather than coerced: the schema keeps all three of these
	// strings, and turning a stray number into one here would launder an object admission should
	// have rejected.
	out.PodSubnetCIDR, _ = group["podSubnetCIDR"].(string)
	out.ServiceSubnetCIDR, _ = group["serviceSubnetCIDR"].(string)
	out.PodSubnetNodeCIDRPrefix, _ = group["podSubnetNodeCIDRPrefix"].(string)
	return out, nil
}
