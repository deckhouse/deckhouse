package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "exporter-cm",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-admission-policy-engine"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"constraint-exporter"},
			},
			FilterFunc: filterExporterCM,
		},
	},
}, xxxx)

func xxxx(input *go_hook.HookInput) error {
	snap := input.Snapshots["exporter-cm"]
	if len(snap) == 0 {
		input.LogEntry.Info("no exporter cm found")
		return nil
	}

	kinds := snap[0].(string)

	input.LogEntry.Infof("Find kinds: %s", kinds)

	input.Values.Set("admissionPolicyEngine.internal.trackedKinds", kinds)

	return nil
}

func filterExporterCM(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var cm corev1.ConfigMap

	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		return nil, err
	}

	return cm.Data["kinds.yaml"], nil
}
