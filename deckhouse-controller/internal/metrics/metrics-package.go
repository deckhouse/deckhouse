// Copyright 2026 Flant JSC
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

package metrics

import (
	"fmt"
	"time"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/options"
)

// Gauges of the package runtime: the last duration of a hook, of a phase of a package's run or
// startup, and of a convergence, and the state of the hook lanes.
const (
	// HookLastRunSeconds is the duration of the last run of a hook.
	HookLastRunSeconds = "deckhouse_hook_last_run_seconds"
	// PackageRunLastSeconds is the duration of the last successful phase of a package's run.
	PackageRunLastSeconds = "deckhouse_package_run_last_seconds"
	// PackageStartupLastSeconds is the duration of the last successful phase of a package's startup.
	PackageStartupLastSeconds = "deckhouse_package_startup_last_seconds"
	// ConvergenceLastSeconds is the duration of the last convergence, a startup one included.
	ConvergenceLastSeconds = "deckhouse_convergence_last_seconds"
	// HooklaneLength is the number of items in a hook lane, the one in flight included.
	HooklaneLength = "deckhouse_hooklane_length"
	// HooklaneState is 1 for the state a hook lane is in and 0 for the others.
	HooklaneState = "deckhouse_hooklane_state"
)

// Phases of a package's run and startup.
const (
	PhaseBeforeHelm = "before_helm" // PhaseBeforeHelm: the beforeHelm hooks of a run.
	PhaseHelm       = "helm"        // PhaseHelm: the release upgrade of a run.
	PhaseAfterHelm  = "after_helm"  // PhaseAfterHelm: the afterHelm hooks of a run.
	PhaseInitHooks  = "init_hooks"  // PhaseInitHooks: the hooks start and synchronize at startup.
	PhaseOnStartup  = "on_startup"  // PhaseOnStartup: the onStartup hooks of a startup.
)

// GlobalPackage is the package label of Global's hooks.
const GlobalPackage = "global"

// Gauges grouped by package, so a removed package drops them all.
var (
	hooklaneMetrics     = []string{HooklaneLength, HooklaneState}
	packageGroupMetrics = append([]string{HookLastRunSeconds, PackageRunLastSeconds, PackageStartupLastSeconds}, hooklaneMetrics...)
)

// PackageGauges is the part of the grouped metric storage the package gauges use.
type PackageGauges interface {
	GaugeSet(group, metric string, value float64, labels map[string]string)
	ExpireGroupMetricByName(group, name string)
}

// SetHookLastRun records the duration of a hook's last run; Global's hooks use GlobalPackage.
func SetHookLastRun(g PackageGauges, pkg, hook, binding string, took time.Duration) {
	g.GaugeSet(pkg, HookLastRunSeconds, took.Seconds(), map[string]string{
		LabelPackage: pkg, LabelHook: hook, LabelBinding: binding,
	})
}

// SetPackageRun records the duration of a phase of a package's run.
func SetPackageRun(g PackageGauges, pkg, phase string, took time.Duration) {
	g.GaugeSet(pkg, PackageRunLastSeconds, took.Seconds(), map[string]string{LabelPackage: pkg, LabelPhase: phase})
}

// SetPackageStartup records the duration of a phase of a package's startup.
func SetPackageStartup(g PackageGauges, pkg, phase string, took time.Duration) {
	g.GaugeSet(pkg, PackageStartupLastSeconds, took.Seconds(), map[string]string{LabelPackage: pkg, LabelPhase: phase})
}

// SetHooklaneLength records the number of items in a lane.
func SetHooklaneLength(g PackageGauges, pkg, lane string, length int) {
	g.GaugeSet(pkg, HooklaneLength, float64(length), map[string]string{LabelPackage: pkg, LabelLane: lane})
}

// SetHooklaneState records whether a lane is in state.
func SetHooklaneState(g PackageGauges, pkg, lane, state string, active bool) {
	value := 0.0
	if active {
		value = 1
	}

	g.GaugeSet(pkg, HooklaneState, value, map[string]string{LabelPackage: pkg, LabelLane: lane, LabelState: state})
}

// ExpireHooklane drops the lane gauges of pkg, whose lanes are gone.
func ExpireHooklane(g PackageGauges, pkg string) {
	for _, name := range hooklaneMetrics {
		g.ExpireGroupMetricByName(pkg, name)
	}
}

// ExpirePackage drops every package-grouped gauge of pkg.
func ExpirePackage(g PackageGauges, pkg string) {
	for _, name := range packageGroupMetrics {
		g.ExpireGroupMetricByName(pkg, name)
	}
}

// registerPackageMetrics registers the gauges of the package runtime with their help.
func registerPackageMetrics(metricStorage metricsstorage.Storage) error {
	gauges := []struct {
		name   string
		labels []string
		help   string
	}{
		{HookLastRunSeconds, []string{LabelPackage, LabelHook, LabelBinding}, "Duration of the last run of a hook in seconds"},
		{PackageRunLastSeconds, []string{LabelPackage, LabelPhase}, "Duration of the last successful phase (before_helm, helm, after_helm) of a package's run in seconds"},
		{PackageStartupLastSeconds, []string{LabelPackage, LabelPhase}, "Duration of the last successful phase (init_hooks, on_startup) of a package's startup in seconds"},
		{ConvergenceLastSeconds, nil, "Duration of the last convergence in seconds, a startup one included"},
		{HooklaneLength, []string{LabelPackage, LabelLane}, "Number of items in a hook lane, the one in flight included"},
		{HooklaneState, []string{LabelPackage, LabelLane, LabelState}, "State of a hook lane: 1 for the state it is in, 0 for the others"},
	}

	for _, g := range gauges {
		if _, err := metricStorage.RegisterGauge(g.name, g.labels, options.WithHelp(g.help)); err != nil {
			return fmt.Errorf("register %s: %w", g.name, err)
		}
	}

	return nil
}
