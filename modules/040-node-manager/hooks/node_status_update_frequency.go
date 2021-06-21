package hooks

import (
	"encoding/json"
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-control-plane-manager-control-plane-arguments"},
			},
			FilterFunc: updateFreqFilter,
		},
	},
}, handleUpdateFreq)

func handleUpdateFreq(input *go_hook.HookInput) error {
	snap := input.Snapshots["secret"]

	if len(snap) == 0 {
		input.Values.Remove("nodeManager.internal.nodeStatusUpdateFrequency")
		return nil
	}

	args := snap[0].(nodeArguments)
	if args.NodeMonitorGracePeriodSeconds == 0 {
		input.Values.Remove("nodeManager.internal.nodeStatusUpdateFrequency")
		return nil
	}

	freq := math.Round(float64(args.NodeMonitorGracePeriodSeconds) / 4)
	input.Values.Set("nodeManager.internal.nodeStatusUpdateFrequency", freq)

	return nil
}

type nodeArguments struct {
	NodeMonitorGracePeriodSeconds int64 `json:"nodeMonitorGracePeriod,omitempty"`
}

func updateFreqFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	argData := secret.Data["arguments.json"]

	var args nodeArguments

	err = json.Unmarshal(argData, &args)

	return args, err
}
