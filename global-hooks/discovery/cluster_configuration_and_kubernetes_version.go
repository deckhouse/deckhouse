// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/global-hooks/discovery/internal"
)

var kubernetesConfigurationsForClusterConfigurationAndK8sVersion = append(
	append(
		[]go_hook.KubernetesConfig{},
		internal.KubernetesVersionConfigs...,
	),
	internal.ClusterConfigurationConfig...)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: kubernetesConfigurationsForClusterConfigurationAndK8sVersion,
}, kubernetesVersionAndClusterConfiguration)

func kubernetesVersionAndClusterConfiguration(input *go_hook.HookInput) error {
	currentK8sVersion, err := internal.KubernetesVersions(input)
	if err != nil {
		return err
	}

	return internal.ClusterConfiguration(input, currentK8sVersion)
}
