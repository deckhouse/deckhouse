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
	"slices"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

// The monitoring-deckhouse module was merged into the deckhouse module.
// The standalone monitoring-deckhouse ModuleConfig is no longer used. We do not
// delete it automatically, because that would fight GitOps tooling (e.g. Argo CD
// or Deckhouse Commander) that would re-create it. Instead, we export a metric and
// fire a low-severity alert prompting the user to remove the config from the source
// of truth.

const (
	obsoleteMonitoringDeckhouseMC    = "monitoring-deckhouse"
	obsoleteMonitoringDeckhouseGroup = "D8DeckhouseObsoleteMonitoringDeckhouseModuleConfig"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/deckhouse/alerting",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "obsolete-monitoring-deckhouse-mc",
			ApiVersion:                   "deckhouse.io/v1alpha1",
			Kind:                         "ModuleConfig",
			NameSelector:                 &types.NameSelector{MatchNames: []string{obsoleteMonitoringDeckhouseMC}},
			ExecuteHookOnSynchronization: ptr.To(true),
			ExecuteHookOnEvents:          ptr.To(true),
			FilterFunc:                   filterModuleConfigName,
		},
	},
}, alertObsoleteMonitoringDeckhouseMC)

func filterModuleConfigName(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

func alertObsoleteMonitoringDeckhouseMC(_ context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(obsoleteMonitoringDeckhouseGroup)

	names, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "obsolete-monitoring-deckhouse-mc")
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'obsolete-monitoring-deckhouse-mc' snapshot: %w", err)
	}

	if !slices.Contains(names, obsoleteMonitoringDeckhouseMC) {
		return nil
	}

	input.MetricsCollector.Set(
		"d8_deckhouse_obsolete_moduleconfig",
		1,
		map[string]string{"moduleconfig": obsoleteMonitoringDeckhouseMC},
		metrics.WithGroup(obsoleteMonitoringDeckhouseGroup),
	)

	return nil
}
