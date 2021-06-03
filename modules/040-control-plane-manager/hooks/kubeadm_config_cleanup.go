package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cm_kubeadm_config",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"kubeadm-config"},
			},
			FilterFunc: func(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
				return unstructured, nil
			},
		},
	},
}, handleKubeadmConfig)

func handleKubeadmConfig(input *go_hook.HookInput) error {
	snap, ok := input.Snapshots["cm_kubeadm_config"]
	if !ok {
		return nil
	}

	if len(snap) == 0 {
		return nil
	}
	input.LogEntry.Info("Deleting CM kubeadm-config")
	return input.ObjectPatcher.DeleteObject("v1", "ConfigMap", "kube-system", "kubeadm-config", "")
}
