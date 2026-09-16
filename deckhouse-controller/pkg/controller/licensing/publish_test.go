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

package licensing

import (
	"crypto/ed25519"
	"testing"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// An unlimited resource must export no limit at all. Exporting 0 for it would
// fire every consumption alert on the customers who bought the most.
func TestPublishMetricsOmitsUnlimitedResources(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	limit := int64(50)
	res := validResult()
	res.Effective = map[string]*int64{metricVCPU: &limit, metricNodes: nil}
	res.Unlimited = true

	env.r.publishMetrics(res, map[string]licensing.MetricValue{
		metricVCPU: {Instant: 4, Avg7d: 4, Extrapolated: 4},
	}, nil, testNow)

	limits := env.series(t, metrics.D8LicenseEffectiveLimit)
	if len(limits) != 1 {
		t.Fatalf("effective limit has %d series, want only the finite resource", len(limits))
	}
	if got := labelOf(limits[0], metrics.LabelResource); got != metricVCPU {
		t.Fatalf("effective limit is exported for %q, want %q", got, metricVCPU)
	}
	if got := limits[0].GetGauge().GetValue(); got != 50 {
		t.Fatalf("effective limit = %v, want 50", got)
	}
}

// The compliance state carries the reason it is in, so that an alert can exclude
// a state that is expected of a cluster nobody has registered yet.
func TestPublishMetricsLabelsComplianceStateWithItsReason(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	res := validResult()
	res.State = licensing.StateViolation
	res.Reason = licensing.ReasonUnregistered

	env.r.publishMetrics(res, nil, nil, testNow)

	state := env.series(t, metrics.D8LicenseComplianceState)
	if len(state) != 1 {
		t.Fatalf("compliance state has %d series, want 1", len(state))
	}
	if got := state[0].GetGauge().GetValue(); got != 3 {
		t.Fatalf("compliance state = %v, want 3", got)
	}
	if got := labelOf(state[0], metrics.LabelReason); got != licensing.ReasonUnregistered {
		t.Fatalf("reason label = %q, want %q", got, licensing.ReasonUnregistered)
	}
}

// A state this build has no code for is not published at all: a 0 would read as
// Valid on every dashboard.
func TestPublishMetricsSkipsUnknownState(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	res := validResult()
	res.State = "SomethingNewerThanThisBuild"

	env.r.publishMetrics(res, nil, nil, testNow)

	if got := env.series(t, metrics.D8LicenseComplianceState); len(got) != 0 {
		t.Fatalf("compliance state has %d series, want none", len(got))
	}
}
