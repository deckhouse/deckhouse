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
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Cluster and MachineHealthCheck are created via this hook (not helm) on a
// dedicated queue so that transient failures don't block the main queue.
// Hook is idempotent (CreateIfNotExists), so retries are safe.

const (
	capiNamespace = "d8-cloud-instance-manager"
)

// capiClusterInfo carries the cloud-provider registration data the hook needs.
// It mirrors the relevant subset of d8-node-manager-cloud-provider Secret keys.
type capiClusterInfo struct {
	ClusterName       string
	ClusterKind       string
	ClusterAPIVersion string
}

type capiClusterCRDConversion struct {
	Strategy string
}

func filterCapiClusterCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	strategy, _, _ := unstructured.NestedString(obj.Object, "spec", "conversion", "strategy")
	return capiClusterCRDConversion{Strategy: strategy}, nil
}

func filterCapiClusterSecret(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	data, err := decodeDataFromSecret(obj)
	if err != nil {
		return nil, err
	}
	info := capiClusterInfo{}
	if v, ok := data["capiClusterName"].(string); ok {
		info.ClusterName = v
	}
	if v, ok := data["capiClusterKind"].(string); ok {
		info.ClusterKind = v
	}
	if v, ok := data["capiClusterAPIVersion"].(string); ok {
		info.ClusterAPIVersion = v
	}
	return info, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// Own queue so a transient failure to create Cluster/MHC (capi conversion
	// webhook still warming up on first install) doesn't block the main queue.
	Queue: "/modules/node-manager/create-capi-cluster-resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Trigger when the cloud-provider registration secret appears or
			// changes — that's also the moment `nodeManager.internal.cloudProvider`
			// becomes meaningful for downstream hooks.
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-node-manager-cloud-provider"}},
			FilterFunc:   filterCapiClusterSecret,
		},
		{
			Name:       "cluster_crd",
			ApiVersion: "apiextensions.k8s.io/v1",
			Kind:       "CustomResourceDefinition",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"clusters.cluster.x-k8s.io"},
			},
			FilterFunc: filterCapiClusterCRD,
		},
	},
}, createCapiClusterResources)

func createCapiClusterResources(_ context.Context, input *go_hook.HookInput) error {
	crdSnaps := input.Snapshots.Get("cluster_crd")
	if len(crdSnaps) == 0 {
		return fmt.Errorf("CRD clusters.cluster.x-k8s.io not found yet, will retry")
	}
	var crdConv capiClusterCRDConversion
	if err := crdSnaps[0].UnmarshalTo(&crdConv); err != nil {
		return fmt.Errorf("unmarshal cluster_crd snapshot: %w", err)
	}
	if crdConv.Strategy != "Webhook" {
		return fmt.Errorf("CRD clusters.cluster.x-k8s.io conversion webhook not ready (strategy=%q), will retry", crdConv.Strategy)
	}

	snaps, err := sdkobjectpatch.UnmarshalToStruct[capiClusterInfo](input.Snapshots, "cloud_provider_secret")
	if err != nil {
		return fmt.Errorf("unmarshal cloud_provider_secret snapshot: %w", err)
	}
	if len(snaps) == 0 {
		return nil
	}
	info := snaps[0]
	if info.ClusterName == "" || info.ClusterKind == "" {
		return nil
	}
	infraAPIVersion := info.ClusterAPIVersion
	if infraAPIVersion == "" {
		infraAPIVersion = "infrastructure.cluster.x-k8s.io/v1alpha1"
	}
	// v1beta2 uses apiGroup (e.g. "infrastructure.cluster.x-k8s.io") instead of apiVersion
	infraAPIGroup := infraAPIVersion
	if idx := strings.LastIndex(infraAPIGroup, "/"); idx >= 0 {
		infraAPIGroup = infraAPIGroup[:idx]
	}

	podCIDR := input.Values.Get("global.clusterConfiguration.podSubnetCIDR").String()
	serviceCIDR := input.Values.Get("global.clusterConfiguration.serviceSubnetCIDR").String()
	serviceDomain := input.Values.Get("global.clusterConfiguration.clusterDomain").String()

	commonLabels := map[string]interface{}{
		"heritage": "deckhouse",
		"module":   "node-manager",
		"app":      "capi-controller-manager",
	}

	cluster := map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "Cluster",
		"metadata": map[string]interface{}{
			"name":      info.ClusterName,
			"namespace": capiNamespace,
			"labels":    commonLabels,
			"finalizers": []interface{}{
				"deckhouse.io/capi-controller-manager",
			},
		},
		"spec": map[string]interface{}{
			"clusterNetwork": map[string]interface{}{
				"pods":          map[string]interface{}{"cidrBlocks": []interface{}{podCIDR}},
				"services":      map[string]interface{}{"cidrBlocks": []interface{}{serviceCIDR}},
				"serviceDomain": serviceDomain,
			},
			"infrastructureRef": map[string]interface{}{
				"apiGroup": infraAPIGroup,
				"kind":     info.ClusterKind,
				"name":     info.ClusterName,
			},
			"controlPlaneRef": map[string]interface{}{
				"apiGroup": "infrastructure.cluster.x-k8s.io",
				"kind":     "DeckhouseControlPlane",
				"name":     info.ClusterName + "-control-plane",
			},
		},
	}

	mhc := map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta2",
		"kind":       "MachineHealthCheck",
		"metadata": map[string]interface{}{
			"name":      info.ClusterName + "-machine-health-check",
			"namespace": capiNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterName": info.ClusterName,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": info.ClusterName,
				},
			},
			"checks": map[string]interface{}{
				"nodeStartupTimeoutSeconds": int64(1200),
				"unhealthyNodeConditions": []interface{}{
					map[string]interface{}{"type": "Ready", "status": "Unknown", "timeoutSeconds": int64(300)},
					map[string]interface{}{"type": "Ready", "status": "False", "timeoutSeconds": int64(300)},
				},
			},
		},
	}

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
