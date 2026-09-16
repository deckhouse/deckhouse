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
	"testing"
	"time"

	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

func validResult() licensing.Result {
	limit := int64(50)
	res := licensing.Result{
		State:        licensing.StateValid,
		Effective:    map[string]*int64{"vCPU": &limit},
		WithinLimits: true,
		Records: []licensing.RecordStatus{{
			Record: licensing.Record{
				Type:    licensing.TypeWorkload,
				ID:      testRecordID,
				StartAt: testNow.Add(-24 * time.Hour),
			},
			Accepted: true,
		}},
	}
	res.Counts.Packages = 1
	res.Counts.Records = 1
	res.Counts.Accepted = 1
	return res
}

// Recomputing an unchanged policy must produce a byte identical status, so the
// reconcile loop does not write on every tick.
func TestEffectiveStatusIsStable(t *testing.T) {
	res := validResult()
	values := map[string]licensing.MetricValue{"vCPU": {Instant: 4, Avg7d: 4, Extrapolated: 4}}

	first := effectiveStatus(v1alpha1.EffectiveLicenseStatus{}, res, values, "token", testNow)
	second := effectiveStatus(first, res, values, "token", testNow.Add(9*time.Minute))

	if !equality.Semantic.DeepEqual(first, second) {
		t.Fatalf("status changed without a policy change:\nfirst  = %+v\nsecond = %+v", first, second)
	}
}

// The consumption views are a least squares fit, so a window that did not change
// still moves in the last bits of the mantissa from one reconcile to the next.
// Published raw, that noise would rewrite the status forever.
func TestEffectiveStatusSurvivesFloatNoise(t *testing.T) {
	journal := new(licensing.Journal)
	journal.Add(licensing.Sample{At: testNow.Add(-70 * time.Minute), Values: map[string]float64{metricVCPU: 4}}, retention)
	journal.Add(licensing.Sample{At: testNow.Add(-10 * time.Minute), Values: map[string]float64{metricVCPU: 4}}, retention)

	// A fixed point in the future, the way a record expiry is fixed.
	horizon := testNow.Add(240 * 24 * time.Hour)
	later := testNow.Add(10 * time.Minute)

	raw := journal.Stats(metricVCPU, testNow, horizon, retention)
	if raw == journal.Stats(metricVCPU, later, horizon, retention) {
		t.Log("the raw fit happened to be exact here; the rounding still has to hold")
	}

	res := validResult()
	first := effectiveStatus(v1alpha1.EffectiveLicenseStatus{}, res,
		map[string]licensing.MetricValue{metricVCPU: stats(journal, metricVCPU, testNow, horizon)}, "token", testNow)
	second := effectiveStatus(first, res,
		map[string]licensing.MetricValue{metricVCPU: stats(journal, metricVCPU, later, horizon)}, "token", later)

	if !equality.Semantic.DeepEqual(first, second) {
		t.Fatalf("status changed on float noise alone:\nfirst  = %+v\nsecond = %+v", first.Metrics, second.Metrics)
	}
}

// The moment a state was entered survives recomputation, and is reset when the
// state or its reason changes.
func TestComplianceSince(t *testing.T) {
	values := map[string]licensing.MetricValue{"vCPU": {Instant: 4, Avg7d: 4, Extrapolated: 4}}
	first := effectiveStatus(v1alpha1.EffectiveLicenseStatus{}, validResult(), values, "token", testNow)

	later := testNow.Add(3 * time.Hour)

	same := effectiveStatus(first, validResult(), values, "token", later)
	if same.Compliance.Since == nil || !same.Compliance.Since.Equal(first.Compliance.Since) {
		t.Fatalf("since = %v, want the original %v", same.Compliance.Since, first.Compliance.Since)
	}

	changed := validResult()
	changed.State = licensing.StateWarning
	changed.Reason = licensing.ReasonLimitsExceeded

	moved := effectiveStatus(first, changed, values, "token", later)
	if moved.Compliance.Since == nil || !moved.Compliance.Since.Equal(&metav1.Time{Time: later}) {
		t.Fatalf("since = %v, want %v", moved.Compliance.Since, later)
	}
}

// A record that stopped contributing keeps the condition it caused only while it
// is actually rejected.
func TestConditionsFollowRecords(t *testing.T) {
	res := validResult()
	res.Records = append(res.Records, licensing.RecordStatus{
		Record:   licensing.Record{Type: licensing.TypeWorkload, ID: testPackageID},
		Accepted: false,
		Reason:   licensing.ReasonClusterMismatch,
	})

	var conditions []metav1.Condition
	setConditions(&conditions, res, testNow)

	if !conditionTrue(conditions, conditionRejectedRecords) {
		t.Fatalf("condition %s is not True: %+v", conditionRejectedRecords, conditions)
	}

	// A record ignored because this build does not know its type is not a
	// problem the customer has to act on.
	benign := validResult()
	benign.Records = append(benign.Records, licensing.RecordStatus{
		Record:   licensing.Record{Type: "Support", ID: testPackageID},
		Accepted: false,
		Reason:   licensing.ReasonUnsupportedType,
	})

	conditions = nil
	setConditions(&conditions, benign, testNow)
	if conditionTrue(conditions, conditionRejectedRecords) {
		t.Fatalf("condition %s is True for an unsupported record type", conditionRejectedRecords)
	}
}

func conditionTrue(conditions []metav1.Condition, conditionType string) bool {
	for _, c := range conditions {
		if c.Type == conditionType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}

// A condition is stamped with the reconciler clock, and the stamp stands still
// while the condition does: LimitsSatisfied's lastTransitionTime is the moment
// the exceedance began, not the moment it was last noticed.
func TestConditionTransitionTimeMarksTheFlip(t *testing.T) {
	exceeded := validResult()
	exceeded.WithinLimits = false

	var conditions []metav1.Condition
	setConditions(&conditions, exceeded, testNow)

	began := meta.FindStatusCondition(conditions, conditionLimitsSatisfied).LastTransitionTime
	if !began.Equal(&metav1.Time{Time: testNow}) {
		t.Fatalf("lastTransitionTime = %v, want the reconciler clock %v", began, testNow)
	}

	// Noticed again an hour later, still exceeded.
	setConditions(&conditions, exceeded, testNow.Add(time.Hour))
	if got := meta.FindStatusCondition(conditions, conditionLimitsSatisfied).LastTransitionTime; !got.Equal(&began) {
		t.Fatalf("lastTransitionTime = %v, want the unchanged %v", got, began)
	}

	// Back within the limits two hours in: that is a new transition.
	recovered := testNow.Add(2 * time.Hour)
	setConditions(&conditions, validResult(), recovered)
	if got := meta.FindStatusCondition(conditions, conditionLimitsSatisfied).LastTransitionTime; !got.Equal(&metav1.Time{Time: recovered}) {
		t.Fatalf("lastTransitionTime = %v, want %v", got, recovered)
	}
}
