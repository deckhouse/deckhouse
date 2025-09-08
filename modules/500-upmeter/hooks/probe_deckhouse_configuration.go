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
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(
	&go_hook.HookConfig{
		Queue: "/modules/upmeter",
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "probe_objects",
				ApiVersion: "deckhouse.io/v1",
				Kind:       "UpmeterHookProbe",
				FilterFunc: filterProbeObject,
			},
		},
	},
	mirrorProbeValue,
)

// probeObject is an intermediate object to calculate checksums. Since addon operator does not know
// the object structure, it relies on JSON. To have non-empty JSON, we need to declare public fields
// that matter for the checksum comparison.
type probeObject struct {
	Name   string
	Inited string
}

func filterProbeObject(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	value, ok, err := unstructured.NestedString(obj.Object, "spec", "inited")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("no spec.inited field")
	}
	return probeObject{
		Name:   obj.GetName(),
		Inited: value,
	}, nil
}

func mirrorProbeValue(_ context.Context, input *go_hook.HookInput) error {
	const (
		apiVersion = "deckhouse.io/v1"
		kind       = "UpmeterHookProbe"
		namespace  = ""
	)

	probeObjects, err := sdkobjectpatch.UnmarshalToStruct[probeObject](input.Snapshots, "probe_objects")
	if err != nil {
		return fmt.Errorf("failed to unmarshal probe_objects snapshot: %w", err)
	}

	input.MetricsCollector.Set("d8_upmeter_upmeterhookprobe_count", float64(len(probeObjects)), nil)

	for _, obj := range probeObjects {
		patchRaw := map[string]interface{}{
			"spec": map[string]string{
				"mirror": obj.Inited,
			},
		}

		patch, err := json.Marshal(patchRaw)
		if err != nil {
			return err
		}

		input.PatchCollector.PatchWithMerge(patch, apiVersion, kind, namespace, obj.Name)
	}

	return nil
}
