// Copyright 2022 Flant JSC
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

package telemetry

import (
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/module"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterAll: &go_hook.OrderedConfig{Order: float64(1000)},
}, setTelemetry)

func setTelemetry(input *go_hook.HookInput) error {
	collector := telemetry.NewTelemetryMetricCollector(input)
	const group = "modules_enable"
	collector.Expire(group)

	modules := []string{"istio"}
	for _, m := range modules {
		val := 0.0
		if module.IsEnabled(m, input) {
			val = 1.0
		}
		collector.Set(
			fmt.Sprintf("%s_module_enabled", m),
			val,
			nil,
			telemetry.NewOptions().WithGroup(group))
	}

	return nil
}
