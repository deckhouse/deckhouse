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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

const (
	deckhouseModuleVersionMetricName = "deckhouse_module_version"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "modules",
			ApiVersion: "deckhouse.io/v1alpha1",
			Kind:       "Module",
			FilterFunc: filterModuleVersion,
		},
	},
}, collectMetrics)

// moduleVersionSnapshot is the filter result for Module CR: name and version for telemetry.
type moduleVersionSnapshot struct {
	Name    string
	Version string
}

func filterModuleVersion(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, _ := unstructured.NestedString(obj.Object, "properties", "version")
	if version == "" {
		return nil, nil
	}
	name := obj.GetName()
	return moduleVersionSnapshot{Name: name, Version: version}, nil
}

func collectMetrics(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Set("deckhouse_release_channel", 1, map[string]string{
		"release_channel": input.Values.Get("deckhouse.releaseChannel").String(),
	})

	input.MetricsCollector.Set(telemetry.WrapName("update_window_approval_mode"), 1, map[string]string{
		"mode": input.Values.Get("deckhouse.update.mode").String(),
	})

	windowsData, exists := input.Values.GetOk("deckhouse.update.windows")
	if exists {
		windows, err := update.FromJSON([]byte(windowsData.Raw))
		if err != nil {
			return err
		}

		for _, windows := range windows {
			input.MetricsCollector.Set(telemetry.WrapName("update_window"), 1, map[string]string{
				"from": windows.From,
				"to":   windows.To,
				"days": strings.Join(windows.Days, " "),
			})
		}
	}
	snapshots := input.Snapshots.Get("modules")
	for snap, err := range sdkobjectpatch.SnapshotIter[moduleVersionSnapshot](snapshots) {
		if err != nil {
			continue
		}
		input.MetricsCollector.Set(telemetry.WrapName(deckhouseModuleVersionMetricName), 1, map[string]string{
			"module_name":    snap.Name,
			"module_version": snap.Version,
		})
	}
	return nil
}
