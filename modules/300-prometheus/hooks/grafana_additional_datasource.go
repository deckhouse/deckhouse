/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"context"
	"fmt"
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
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
	spec["uid"] = obj.GetName()
	spec["isDefault"] = false
	spec["version"] = 1
	spec["editable"] = false
	access, ok := spec["access"].(string)
	if ok {
		// For grafana datasource we have to change access (Direct/Proxy) to direct/proxy
		spec["access"] = strings.ToLower(access)
	}

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

func grafanaDatasourcesHandler(_ context.Context, input *go_hook.HookInput) error {
	gad, err := sdkobjectpatch.UnmarshalToStruct[GrafanaAdditionalDatasource](input.Snapshots, "grafana_additional_datasources")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'grafana_additional_datasources' snapshots: %w", err)
	}

	if len(gad) == 0 {
		input.Values.Set("prometheus.internal.grafana.additionalDatasources", make([]interface{}, 0))
		return nil
	}

	input.Values.Set("prometheus.internal.grafana.additionalDatasources", gad)

	return nil
}
