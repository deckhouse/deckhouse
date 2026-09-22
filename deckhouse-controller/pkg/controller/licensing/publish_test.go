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
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/metrics"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// validResult is the policy of a healthy single key cluster.
func validResult() licensing.Result {
	res := licensing.Result{
		State:    licensing.StateValid,
		Licensed: true,
		Limits: licensing.Limits{Speaking: true, Values: map[string]*int64{
			licensing.MetricServers: ptrTo(int64(10)),
			licensing.MetricVCPU:    ptrTo(int64(100)),
			licensing.MetricCores:   ptrTo(int64(0)),
		}},
		Consumption: map[string]int64{
			licensing.MetricServers: 1, licensing.MetricVCPU: 4, licensing.MetricCores: 2,
		},
		Allocation: licensing.Allocation{
			Servers: licensing.MetricAllocation{
				Nodes: []string{"worker"}, Limit: ptrTo(int64(10)), Used: 1,
			},
			VCPU:  licensing.MetricAllocation{Limit: ptrTo(int64(100))},
			Cores: licensing.MetricAllocation{Limit: ptrTo(int64(0))},
		},
		Records: []licensing.RecordStatus{{
			Record: licensing.Record{
				Type:    licensing.TypePlatform,
				ID:      testRecordID,
				StartAt: testNow.Add(-24 * time.Hour),
			},
			Accepted: true,
		}},
	}
	res.Counts.Keys = 1
	res.Counts.Records = 1
	res.Counts.Accepted = 1
	return res
}

func ptrTo[T any](v T) *T { return &v }

// An unlimited metric is exported as +Inf: a comparison against it is silent,
// and exporting 0 for it would fire every consumption alert on the customers who
// bought the most.
func TestPublishMetricsExportsUnlimitedAsInfinity(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	res := validResult()
	res.Limits.Values[licensing.MetricVCPU] = nil

	env.r.publishMetrics(res, nil, testNow)

	for _, sample := range env.series(t, metrics.D8LicenseLimit) {
		value := sample.GetGauge().GetValue()
		switch labelOf(sample, metrics.LabelResource) {
		case licensing.MetricVCPU:
			if !math.IsInf(value, 1) {
				t.Fatalf("unlimited vCPU = %v, want +Inf", value)
			}
		case licensing.MetricServers:
			if value != 10 {
				t.Fatalf("servers limit = %v, want 10", value)
			}
		}
	}
}

// The consumption of every metric is exported once, with no kind label: there is
// no moving average and no projection any more.
func TestPublishMetricsExportsTheThreeMetrics(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})
	env.r.publishMetrics(validResult(), nil, testNow)

	got := map[string]float64{}
	for _, sample := range env.series(t, metrics.D8LicenseConsumption) {
		got[labelOf(sample, metrics.LabelResource)] = sample.GetGauge().GetValue()
	}
	want := map[string]float64{licensing.MetricServers: 1, licensing.MetricVCPU: 4, licensing.MetricCores: 2}
	for name, value := range want {
		if got[name] != value {
			t.Fatalf("consumption = %v, want %v", got, want)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("consumption has %d series, want %d", len(got), len(want))
	}
}

// The over-limit clock is exported even while it is zero: an alert needs to be
// able to tell "covered" from "no data".
func TestPublishMetricsExportsTheOverLimitClock(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	res := validResult()
	since := testNow.Add(-48 * time.Hour)
	res.OverLimitSince = &since
	res.Allocation.VCPU.Nodes = []string{"mid"}
	res.Allocation.Cores.Nodes = []string{"other", "another"}
	res.Allocation.Unlicensed = []string{"small"}
	res.Allocation.UnlicensedVCPU = 8

	env.r.publishMetrics(res, nil, testNow)

	over := env.series(t, metrics.D8LicenseOverLimitSeconds)
	if len(over) != 1 || over[0].GetGauge().GetValue() != 48*3600 {
		t.Fatalf("over limit seconds = %+v, want 172800", over)
	}
	vcpu := env.series(t, metrics.D8LicenseUnlicensedVCPU)
	if len(vcpu) != 1 || vcpu[0].GetGauge().GetValue() != 8 {
		t.Fatalf("unlicensed vCPU = %+v, want 8", vcpu)
	}

	nodes := map[string]float64{}
	for _, sample := range env.series(t, metrics.D8LicenseNodes) {
		nodes[labelOf(sample, metrics.LabelBilling)] = sample.GetGauge().GetValue()
	}
	want := map[string]float64{
		licensing.BillingServer:     1,
		licensing.BillingVCPU:       1,
		licensing.BillingCores:      2,
		licensing.BillingUnlicensed: 1,
	}
	if !reflect.DeepEqual(nodes, want) {
		t.Fatalf("nodes by billing = %v, want %v", nodes, want)
	}
}

// The compliance state carries the reason it is in, so that an alert can exclude
// a state that is expected of a cluster nobody has registered yet.
func TestPublishMetricsLabelsComplianceStateWithItsReason(t *testing.T) {
	env := newTestEnv(t, ed25519.PublicKey{})

	res := validResult()
	res.State = licensing.StateViolation
	res.Reason = licensing.ReasonUnregistered

	env.r.publishMetrics(res, nil, testNow)

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

	env.r.publishMetrics(res, nil, testNow)

	if got := env.series(t, metrics.D8LicenseComplianceState); len(got) != 0 {
		t.Fatalf("compliance state has %d series, want none", len(got))
	}
}
