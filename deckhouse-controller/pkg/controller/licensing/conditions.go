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
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// Condition types published on EffectiveLicense. Their semantics are documented
// in the CRD, next to the field they describe.
const (
	conditionLimitsSatisfied     = "LimitsSatisfied"
	conditionSomeRecordsExpiring = "SomeRecordsExpiring"
	conditionRejectedRecords     = "RejectedRecords"
	conditionRegistered          = "Registered"
	// conditionRetirable lives on ClusterLicense, not on EffectiveLicense: it is a
	// verdict about one key.
	conditionRetirable = "Retirable"
)

// benignRejections are the reasons a record may legitimately not contribute:
// none of them is a problem the customer has to act on.
var benignRejections = map[string]bool{
	licensing.ReasonUnsupportedType: true,
	licensing.ReasonRenewed:         true,
	licensing.ReasonSuperseded:      true,
	licensing.ReasonExpired:         true,
}

func setConditions(conditions *[]metav1.Condition, res licensing.Result, now time.Time) {
	registered := false
	rejected := make([]string, 0, len(res.Records))
	for _, rec := range res.Records {
		if rec.Accepted {
			if rec.Type == licensing.TypePlatform {
				registered = true
			}
			continue
		}
		if !benignRejections[rec.Reason] {
			rejected = append(rejected, fmt.Sprintf("%s (%s)", rec.ID, rec.Reason))
		}
	}
	sort.Strings(rejected)

	expiring := make([]string, 0, len(res.ExpiringSoon))
	for _, rec := range res.ExpiringSoon {
		if rec.ExpireAt == nil {
			continue
		}
		expiring = append(expiring, fmt.Sprintf("%s expires at %s", rec.ID, rec.ExpireAt.UTC().Format(time.RFC3339)))
	}
	sort.Strings(expiring)

	set(conditions, now, conditionRegistered, registered,
		"RecordAccepted", "no accepted Platform record is installed",
		"at least one Platform record is part of the policy")

	set(conditions, now, conditionLimitsSatisfied, res.WithinLimits,
		"WithinLimits", "consumption is above an effective limit",
		"consumption is within every effective limit")

	set(conditions, now, conditionSomeRecordsExpiring, len(expiring) > 0,
		"RecordsExpiring", "no active record expires within the warning window",
		joinOr(expiring, "records expire soon"))

	set(conditions, now, conditionRejectedRecords, len(rejected) > 0,
		"RecordsRejected", "no record is rejected",
		joinOr(rejected, "records are rejected"))
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
	out := items[0]
	for _, item := range items[1:] {
		out += "; " + item
	}
	return out
}
