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
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// Condition types published on EffectiveLicense. UpdatesAllowed and
// RuntimeRestricted are named by the specification for the second iteration and
// are deliberately not published yet.
const (
	conditionLimitsSatisfied = "LimitsSatisfied"
	conditionKeyExpiring     = "KeyExpiring"
	conditionKeyRejected     = "KeyRejected"
	conditionSupersedeCycle  = "SupersedeCycle"
	// conditionValid lives on ClusterLicense, not on EffectiveLicense: it is a
	// verdict about one key.
	conditionValid = "Valid"
)

func setConditions(conditions *[]metav1.Condition, res licensing.Result, now time.Time) {
	set(conditions, now, conditionLimitsSatisfied, res.Allocation.WithinLimits(),
		unlicensedReason(res), unlicensedMessage(res, now),
		"every licensable node is covered by the license")

	expiry := "the license key does not expire within the warning window"
	if res.Key != nil && res.Key.ValidUntil != nil {
		expiry = fmt.Sprintf("the license expires at %s", res.Key.ValidUntil.UTC().Format(time.RFC3339))
	}
	set(conditions, now, conditionKeyExpiring, res.Expiring, "KeyExpiry",
		"the license key does not expire within the warning window", expiry)

	set(conditions, now, conditionKeyRejected, len(res.Rejected) > 0, "RecordsRejected",
		"no record and no key is rejected",
		joinOr(res.Rejected, "records are rejected"))

	set(conditions, now, conditionSupersedeCycle, len(res.SupersedeCycle) > 0, "ExtinctionCycle",
		"no record supersedes itself through a cycle",
		joinOr(res.SupersedeCycle, "records form a supersede cycle and all of them are excluded"))
}

func unlicensedReason(res licensing.Result) string {
	if res.Allocation.WithinLimits() {
		return "WithinLimits"
	}
	return "UnlicensedNodes"
}

// unlicensedMessage is the operator facing half of specification 13: how many
// nodes are uncovered, how much they weigh, since when, and how long is left.
func unlicensedMessage(res licensing.Result, now time.Time) string {
	if res.Allocation.WithinLimits() {
		return "every licensable node is covered by the license"
	}
	message := fmt.Sprintf("%d node(s) (%d vCPU) are not covered by the license",
		len(res.Allocation.Unlicensed), res.Allocation.UnlicensedVCPU)
	if res.OverLimitSince == nil {
		return message
	}
	message += " since " + res.OverLimitSince.UTC().Format(time.RFC3339)
	if left := res.OverLimitSince.Add(licensing.DefaultThresholds().OverLimitWindow).Sub(now); left > 0 {
		message += fmt.Sprintf("; in %d day(s) this becomes a violation", int64(left/(24*time.Hour))+1)
	}
	return message
}

// set publishes one condition through meta.SetStatusCondition, which keeps
// lastTransitionTime pinned to the moment the status actually flipped.
//
// The timestamp is the reconciler clock, not time.Now: a flip must be stamped
// with the instant the policy was computed at, otherwise LimitsSatisfied claims
// an exceedance began a few milliseconds after the reading that found it, and a
// test cannot pin the value at all.
func set(conditions *[]metav1.Condition, now time.Time, conditionType string, positive bool, reason, negativeMessage, positiveMessage string) {
	status := metav1.ConditionFalse
	message := negativeMessage
	if positive {
		status = metav1.ConditionTrue
		message = positiveMessage
	}
	if reason == "" {
		reason = "Unknown"
	}
	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(now),
	})
}

func joinOr(items []string, fallback string) string {
	if len(items) == 0 {
		return fallback
	}
	return strings.Join(items, "; ")
}
