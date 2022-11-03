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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	hook "github.com/deckhouse/deckhouse/go_lib/hooks/telemetry"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

var _ = hook.RegisterHook(func(input *go_hook.HookInput, telemetryCollector telemetry.MetricsCollector) error {
	h, err := newWindowTelemetryHook(input, telemetryCollector)
	if err != nil {
		return err
	}

	h.setUpdateModeMetrics()

	return h.setWindowsMetrics()
})

type windowTelemetryHook struct {
	approvalMode string
	collector    telemetry.MetricsCollector
	windows      update.Windows
}

func newWindowTelemetryHook(input *go_hook.HookInput, telemetryCollector telemetry.MetricsCollector) (*windowTelemetryHook, error) {
	windows, err := getUpdateWindows(input)
	if err != nil {
		return nil, err
	}
	approvalMode := input.Values.Get("deckhouse.update.mode").String()
	if approvalMode == "" {
		approvalMode = "Auto"
	}

	return &windowTelemetryHook{
		windows:      windows,
		approvalMode: approvalMode,
		collector:    telemetryCollector,
	}, nil
}

func (h *windowTelemetryHook) setWindowsMetrics() error {
	if h.approvalMode == "Auto" && len(h.windows) > 0 {
		h.setFlattenWindowsMetrics()
	}

	return nil
}

func (h *windowTelemetryHook) humanOut(w update.Window, day string) string {
	human := fmt.Sprintf("%s - %s", w.From, w.To)
	if day != "" {
		human = fmt.Sprintf("%s - %s", day, human)
	}

	return human
}

func (h *windowTelemetryHook) setFlattenWindowsMetrics() {
	const group = "update_window"
	h.collector.Expire(group)

	for _, w := range h.windows {
		for _, day := range w.Days {
			d := day
			h.collector.Set(group, 1.0, map[string]string{
				"from":  w.From,
				"to":    w.To,
				"day":   d,
				"human": h.humanOut(w, d),
			}, telemetry.NewOptions().WithGroup(group))
		}

		if len(w.Days) == 0 {
			h.collector.Set(group, 1.0, map[string]string{
				"from":  w.From,
				"to":    w.To,
				"human": h.humanOut(w, ""),
			}, telemetry.NewOptions().WithGroup(group))
		}
	}
}

func (h *windowTelemetryHook) setUpdateModeMetrics() {
	const group = "update_window_approval_mode"
	h.collector.Expire(group)

	h.collector.Set(group, 1.0, map[string]string{
		"mode": h.approvalMode,
	}, telemetry.NewOptions().WithGroup(group))
}
