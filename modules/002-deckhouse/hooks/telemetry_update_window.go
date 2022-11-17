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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 1000},
	// telemetry should not block Deckhouse
	AllowFailure: true,
}, updateWindowTelemetry)

func updateWindowTelemetry(input *go_hook.HookInput) error {
	telemetryCollector := input.MetricsCollector

	h, err := newWindowTelemetryHook(input, telemetryCollector)
	if err != nil {
		return err
	}

	return h.setWindowsMetrics()
}

type windowTelemetryHook struct {
	approvalMode string
	collector    go_hook.MetricsCollector
	windows      update.Windows
}

func newWindowTelemetryHook(input *go_hook.HookInput, telemetryCollector go_hook.MetricsCollector) (*windowTelemetryHook, error) {
	windows, err := getUpdateWindows(input)
	if err != nil {
		return nil, err
	}
	approvalMode := input.Values.Get("deckhouse.update.mode").String()
	if approvalMode == "" {
		approvalMode = "NotSet"
	}

	return &windowTelemetryHook{
		windows:      windows,
		approvalMode: approvalMode,
		collector:    telemetryCollector,
	}, nil
}

func (h *windowTelemetryHook) setWindowsMetrics() error {
	h.collector.Set(telemetry.WrapName("update_window_approval_mode"), 1.0, map[string]string{
		"mode": h.approvalMode,
	})

	if h.approvalMode == "Auto" && len(h.windows) > 0 {
		h.setFlattenWindowsMetrics()
	}

	return nil
}

func (h *windowTelemetryHook) setFlattenWindowsMetrics() {
	metricName := telemetry.WrapName("update_window")

	for _, w := range h.windows {
		for _, day := range w.Days {
			d := day
			h.collector.Set(metricName, 1.0, map[string]string{
				"from": w.From,
				"to":   w.To,
				"day":  d,
			})
		}

		if len(w.Days) == 0 {
			h.collector.Set(metricName, 1.0, map[string]string{
				"from": w.From,
				"to":   w.To,
				"day":  "",
			})
		}
	}
}
