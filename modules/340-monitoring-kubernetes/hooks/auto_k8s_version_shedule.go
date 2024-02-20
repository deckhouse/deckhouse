/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	helmreleases "github.com/deckhouse/deckhouse/modules/340-monitoring-kubernetes/hooks/internal"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes/auto_k8s_version_schedule",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "auto_k8s_version",
			Crontab: "0 * * * *", // every hour
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:              "kubernetesVersion",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-cluster-configuration"}},
			FilterFunc:        applyClusterConfigurationYamlFilter,

			// only snapshot update is needed
			ExecuteHookOnEvents:          go_hook.Bool(false),
			ExecuteHookOnSynchronization: go_hook.Bool(false),
		},
	},
}, dependency.WithExternalDependencies(clusterConfigurationBySchedule))

func clusterConfigurationBySchedule(input *go_hook.HookInput, dc dependency.Container) error {
	return clusterConfiguration(input, dc, helmreleases.IntervalImmediately)
}
