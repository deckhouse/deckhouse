/*
Copyright 2026 Flant JSC

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

package autotune

import (
	"context"
	"fmt"
	"math"
	"time"
)

const (
	// delta = (measured − baseline) / baseline. Raising is cheap and urgent,
	// lowering is neither, hence the asymmetry.
	raiseThreshold = 0.20
	lowerThreshold = 0.30

	// Counted in runs, not in days: only the daily cron reaches the deciding path,
	// so 24h would let scheduling drift defer a change by another whole run.
	raiseCooldown = metricsRunInterval
	lowerCooldown = 3 * metricsRunInterval

	neverChanged = time.Duration(math.MaxInt64)
)

type dynamicResolver struct {
	state    autotuneState
	nodes    []Node
	fallback requestsResolver

	headroom       map[resourceKind]int64
	headroomKnown  bool
	headroomLoaded bool
}

func (r *dynamicResolver) resolve(ctx context.Context, deps resolveDeps, kind resourceKind) (resolvedRequests, error) {
	// Not getOrCreateMeasurement: the entry would leak into the ConfigMap for a
	// kind another link answers.
	measurement := r.state[kind]

	overrideRemoved := measurement != nil && measurement.AppliedOverride != nil
	if overrideRemoved || !prometheusMetricsAvailable(deps.input) {
		measurement.handOverToFallback()
		return r.fallback.resolve(ctx, deps, kind)
	}

	now := deps.dc.GetClock().Now().UTC()
	if !metricsRunDue(measurement, now) {
		return r.hold(ctx, deps, kind)
	}

	r.loadHeadroom(ctx, deps)
	if !r.headroomKnown {
		return r.hold(ctx, deps, kind)
	}

	baseline, err := r.appliedOrFallback(ctx, deps, kind)
	if err != nil {
		return resolvedRequests{}, err
	}

	usage, someFetchFailed := readComponentUsage(ctx, deps, kind)

	// Stamped on the fetch, not the commit, or a cluster whose series never appear
	// would call the metrics API on every event.
	measurement = r.state.getOrCreateMeasurement(kind)
	measurement.LastMetricsRun = now.Format(time.RFC3339)

	if len(usage) == 0 {
		if someFetchFailed && countApplied(measurement, kind) == 0 {
			return r.fallback.resolve(ctx, deps, kind)
		}
		deps.input.Logger.Warn("autotune: no usage datapoints from the metrics API, holding current requests", "resource", kind)
		baseline.deficit = r.pendingDeficit(ctx, deps, kind)
		return baseline, nil
	}

	cooldownAge := readCooldownAges(deps, kind, measurement, now)
	return r.proposeRequests(measurement, kind, usage, baseline.byComponent, cooldownAge), nil
}

// Re-reports the shortfall because the hook expires the metric group on every
// run: staying quiet about a still-blocked raise would read as "it fits now".
func (r *dynamicResolver) hold(ctx context.Context, deps resolveDeps, kind resourceKind) (resolvedRequests, error) {
	held, err := r.appliedOrFallback(ctx, deps, kind)
	if err != nil {
		return resolvedRequests{}, err
	}
	held.deficit = r.pendingDeficit(ctx, deps, kind)
	return held, nil
}

func (r *dynamicResolver) appliedOrFallback(ctx context.Context, deps resolveDeps, kind resourceKind) (resolvedRequests, error) {
	fallback, err := r.fallback.resolve(ctx, deps, kind)
	if err != nil {
		return resolvedRequests{}, err
	}

	measurement := r.state[kind]
	held := make(requestsByComponent, len(controlPlaneComponents))
	for _, comp := range controlPlaneComponents {
		if applied := appliedRequest(measurement, comp, kind); applied > 0 {
			held[comp] = applied
			continue
		}
		held[comp] = fallback.byComponent[comp]
	}

	return resolvedRequests{byComponent: held, source: sourceDynamic}, nil
}

func (r *dynamicResolver) pendingDeficit(ctx context.Context, deps resolveDeps, kind resourceKind) int64 {
	measurement := r.state[kind]
	if measurement == nil || measurement.PendingRaiseSum <= 0 {
		return 0
	}

	r.loadHeadroom(ctx, deps)
	if !r.headroomKnown {
		return 0
	}
	return r.state.refreshPendingRaiseDeficit(kind, r.headroom[kind])
}

func (r *dynamicResolver) loadHeadroom(ctx context.Context, deps resolveDeps) {
	if r.headroomLoaded {
		return
	}
	r.headroomLoaded = true

	otherRequests, err := readOtherPodRequests(ctx, deps.dc, r.nodes)
	if err != nil {
		deps.input.Logger.Error("autotune: cannot list non-control-plane pod requests on masters, skipping the metrics path", "error", err)
		setAutotuneDegraded(deps.input, autotuneDegradedMetricGroup, degradedReasonListPods)
		return
	}

	headroomMilliCPU, headroomBytes, ok := weakestMasterHeadroom(r.nodes, otherRequests)
	if !ok {
		deps.input.Logger.Error("autotune: no master nodes to compute headroom from")
		setAutotuneDegraded(deps.input, autotuneDegradedMetricGroup, degradedReasonBadNodes)
		return
	}

	r.headroom = map[resourceKind]int64{
		resourceCPU:    headroomMilliCPU,
		resourceMemory: headroomBytes,
	}
	r.headroomKnown = true
}

func (r *dynamicResolver) proposeRequests(
	measurement *autotuneMeasurementState,
	kind resourceKind,
	usage usageByComponent,
	baseline requestsByComponent,
	cooldownAge map[string]time.Duration,
) resolvedRequests {
	proposed := make(requestsByComponent, len(controlPlaneComponents))
	raising := make(map[string]bool)

	for _, comp := range controlPlaneComponents {
		proposed[comp] = baseline[comp]

		measured, hasMeasurement := usage[comp]
		if !hasMeasurement {
			continue
		}

		switch decide(measured, baseline[comp], cooldownAge[comp]) {
		case decideRaise:
			proposed[comp] = measured
			raising[comp] = true
		case decideLower:
			proposed[comp] = measured
		case decideSkip:
		}
	}

	result := resolvedRequests{byComponent: proposed, source: sourceDynamic}
	if len(raising) == 0 {
		result.deficit = r.state.refreshPendingRaiseDeficit(kind, r.headroom[kind])
		return result
	}

	result.deficit = r.gateRaises(measurement, kind, baseline, proposed, raising)
	return result
}

func (r *dynamicResolver) gateRaises(
	measurement *autotuneMeasurementState,
	kind resourceKind,
	baseline, proposed requestsByComponent,
	raising map[string]bool,
) int64 {
	headroom := r.headroom[kind]

	var proposedSum int64
	for _, comp := range controlPlaneComponents {
		proposedSum += proposed[comp]
	}

	if proposedSum <= headroom {
		measurement.PendingRaiseSum = 0
		return 0
	}

	for comp := range raising {
		proposed[comp] = baseline[comp]
	}
	measurement.PendingRaiseSum = proposedSum
	return proposedSum - headroom
}

type decideAction string

const (
	decideSkip  decideAction = "skip"
	decideRaise decideAction = "raise"
	decideLower decideAction = "lower"
)

func decide(measured, baseline int64, cooldownAge time.Duration) decideAction {
	if baseline <= 0 {
		if measured > 0 {
			return decideRaise
		}
		return decideSkip
	}

	delta := float64(measured-baseline) / float64(baseline)
	switch {
	case delta > raiseThreshold && cooldownAge >= raiseCooldown:
		return decideRaise
	case delta < -lowerThreshold && cooldownAge >= lowerCooldown:
		return decideLower
	}
	return decideSkip
}

func readCooldownAges(deps resolveDeps, kind resourceKind, measurement *autotuneMeasurementState, now time.Time) map[string]time.Duration {
	ages := make(map[string]time.Duration, len(controlPlaneComponents))
	for _, comp := range controlPlaneComponents {
		age, err := sinceLastChange(measurement.Components[comp].LastChange, now)
		if err != nil {
			deps.input.Logger.Warn("autotune: untrustworthy lastChange, keeping the component on cooldown",
				"resource", kind, "component", comp, "error", err)
			setAutotuneDegraded(deps.input, autotuneDegradedMetricGroup, degradedReasonBadState)
		}
		ages[comp] = age
	}
	return ages
}

// Fails closed: an unparsable timestamp, or one ahead of now because a clock
// jumped, gives an age of zero and keeps the component on cooldown. The opposite
// would quietly turn anti-flap off.
func sinceLastChange(raw string, now time.Time) (time.Duration, error) {
	if raw == "" {
		return neverChanged, nil
	}

	changed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return 0, fmt.Errorf("parse lastChange %q: %w", raw, err)
	}
	if changed.After(now) {
		return 0, fmt.Errorf("lastChange %q is ahead of now %q", raw, now.Format(time.RFC3339))
	}
	return now.Sub(changed), nil
}
