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
	"encoding/json"
	"math/rand/v2"
	"testing"
)

// The API server returns ClusterLicense objects in no guaranteed order, and the
// result is written back to an object the customer reads. A reshuffle of the
// input must not produce a diff.
func TestComputeIsIndependentOfKeyOrder(t *testing.T) {
	now := ts("2026-05-01T00:00:00Z")
	metrics := map[string]MetricValue{"vCPU": {Instant: 80, Avg7d: 70, Extrapolated: f64(95)}}

	// Deliberately messy: a duplicate across two keys, a renewal, an expired
	// record and an unlimited grant, so every ordering-sensitive stage is hit.
	renewal := wl(recordC, "2026-06-01T00:00:00Z", "2027-06-01T00:00:00Z", map[string]*int64{"vCPU": i64(30)})
	renewal.Renews = []string{recordA}

	keys := []KeyRecords{
		{Key: "b-license", Records: []RecordStatus{wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})}},
		{Key: "a-license", Records: []RecordStatus{wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})}},
		{Key: "d-license", Records: []RecordStatus{renewal}},
		{Key: "c-license", Records: []RecordStatus{wl(recordB, "2026-02-01T00:00:00Z", "", map[string]*int64{"nodes": nil})}},
	}

	want := jsonOf(t, Compute(keys, metrics, nil, now, DefaultThresholds()))

	rng := rand.New(rand.NewPCG(1, 2))
	for i := range 50 {
		shuffled := make([]KeyRecords, len(keys))
		copy(shuffled, keys)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })

		if got := jsonOf(t, Compute(shuffled, metrics, nil, now, DefaultThresholds())); got != want {
			t.Fatalf("shuffle %d produced a different result:\n%s\nwant\n%s", i, got, want)
		}
	}
}

// jsonOf renders a Result for comparison. Maps serialize with sorted keys, so
// this compares the values and the order of every slice in the result.
func jsonOf(t *testing.T, res Result) string {
	t.Helper()
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	return string(b)
}

// R13: two successors both renewing L1. Either answer is correct, so the tie is
// broken on the successor id and stays broken the same way under any order.
func TestAmbiguousRenewalIsDeterministic(t *testing.T) {
	first := wl(recordA, "2026-03-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(30)})
	first.Renews = []string{recordC}
	second := wl(recordB, "2026-03-01T00:00:00Z", "2027-01-01T00:00:00Z", map[string]*int64{"vCPU": i64(20)})
	second.Renews = []string{recordC}
	predecessor := wl(recordC, "2026-01-01T00:00:00Z", "2026-12-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})

	now := ts("2026-05-01T00:00:00Z")
	forward := Compute(oneKey(predecessor, first, second), nil, nil, now, DefaultThresholds())
	reversed := Compute(oneKey(second, first, predecessor), nil, nil, now, DefaultThresholds())

	for _, res := range []Result{forward, reversed} {
		gone := statusOf(t, res, recordC)
		if gone.Accepted || gone.Reason != ReasonRenewed {
			t.Fatalf("the predecessor must be extinguished by both successors: %+v", gone)
		}
		// recordB sorts before recordA, so the smaller id is the one named.
		if gone.RenewedBy != recordB {
			t.Fatalf("renewedBy = %q, want the lowest successor id %q", gone.RenewedBy, recordB)
		}
		// Only the two successors contribute: 30 + 20, never the 50 of L1.
		if got := limitOf(t, res, "vCPU"); got != 50 {
			t.Fatalf("vCPU = %d, want 30+20 from the successors alone", got)
		}
	}
}

// R12: a renewal to a smaller volume. The quota really does drop, and the drop
// is announced rather than quietly applied.
func TestRenewalToSmallerVolumeReducesQuota(t *testing.T) {
	original := wl(recordA, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(50)})
	smaller := wl(recordB, "2026-07-01T00:00:00Z", "2027-07-01T00:00:00Z", map[string]*int64{"vCPU": i64(30)})
	smaller.Renews = []string{recordA}

	res := Compute(oneKey(original, smaller), nil, nil, ts("2026-05-01T00:00:00Z"), DefaultThresholds())

	// Before the successor starts, the original is untouched.
	if got := limitOf(t, res, "vCPU"); got != 50 {
		t.Fatalf("vCPU now = %d, want the original 50", got)
	}
	assertTimeline(t, res.Timeline, []string{
		"2026-01-01T00:00:00Z..2026-07-01T00:00:00Z Valid vCPU=50",
		"2026-07-01T00:00:00Z..2027-07-01T00:00:00Z Valid vCPU=30",
		"2027-07-01T00:00:00Z..2027-07-15T00:00:00Z Grace vCPU=0",
		"2027-07-15T00:00:00Z..nil Violation vCPU=0",
	})
	if res.NextReduction == nil {
		t.Fatal("nextReduction is nil, want the 50 -> 30 drop")
	}
	if !res.NextReduction.At.Equal(ts("2026-07-01T00:00:00Z")) {
		t.Fatalf("nextReduction.At = %s, want the successor start", res.NextReduction.At)
	}
	if *res.NextReduction.From["vCPU"] != 50 || *res.NextReduction.To["vCPU"] != 30 {
		t.Fatalf("nextReduction = %+v, want 50 -> 30", res.NextReduction)
	}

	// L2: the published reduction must not alias the timeline.
	*res.NextReduction.From["vCPU"] = 999
	if *res.Timeline[0].Limits["vCPU"] != 50 {
		t.Fatalf("editing the reduction changed the timeline: %v", *res.Timeline[0].Limits["vCPU"])
	}
}
