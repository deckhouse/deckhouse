/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
package migrate

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func applyConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	cm := &v1.ConfigMap{}
	err := sdk.FromUnstructured(obj, cm)
	if err != nil {
		return nil, err
	}

	return true, nil
}

var (
	_ = sdk.RegisterFunc(&go_hook.HookConfig{
		Queue: "/modules/flant-integration",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "d8_migrate_cluster_kubernetes_version_cm",
				ApiVersion: "v1",
				Kind:       "ConfigMap",
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-migrate-cluster-kubernetes-version"},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{"d8-flant-integration"},
					},
				},
				FilterFunc: applyConfigMapFilter,
			},
		},
	}, removeMigration)
)

func removeMigration(input *go_hook.HookInput) error {
	cm := input.Snapshots["d8_migrate_cluster_kubernetes_version_cm"]
	if len(cm) == 1 && cm[0].(bool) {
		input.LogEntry.Info(`find d8-flant-integration/d8-migrate-cluster-kubernetes-version configMap, remove it`)
		input.PatchCollector.Delete("v1", "ConfigMap", "d8-flant-integration", "d8-migrate-cluster-kubernetes-version")
	}

	return nil
}
