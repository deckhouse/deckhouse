/*
Copyright 2025 Flant JSC

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
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	// The Order below matters for ensure_crds_istio.go, it needs globalVersion to deploy proper CRDs
	OnStartup:    &go_hook.OrderedConfig{Order: 5},
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 5},
}, setGrafanaDeployFlag)

func setGrafanaDeployFlag(input *go_hook.HookInput) error {
	// Stub logic for future use
	if grafanaEnabled, ok := input.ConfigValues.GetOk("prometheus.grafana.enabled"); ok {
		input.Values.Set("prometheus.internal.grafana.enabled", grafanaEnabled.Bool())
	}

	return nil
}
