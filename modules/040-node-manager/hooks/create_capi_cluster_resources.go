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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// Cluster / MachineHealthCheck (cluster.x-k8s.io/v1beta1) go through a
// conversion webhook served by capi-webhook-service → capi-controller-manager.
// Rendering them via the node-manager helm release races the very first install
// of that Deployment: apiserver calls the webhook during SSA, gets connection-
// refused, helm retries with 45s backoff (~4 retries = ~3 minutes lost on the
// main queue).
//
// Owning them in a hook on a dedicated queue removes them from helm's apply
// list — helm install of node-manager succeeds on the first try, and the hook
// retries the create until the webhook backend is up. The objects are not on
// the bootstrap critical path: nothing helm-rendered references them during
// initial install (MachineDeployment is only emitted for Cloud/CloudEphemeral
// NodeGroups, and master NG is CloudPermanent).
//
// Hook is idempotent (CreateIfNotExists), so retries are safe.

const (
	capiNamespace = "d8-cloud-instance-manager"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/create-capi-cluster-resources",
	// Order > create_master_node_group (6) so it runs in the same OnStartup
	// pass but after the CRDs ensure step (order 5) and master NG hook (6).
	OnStartup: &go_hook.OrderedConfig{Order: 7},
}, createCapiClusterResources)

func createCapiClusterResources(_ context.Context, input *go_hook.HookInput) error {
	if !input.Values.Get("nodeManager.internal.capiControllerManagerEnabled").Bool() {
		return nil
	}

	prefix := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
	infraKind := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterKind").String()
	if prefix == "" || infraKind == "" {
		return nil
	}
	infraAPIVersion := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterAPIVersion").String()
	if infraAPIVersion == "" {
		infraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}

	podCIDR := input.Values.Get("global.clusterConfiguration.podSubnetCIDR").String()
	serviceCIDR := input.Values.Get("global.clusterConfiguration.serviceSubnetCIDR").String()
	serviceDomain := input.Values.Get("global.clusterConfiguration.clusterDomain").String()

	commonMeta := map[string]interface{}{
		"namespace": capiNamespace,
		"labels": map[string]interface{}{
			"heritage": "deckhouse",
			"module":   "node-manager",
			"app":      "capi-controller-manager",
		},
	}

	clusterMeta := mergeMap(commonMeta, map[string]interface{}{
		"name": prefix,
		"finalizers": []interface{}{
			"deckhouse.io/capi-controller-manager",
		},
	})
	cluster := unstructuredFrom("cluster.x-k8s.io/v1beta1", "Cluster", clusterMeta, map[string]interface{}{
		"clusterNetwork": map[string]interface{}{
			"pods":          map[string]interface{}{"cidrBlocks": []interface{}{podCIDR}},
			"services":      map[string]interface{}{"cidrBlocks": []interface{}{serviceCIDR}},
			"serviceDomain": serviceDomain,
		},
		"infrastructureRef": map[string]interface{}{
			"apiVersion": infraAPIVersion,
			"kind":       infraKind,
			"namespace":  capiNamespace,
			"name":       prefix,
		},
		"controlPlaneRef": map[string]interface{}{
			"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
			"kind":       "DeckhouseControlPlane",
			"namespace":  capiNamespace,
			"name":       prefix + "-control-plane",
		},
	})

	mhcMeta := mergeMap(commonMeta, map[string]interface{}{
		"name": prefix + "-machine-health-check",
	})
	mhc := unstructuredFrom("cluster.x-k8s.io/v1beta1", "MachineHealthCheck", mhcMeta, map[string]interface{}{
		"clusterName":        prefix,
		"nodeStartupTimeout": "20m",
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"cluster.x-k8s.io/cluster-name": prefix,
			},
		},
		"unhealthyConditions": []interface{}{
			map[string]interface{}{"type": "Ready", "status": "Unknown", "timeout": "5m"},
			map[string]interface{}{"type": "Ready", "status": "False", "timeout": "5m"},
		},
	})

	clusterU, err := sdk.ToUnstructured(&cluster)
	if err != nil {
		return err
	}
	mhcU, err := sdk.ToUnstructured(&mhc)
	if err != nil {
		return err
	}

	input.PatchCollector.CreateIfNotExists(clusterU)
	input.PatchCollector.CreateIfNotExists(mhcU)

	return nil
}

func unstructuredFrom(apiVersion, kind string, metadata, spec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   metadata,
		"spec":       spec,
	}
}

func mergeMap(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

