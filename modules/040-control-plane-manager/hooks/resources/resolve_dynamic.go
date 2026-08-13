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

package resources

import (
	"context"
	"time"
)

const (
	// Asymmetric deadband for raise/lower. Go constants, not config-values.
	// Change is significant only when delta > raiseThreshold or
	// delta < -lowerThreshold (delta = (measured - baseline) / baseline).
	raiseThreshold = 0.20 // +20%
	lowerThreshold = 0.30 // −30%

	// Anti-flap cooldowns.
	raiseCooldown = 24 * time.Hour
	lowerCooldown = 72 * time.Hour
)

// dynamicResolver answers from measured usage (custom.metrics.k8s.io): per
// component, under the asymmetric deadband and the raise/lower cooldowns, and
// only if the raises collectively fit the headroom left on the weakest master.
//
// It is deliberately stateful. It reads autotuneState for the baseline and the
// cooldown timestamps, and it writes back the two fields that are its own
// bookkeeping rather than an answer: LastMetricsRun, which keeps event-driven
// runs out of the daily window, and PendingRaiseSum, a raise the headroom gate
// blocked. It never writes applied values — those come back in the result, for
// the caller to commit.
type dynamicResolver struct {
	state    autotuneState
	nodes    []Node
	fallback requestsResolver

	// Headroom costs a pod list per master, so it is loaded on first use rather
	// than up front: with prometheus disabled, or outside the daily window, this
	// link never needs it. resolve is called once per kind and both kinds share
	// one pod list. No sync.Once — the hook resolves kinds sequentially.
	headroom       map[resourceKind]int64
	headroomKnown  bool
	headroomLoaded bool
}

func (r *dynamicResolver) resolve(ctx context.Context, rctx resolveContext, kind resourceKind) (resolvedRequests, error) {
	// Read-only until this link actually owns the answer: creating the measurement
	// up front would leave an empty entry in the state — and so in the ConfigMap —
	// for a kind that ends up resolved by another link, or not resolved at all.
	measurement := r.state[kind]

	// An override was in force on the previous run and is gone now: rebase on the
	// static split right away, instead of leaving the hand-set value to linger
	// inside the deadband until some later run drifts off it.
	overrideRemoved := measurement != nil && measurement.AppliedOverride != nil

	if overrideRemoved || !prometheusMetricsAvailable(rctx.input) {
		if measurement != nil {
			measurement.AppliedOverride = nil
			// PendingRaiseSum is this link's own bookkeeping and means nothing while
			// the answer comes from somewhere else.
			measurement.PendingRaiseSum = 0
		}
		return r.fallback.resolve(ctx, rctx, kind)
	}

	now := rctx.dc.GetClock().Now().UTC()
	if !metricsRunDue(measurement, now) {
		return r.hold(ctx, rctx, kind)
	}

	r.loadHeadroom(ctx, rctx)
	if !r.headroomKnown {
		// Without headroom a raise cannot be gated, and committing one blindly is
		// how a master ends up unschedulable.
		return r.hold(ctx, rctx, kind)
	}

	baseline, err := r.hold(ctx, rctx, kind)
	if err != nil {
		return resolvedRequests{}, err
	}

	usage, noFetchErrors := readComponentUsage(ctx, rctx, kind)

	// From here on this link owns the run, so its own bookkeeping gets written.
	// LastMetricsRun is stamped on the fact of the fetch, not on committing
	// anything: otherwise a cluster whose series never appear would go to the
	// metrics API on every event. Safe to create the measurement now — baseline
	// resolved, so the fallback below cannot fail either.
	measurement = r.state.getOrCreateMeasurement(kind)
	measurement.LastMetricsRun = now.Format(time.RFC3339)

	if len(usage) == 0 {
		if !noFetchErrors && countApplied(measurement, kind) == 0 {
			// Cold start: nothing measured, and nothing ever applied, so there is no
			// baseline of our own to hold on to. Let the static split bootstrap it.
			return r.fallback.resolve(ctx, rctx, kind)
		}
		rctx.input.Logger.Warn("autotune: no usage datapoints from the metrics API, holding current requests", "resource", kind)
		baseline.deficit = r.state.refreshPendingRaiseDeficit(kind, r.headroom[kind])
		return baseline, nil
	}

	return r.decideRequests(measurement, kind, usage, baseline.byComponent, now), nil
}

