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

package licensing

import (
	"testing"
	"time"
)

// stateOf is the journal and the policy wired together the way the controller
// wires them: the limits come out of Compute, the sustained verdict out of the
// journal, and both go back into Compute.
func stateOf(j *Journal, now time.Time, records ...RecordStatus) Result {
	th := DefaultThresholds()
	keys := oneKey(records...)

	policy := Compute(keys, nil, nil, now, th)
	metrics := map[string]MetricValue{
		"vCPU": j.Stats("vCPU", now, now.Add(30*24*time.Hour), testWindow, noCeiling),
	}
	sustained := map[string]bool{}
	if limit := policy.Effective["vCPU"]; limit != nil {
		sustained["vCPU"] = j.ExceededThroughout("vCPU", now, th.SustainedWindow, *limit)
	}
	return Compute(keys, metrics, sustained, now, th)
}

// The whole point of the fix: a cluster that briefly went over its quota warns
// while it is over and goes straight back to Valid when it comes back, without
// ever passing through Violation.
func TestSpikeWarnsAndRecovers(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	live := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(8)})

	// Eight days of hourly observations of a cluster sitting exactly at its
	// quota, and one node added an hour ago.
	j := hourlyJournal(now, 8*24*time.Hour, func(at time.Time) float64 {
		if at.Equal(now) {
			return 12
		}
		return 8
	})

	if res := stateOf(j, now, live); res.State != StateWarning || res.Reason != ReasonLimitsExceeded {
		t.Fatalf("during the spike: %q/%q, want %q/%q", res.State, res.Reason, StateWarning, ReasonLimitsExceeded)
	}

	// The extra node is removed and the next observation is back at the quota.
	j.Add(Sample{At: now.Add(time.Hour), Values: map[string]float64{"vCPU": 8}}, testRetention)
	back := now.Add(time.Hour)

	res := stateOf(j, back, live)
	if res.State != StateValid {
		t.Fatalf("after the spike: %q/%q, want %q", res.State, res.Reason, StateValid)
	}
	if !res.WithinLimits {
		t.Fatal("withinLimits = false although consumption is back at the quota")
	}
	// The window still carries the spike; it must not hold the cluster in a
	// state it has left.
	if raw := j.Stats("vCPU", back, back, testWindow, noCeiling); raw.Instant != 8 {
		t.Fatalf("instant = %v, want the cluster back at its quota", raw.Instant)
	}
}

// A week above the quota is what a violation is made of, and one observation
// back inside it ends the violation on the next recomputation.
func TestSustainedExceedanceViolatesAndExits(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	live := wl(recordA, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(8)})

	j := hourlyJournal(now, 8*24*time.Hour, func(time.Time) float64 { return 12 })

	res := stateOf(j, now, live)
	if res.State != StateViolation || res.Reason != ReasonLimitsExceeded {
		t.Fatalf("state/reason = %q/%q, want %q/%q", res.State, res.Reason, StateViolation, ReasonLimitsExceeded)
	}

	j.Add(Sample{At: now.Add(time.Hour), Values: map[string]float64{"vCPU": 8}}, testRetention)
	if res := stateOf(j, now.Add(time.Hour), live); res.State == StateViolation {
		t.Fatalf("state = %q, want the violation left on the first recomputation", res.State)
	}
}
