package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "mc",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"deckhouse-web"},
			},
			ExecuteHookOnEvents:          pointer.Bool(true),
			ExecuteHookOnSynchronization: pointer.Bool(true),
			FilterFunc:                   filterMC,
		},
	},
}, setAlertMetrics)

func filterMC(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func setAlertMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_mc")

	if len(input.Snapshots["mc"]) > 0 {
		input.MetricsCollector.Set("d8_mc_deprecated", 1, map[string]string{"module": "deckhouse-web"}, metrics.WithGroup("d8_mc"))
	}

	return nil
}
