/*
Copyright 2022 Flant JSC

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

package telemetry

import (
	"github.com/deckhouse/deckhouse/go_lib/telemetry"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type ActionFunc func(input *go_hook.HookInput, telemetryCollector telemetry.MetricsCollector) error

func RegisterHook(action ActionFunc) bool {
	handler := func(input *go_hook.HookInput) error {
		collector := telemetry.NewTelemetryMetricCollector(input)
		err := action(input, collector)
		if err != nil {
			input.LogEntry.Errorf("Telemetry action run got error: %s", err)
		}

		return nil
	}

	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnAfterAll: &go_hook.OrderedConfig{Order: 100},
		// telemetry should not block Deckhouse
		AllowFailure: true,
	}, handler)
}

func RegisterScheduledHook(config go_hook.ScheduleConfig, action ActionFunc) bool {
	handler := func(input *go_hook.HookInput) error {
		collector := telemetry.NewTelemetryMetricCollector(input)
		err := action(input, collector)
		if err != nil {
			input.LogEntry.Errorf("Telemetry action run got error: %s", err)
		}

		return nil
	}

	return sdk.RegisterFunc(&go_hook.HookConfig{
		Schedule: []go_hook.ScheduleConfig{config},
		// telemetry should not block Deckhouse
		AllowFailure: true,
	}, handler)
}
