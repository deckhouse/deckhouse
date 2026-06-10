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
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// Cluster and MachineHealthCheck are created via this hook (not helm).
// Runs OnBeforeHelm at Order 20 — after generate_capi_webhook_certs (5) and
// capi_crds_cabundle_injection (10) — so the conversion webhook is wired up
// before we touch CAPI objects. Synchronization is disabled so the hook never
// runs before the injection at startup; the Secret watch only feeds the
// snapshot. Hook is idempotent (CreateIfNotExists), so re-runs are safe.

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
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 20},
	// Own queue so a transient failure on a Secret event doesn't block the main queue.
	Queue: "/modules/node-manager/create-capi-cluster-resources",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			// Watch the cloud-provider registration secret to feed the snapshot.
			// Synchronization is disabled: the hook must not run before the
			// OnBeforeHelm injection at startup. Events still trigger re-runs on
			// the dedicated queue when the secret appears or changes.
			Name:                         "cloud_provider_secret",
			ApiVersion:                   "v1",
			Kind:                         "Secret",
			ExecuteHookOnSynchronization: ptr.To(false),
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
