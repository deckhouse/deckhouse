/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"context"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	clusterAPINamespace = "d8-cloud-instance-manager"
	// clusterWatchAPIVersion is the API version used when patching Cluster status.
	// clusters.cluster.x-k8s.io CRD is installed by node-manager and may not exist
	// during early bootstrap — do NOT add a kubernetes watch for it here.
	clusterWatchAPIVersion = "cluster.x-k8s.io/v1beta1"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/cloud-provider-vcd/cluster-api",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "vcd_cluster",
				ApiVersion: "infrastructure.cluster.x-k8s.io/v1beta2",
				Kind:       "VCDCluster",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{clusterAPINamespace},
					},
				},
				FilterFunc: filterVCDCluster,
			},
		},
	},
	updateClusterInfrastructureProvisioned,
)

type vcdCluster struct {
	Name  string
	Ready bool
}

func filterVCDCluster(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	ready, found, err := unstructured.NestedBool(obj.Object, "status", "ready")
	if err != nil {
		return nil, fmt.Errorf("failed to get status.ready: %w", err)
	}
	if !found {
		ready = false
	}

	return vcdCluster{
		Name:  obj.GetName(),
		Ready: ready,
	}, nil
}

func updateClusterInfrastructureProvisioned(_ context.Context, input *go_hook.HookInput) error {
	readyVCDClusters := make(map[string]bool)
	for vc, err := range sdkobjectpatch.SnapshotIter[vcdCluster](input.Snapshots.Get("vcd_cluster")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'vcd_cluster': %w", err)
		}
		if vc.Ready {
			readyVCDClusters[vc.Name] = true
		}
	}

	if len(readyVCDClusters) == 0 {
		return nil
	}

	// Patch v1beta1 status fields that CAPI maps to status.initialization.infrastructureProvisioned
	// and status.initialization.controlPlaneInitialized (required by CAPI v1.12 / CAPVCD).
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"infrastructureReady": true,
			"controlPlaneReady":   true,
		},
	}

	// Cluster.name == VCDCluster.name by CAPI convention; patch by name to avoid
	// watching clusters.cluster.x-k8s.io, which does not exist until node-manager runs.
	for name := range readyVCDClusters {
		input.PatchCollector.PatchWithMerge(
			statusPatch,
			clusterWatchAPIVersion,
			"Cluster",
			clusterAPINamespace,
			name,
			object_patch.WithIgnoreMissingObject(),
			object_patch.WithSubresource("/status"),
		)
	}

	return nil
}
