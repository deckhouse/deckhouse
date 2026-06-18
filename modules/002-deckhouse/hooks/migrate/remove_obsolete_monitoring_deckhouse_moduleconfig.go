/*
Copyright 2026 Flant JSC

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

package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// The monitoring-deckhouse module was merged into the deckhouse module.
// Remove the orphaned monitoring-deckhouse ModuleConfig left on existing clusters.

const obsoleteMonitoringDeckhouseMC = "monitoring-deckhouse"

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/migrate",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "obsolete-monitoring-deckhouse-mc",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleConfig",
			NameSelector:                 &types.NameSelector{MatchNames: []string{obsoleteMonitoringDeckhouseMC}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(false),
			FilterFunc:                   filterModuleConfigName,
		},
	},
}, removeObsoleteMonitoringDeckhouseMC)

func filterModuleConfigName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func removeObsoleteMonitoringDeckhouseMC(_ context.Context, input *go_hook.HookInput) error {
	names, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "obsolete-monitoring-deckhouse-mc")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'obsolete-monitoring-deckhouse-mc' snapshot: %w", err)
	}

	if len(names) == 0 {
		return nil
	}

	input.Logger.Warn("Removing obsolete ModuleConfig", slog.String("name", obsoleteMonitoringDeckhouseMC))
	input.PatchCollector.Delete("deckhouse.io/v1alpha1", "ModuleConfig", "", obsoleteMonitoringDeckhouseMC)

	return nil
}
