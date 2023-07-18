/*
Copyright 2023 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, collectMetrics)

func collectMetrics(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire("d8_admission_policy_engine_not_bootstrapped")

	if !input.Values.Get("admissionPolicyEngine.internal.bootstrapped").Bool() {
		input.MetricsCollector.Set("d8_admission_policy_engine_not_bootstrapped", 1, map[string]string{}, metrics.WithGroup("d8_admission_policy_engine_not_bootstrapped"))
	}

	return nil
}