// hold is the answer when this link has nothing fresh to act on: the values
// already applied, with any component that has none filled in from the next
// link. Filling in is what keeps a zero out of the result — a component can be
// missing from the state either because it was just added, or because an earlier
// run resolved the kind somewhere else entirely.
func (r *dynamicResolver) hold(ctx context.Context, rctx resolveContext, kind resourceKind) (resolvedRequests, error) {
	fallback, err := r.fallback.resolve(ctx, rctx, kind)
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

// loadHeadroom computes the free capacity on the weakest master, once per hook
// run.
//
// A failure is not fatal: the deadband and the cooldowns keep holding the
// current requests, only raise/lower stops for the day. So it degrades instead
// of returning an error — and reports that itself, because a graceful
// degradation is invisible to the caller by definition.
func (r *dynamicResolver) loadHeadroom(ctx context.Context, rctx resolveContext) {
	if r.headroomLoaded {
		return
	}
	r.headroomLoaded = true

	otherByNode, err := fetchOtherRequestsByMasterNodes(ctx, rctx.dc, r.nodes)
	if err != nil {
		rctx.input.Logger.Error("autotune: cannot list non-control-plane pod requests on masters, skipping the metrics path", "error", err)
		setAutotuneDegraded(rctx.input, autotuneDegradedMetricGroup, degradedReasonListPods)
		return
	}

	headroomMilliCPU, headroomBytes, ok := minMasterFitBudget(r.nodes, otherByNode)
	if !ok {
		rctx.input.Logger.Error("autotune: no master nodes to compute headroom from")
		setAutotuneDegraded(rctx.input, autotuneDegradedMetricGroup, degradedReasonBadNodes)
		return
	}

	r.headroom = map[resourceKind]int64{
		resourceCPU:    headroomMilliCPU,
		resourceMemory: headroomBytes,
	}
	r.headroomKnown = true
}

// decideRequests moves every component with a fresh measurement under the
// deadband and cooldown rules, leaves the rest on their baseline, then gates the
// raises against headroom.
func (r *dynamicResolver) decideRequests(
	measurement *autotuneMeasurementState,
	kind resourceKind,
	usage usageByComponent,
	baseline requestsByComponent,
	now time.Time,
) resolvedRequests {
	proposed := make(requestsByComponent, len(controlPlaneComponents))
	raising := make(map[string]bool)

	for _, comp := range controlPlaneComponents {
		proposed[comp] = baseline[comp]

		measured, hasMeasurement := usage[comp]
		if !hasMeasurement {
			continue
		}

		lastChange := parseLastChange(measurement.Components[comp].LastChange)
		switch decide(measured, baseline[comp], lastChange, now) {
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
		// Nothing raising this run, but a raise blocked earlier may fit now that
		// headroom has been recomputed, so recheck rather than assume it is stuck.
		result.deficit = r.state.refreshPendingRaiseDeficit(kind, r.headroom[kind])
		return result
	}

	result.deficit = r.gateRaises(measurement, kind, baseline, proposed, raising)
	return result
}

// gateRaises enforces that the proposed requests as a whole still fit the
// headroom. If they do, any earlier shortfall clears. If they do not, every
// raising component goes back to its baseline and the shortfall is returned, so
// that the raise gets reported instead of half-applied.
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

// decide is the deadband-and-cooldown rule for one component: raising is cheap
// and urgent, lowering is neither, hence the asymmetry in both the thresholds
// and the cooldowns.
func decide(measured, baseline int64, lastChange, now time.Time) decideAction {
	if baseline <= 0 {
		if measured > 0 {
			return decideRaise
		}
		return decideSkip
	}
	delta := float64(measured-baseline) / float64(baseline)
	switch {
	case delta > raiseThreshold:
		if now.Sub(lastChange) >= raiseCooldown || lastChange.IsZero() {
			return decideRaise
		}
	case delta < -lowerThreshold:
		if now.Sub(lastChange) >= lowerCooldown || lastChange.IsZero() {
			return decideLower
		}
	}
	return decideSkip
}

// parseLastChange treats "never recorded" and "unparsable" the same way: no
// cooldown left to honor.
func parseLastChange(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// appliedRequest is appliedValue over a measurement that may never have been
// written at all.
func appliedRequest(measurement *autotuneMeasurementState, comp string, kind resourceKind) int64 {
	if measurement == nil {
		return 0
	}
	return appliedValue(measurement.Components[comp], kind)
}
