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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Cluster / MachineHealthCheck (cluster.x-k8s.io/v1beta1) go through a
// conversion webhook served by capi-webhook-service → capi-controller-manager.
// They must be created AFTER the webhook endpoint is available and the CA
// bundle has been injected into the CRDs. Running this hook in AfterHelm
// (Order 10) guarantees that:
//   - capi-controller-manager Deployment is running (deployed in pre-install phase)
//   - TLS Secret is created (deployed in Helm main phase)
//   - CA bundle is injected into CRDs by capi_crds_cabundle_injection hook (AfterHelm Order 1)
//
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
	// Run after Helm so that the conversion webhook (capi-controller-manager) is
	// already deployed and the CA bundle has been injected into CRDs by the
	// capi_crds_cabundle_injection hook (AfterHelm Order 1).
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:       "/modules/node-manager/create-capi-cluster-resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cloud_provider_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{"d8-node-manager-cloud-provider"}},
			FilterFunc:   filterCapiClusterSecret,
		},
	},
}, createCapiClusterResources)

func createCapiClusterResources(_ context.Context, input *go_hook.HookInput) error {
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

	podCIDR := input.Values.Get("global.clusterConfiguration.podSubnetCIDR").String()
	serviceCIDR := input.Values.Get("global.clusterConfiguration.serviceSubnetCIDR").String()
	serviceDomain := input.Values.Get("global.clusterConfiguration.clusterDomain").String()

	commonLabels := map[string]interface{}{
		"heritage": "deckhouse",
		"module":   "node-manager",
		"app":      "capi-controller-manager",
	}

	cluster := map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta1",
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
				"apiVersion": infraAPIVersion,
				"kind":       info.ClusterKind,
				"namespace":  capiNamespace,
				"name":       info.ClusterName,
			},
			"controlPlaneRef": map[string]interface{}{
				"apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha1",
				"kind":       "DeckhouseControlPlane",
				"namespace":  capiNamespace,
				"name":       info.ClusterName + "-control-plane",
			},
		},
	}

	mhc := map[string]interface{}{
		"apiVersion": "cluster.x-k8s.io/v1beta1",
		"kind":       "MachineHealthCheck",
		"metadata": map[string]interface{}{
			"name":      info.ClusterName + "-machine-health-check",
			"namespace": capiNamespace,
			"labels":    commonLabels,
		},
		"spec": map[string]interface{}{
			"clusterName":        info.ClusterName,
			"nodeStartupTimeout": "20m",
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"cluster.x-k8s.io/cluster-name": info.ClusterName,
				},
			},
			"unhealthyConditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "Unknown", "timeout": "5m"},
				map[string]interface{}{"type": "Ready", "status": "False", "timeout": "5m"},
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
