package hooks

import (
	"os"
	"strconv"
	"time"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"

	hook "github.com/deckhouse/deckhouse/go_lib/hooks/telemetry"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/go_lib/telemetry"
)

var config = go_hook.ScheduleConfig{
	Name:    "Deckhouse telemetry schedule",
	Crontab: "* */3 * * * *", // every 3 minutes
}

func timeNow() time.Time {
	envTime := os.Getenv("TEST_NOW_TIME")
	if envTime == "" {
		return time.Now()
	}

	t, err := strconv.ParseInt(envTime, 10, 64)
	if err != nil {
		panic(err)
	}

	return time.Unix(t, 0)
}

var _ = hook.RegisterScheduledHook(config, func(input *go_hook.HookInput, telemetryCollector telemetry.MetricsCollector) error {
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

	return &windowTelemetryHook{
		windows:      windows,
		approvalMode: approvalMode,
		collector:    telemetryCollector,
	}, nil
}

func (h *windowTelemetryHook) setWindowsMetrics() error {
	if h.approvalMode == "Auto" && len(h.windows) > 0 {
		h.setWindowMetrics()
		h.setFlattenWindowsMetrics()
		if err := h.setRawWindowsMetrics(); err != nil {
			return err
		}
	}

	return nil
}

func (h *windowTelemetryHook) setRawWindowsMetrics() error {
	rawWindow, err := h.windows.ToJSON()
	if err != nil {
		return err
	}
	const group = "deckhouse_update_window_raw"
	h.collector.Expire(group)
	h.collector.Set(group, 1.0, map[string]string{
		"raw": string(rawWindow),
	}, telemetry.NewOptions().WithGroup(group))

	return nil
}

func (h *windowTelemetryHook) setFlattenWindowsMetrics() {
	const group = "deckhouse_update_window"
	h.collector.Expire(group)

	for _, w := range h.windows {
		for _, day := range w.Days {
			h.collector.Set(group, 1.0, map[string]string{
				"from": w.From,
				"to":   w.To,
				"day":  day,
			}, telemetry.NewOptions().WithGroup(group))
		}
	}
}

func (h *windowTelemetryHook) setUpdateModeMetrics() {
	const group = "deckhouse_update_window_approval_mode"
	h.collector.Expire(group)

	if h.approvalMode == "" {
		h.approvalMode = "NotSet"
	}

	h.collector.Set(group, 1.0, map[string]string{
		"mode": h.approvalMode,
	}, telemetry.NewOptions().WithGroup(group))
}

func (h *windowTelemetryHook) setWindowMetrics() {
	now := timeNow()
	var fromTime time.Time
	var toTime time.Time
	for _, w := range h.windows {
		if w.IsAllowed(now) {
			fromTime = w.FromTime(now)
			toTime = w.ToTime(now)
			break
		}
	}

	if fromTime.IsZero() {
		fromTime, toTime = h.windows.NextAllowedWindow(now)
	}

	const group = "update_window_next"
	h.collector.Expire(group)

	h.collector.Set(
		"update_window_next_from",
		float64(fromTime.UnixNano()),
		map[string]string{},
		telemetry.NewOptions().WithGroup(group),
	)

	h.collector.Set(
		"update_window_next_to",
		float64(toTime.UnixNano()),
		map[string]string{},
		telemetry.NewOptions().WithGroup(group),
	)
}
