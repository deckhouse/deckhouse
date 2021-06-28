package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RemoteWrite struct {
	Name string                 `json:"name"`
	Spec map[string]interface{} `json:"spec"`
}

func filterRemoteWriteCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from PrometheusRemoteWrite: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("prometheusRemoteWrite has no spec field")
	}

	rw := new(RemoteWrite)
	rw.Name = obj.GetName()
	rw.Spec = spec

	return rw, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/remote_write",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "prometheusremotewrite",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "PrometheusRemoteWrite",
			FilterFunc: filterRemoteWriteCRD,
		},
	},
}, remoteWriteHandler)

func remoteWriteHandler(input *go_hook.HookInput) error {
	prw := input.Snapshots["prometheusremotewrite"]

	if len(prw) == 0 {
		input.Values.Set("prometheus.internal.remoteWrite", make([]interface{}, 0))
		return nil
	}

	input.Values.Set("prometheus.internal.remoteWrite", prw)

	return nil
}
