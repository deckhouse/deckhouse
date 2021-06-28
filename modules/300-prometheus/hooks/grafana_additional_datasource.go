package hooks

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type GrafanaAdditionalDatasource map[string]interface{}

func filterGrafanaDSCRD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	spec, ok, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("cannot get spec from GrafanaAdditionalDatasource: %v", err)
	}
	if !ok {
		return nil, fmt.Errorf("has no spec field in GrafanaAdditionalDatasource")
	}

	spec["orgId"] = 1
	spec["name"] = obj.GetName()
	spec["uuid"] = obj.GetName()
	spec["isDefault"] = false
	spec["version"] = 1
	spec["editable"] = false

	n := GrafanaAdditionalDatasource(spec)

	return &n, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/prometheus/grafana_additional_datasource",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "grafana_additional_datasources",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "GrafanaAdditionalDatasource",
			FilterFunc: filterGrafanaDSCRD,
		},
	},
}, grafanaDatasourcesHandler)

func grafanaDatasourcesHandler(input *go_hook.HookInput) error {
	gad := input.Snapshots["grafana_additional_datasources"]

	if len(gad) == 0 {
		input.Values.Set("prometheus.internal.grafana.additionalDatasources", make([]interface{}, 0))
		return nil
	}

	input.Values.Set("prometheus.internal.grafana.additionalDatasources", gad)

	return nil
}
