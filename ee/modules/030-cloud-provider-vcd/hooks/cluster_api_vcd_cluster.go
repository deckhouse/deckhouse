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

const clusterAPINamespace = "d8-cloud-instance-manager"

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
			{
				Name:       "cluster",
				ApiVersion: "cluster.x-k8s.io/v1beta2",
				Kind:       "Cluster",
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{clusterAPINamespace},
					},
				},
				FilterFunc: filterCluster,
			},
		},
	},
	updateClusterInfrastructureProvisioned,
)

type vcdCluster struct {
	Name  string
	Ready bool
}

type cluster struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
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

func filterCluster(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return cluster{
		APIVersion: obj.GetAPIVersion(),
		Kind:       obj.GetKind(),
		Name:       obj.GetName(),
		Namespace:  obj.GetNamespace(),
	}, nil
}

func updateClusterInfrastructureProvisioned(_ context.Context, input *go_hook.HookInput) error {
	// Build a map of ready VCDClusters
	readyVCDClusters := make(map[string]bool)
	for vcdCluster, err := range sdkobjectpatch.SnapshotIter[vcdCluster](input.Snapshots.Get("vcd_cluster")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'vcd_cluster': %w", err)
		}
		if vcdCluster.Ready {
			readyVCDClusters[vcdCluster.Name] = true
		}
	}

	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"initialization": map[string]interface{}{
				"infrastructureProvisioned": true,
			},
		},
	}

	// Patch Clusters that have ready VCDCluster infrastructure
	for cluster, err := range sdkobjectpatch.SnapshotIter[cluster](input.Snapshots.Get("cluster")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'cluster': %w", err)
		}

		// Check if corresponding VCDCluster is ready
		if readyVCDClusters[cluster.Name] {
			// Patch cluster status
			input.PatchCollector.PatchWithMerge(
				statusPatch,
				cluster.APIVersion,
				cluster.Kind,
				cluster.Namespace,
				cluster.Name,
				object_patch.WithIgnoreMissingObject(),
				object_patch.WithSubresource("/status"),
			)
		}
	}

	return nil
}
