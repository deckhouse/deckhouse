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

package quota

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha1"
)

const (
	// StatusWriteInterval is the least time between two writes of the status that only move the usage.
	StatusWriteInterval = 10 * time.Second
	// usageStep is the share of a limit the usage has to move by for such a write.
	usageStep = 0.01
	// rolloutNamespacesShown is how many namespaces a KeyRollout message names.
	rolloutNamespacesShown = 10
)

// Namespace is the d8-project-quota ResourceQuota of one namespace of the project, as the controller
// found it.
type Namespace struct {
	Name string
	// Used is status.used of the ResourceQuota.
	Used corev1.ResourceList
	// Calculated is true when status.hard of the ResourceQuota carries every limit of the project:
	// the resource quota controller has counted it, and the quota plugin applies every key.
	Calculated bool
	// Labelled is true when the namespace carries the enforced label.
	Labelled bool
	// Problem is why the controller could not set the quota up in the namespace, empty when it could.
	Problem string
}

// AllCalculated reports whether every namespace has its quota calculated.
func AllCalculated(namespaces []Namespace) bool {
	for _, ns := range namespaces {
		if !ns.Calculated {
			return false
		}
	}
	return true
}

// Calculated reports whether the resource quota controller has counted the quota for every limit of
// hard: status.hard has each key with the same value. A quota without limits, as when every key of
// the project is one that no ResourceQuota counts, is calculated once it exists: the controller never
// writes status.hard for it.
func Calculated(rq *corev1.ResourceQuota, hard corev1.ResourceList) bool {
	if rq == nil {
		return false
	}
	for key, value := range hard {
		got, ok := rq.Status.Hard[key]
		if !ok || got.Cmp(value) != 0 {
			return false
		}
	}
	return true
}

// Sum adds the usage of the namespaces up for every key of hard; a key nothing uses is zero.
func Sum(hard corev1.ResourceList, namespaces []Namespace) corev1.ResourceList {
	used := make(corev1.ResourceList, len(hard))
	for key, limit := range hard {
		total := resource.NewQuantity(0, limit.Format)
		for _, ns := range namespaces {
			if value, ok := ns.Used[key]; ok {
				total.Add(value)
			}
		}
		used[key] = *total
	}
	return used
}

// Top returns at most n namespaces, the ones closest to a limit first: by the largest share of a limit
// they use, then by name.
func Top(hard corev1.ResourceList, namespaces []Namespace, n int) []v1alpha1.ProjectQuotaNamespaceUsage {
	type ranked struct {
		ns    Namespace
		share float64
	}
	all := make([]ranked, 0, len(namespaces))
	for _, ns := range namespaces {
		share := 0.0
		for key, limit := range hard {
			if value, ok := ns.Used[key]; ok {
				share = math.Max(share, ratio(value, limit))
			}
		}
		all = append(all, ranked{ns: ns, share: share})
	}
	slices.SortFunc(all, func(a, b ranked) int {
		if c := cmp.Compare(b.share, a.share); c != 0 {
			return c
		}
		return cmp.Compare(a.ns.Name, b.ns.Name)
	})

	out := make([]v1alpha1.ProjectQuotaNamespaceUsage, 0, min(n, len(all)))
	for _, r := range all[:min(n, len(all))] {
		used := make(corev1.ResourceList, len(hard))
		for key := range hard {
			if value, ok := r.ns.Used[key]; ok {
				used[key] = value.DeepCopy()
			}
		}
		out = append(out, v1alpha1.ProjectQuotaNamespaceUsage{Name: r.ns.Name, Used: used})
	}
	return out
}

// ratio is used/limit; a zero limit makes any usage infinite.
func ratio(used, limit resource.Quantity) float64 {
	l := limit.AsApproximateFloat64()
	u := used.AsApproximateFloat64()
	if l <= 0 {
		if u > 0 {
			return math.Inf(1)
		}
		return 0
	}
	return u / l
}

// ExceededKeys returns, sorted, the keys whose usage is above the limit.
func ExceededKeys(hard, used corev1.ResourceList) []corev1.ResourceName {
	var keys []corev1.ResourceName
	for key, limit := range hard {
		if value, ok := used[key]; ok && value.Cmp(limit) > 0 {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

// ExceededCondition evaluates the Exceeded condition of the quota of the generation. previous is the
// condition the status had, nil when it had none; previousHard is status.hard of the previous status.
// The reason says how the usage got above the limit: HardLowered when the limit was set below the
// usage, by a change or when the quota was created, Overshoot when the usage grew past the limit that
// the previous evaluation of the same generation found it within. A reason sticks while the usage
// stays above the limit; a later cut of the limit does not turn an overshoot into HardLowered.
func ExceededCondition(previous *metav1.Condition, previousHard, hard, used corev1.ResourceList, generation int64, cause string) metav1.Condition {
	keys := ExceededKeys(hard, used)
	if len(keys) == 0 {
		return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionFalse, Reason: v1alpha1.ProjectQuotaReasonWithinLimits,
			Message: "The usage of the project is within the quota."}
	}

	parts := make([]string, 0, len(keys))
	lowered := false
	for _, key := range keys {
		limit, value := hard[key], used[key]
		parts = append(parts, fmt.Sprintf("%s: used %s, hard %s", key, value.String(), limit.String()))
		if before, ok := previousHard[key]; ok && limit.Cmp(before) < 0 {
			lowered = true
		}
	}
	message := "The usage of the project is above the quota: " + strings.Join(parts, "; ") +
		". Running objects are not touched; creating more of these resources is denied until the usage is back within the quota."

	reason := v1alpha1.ProjectQuotaReasonOvershoot
	switch {
	case previous != nil && previous.Status == metav1.ConditionTrue && previous.Reason == v1alpha1.ProjectQuotaReasonOvershoot:
		// the cause found when the overshoot came stays with it
		if lowered {
			message += " The limit was lowered since."
		}
		return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionTrue, Reason: reason,
			Message: overshootMessage(message, previous.Message)}
	case lowered:
		reason = v1alpha1.ProjectQuotaReasonHardLowered
	case previous != nil && previous.Status == metav1.ConditionTrue:
		reason = previous.Reason
	case previous == nil || previous.ObservedGeneration != generation:
		// the usage was found above a limit that is new to the evaluation
		reason = v1alpha1.ProjectQuotaReasonHardLowered
	}
	if reason == v1alpha1.ProjectQuotaReasonOvershoot && cause != "" {
		message += " Probable cause: " + cause + "."
	}
	return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionExceeded, Status: metav1.ConditionTrue, Reason: reason, Message: message}
}

// overshootMessage keeps the probable cause of the previous message on a new one.
func overshootMessage(message, previous string) string {
	if _, cause, ok := strings.Cut(previous, " Probable cause: "); ok {
		return message + " Probable cause: " + cause
	}
	return message
}

// EnforcedInput is what the Enforced condition depends on.
type EnforcedInput struct {
	Config       Config
	ProjectFound bool
	NotServed    []string
	// NotCounted are keys of a family the ledger does not count, as its module is disabled.
	NotCounted []string
	Namespaces []Namespace
}

// EnforcedCondition evaluates the Enforced condition: True when the quota applies in every namespace
// of the project, and otherwise the first reason it does not.
func EnforcedCondition(in EnforcedInput) metav1.Condition {
	condition := func(reason, message string) metav1.Condition {
		return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionEnforced, Status: metav1.ConditionFalse, Reason: reason, Message: message}
	}
	switch {
	case !in.Config.ControlPlaneManaged:
		return condition(v1alpha1.ProjectQuotaReasonControlPlaneNotManaged,
			"The control plane of the cluster is not managed by Deckhouse, so the sum over the namespaces of a project is not enforced. Every namespace is still limited by the quota on its own.")
	case in.Config.Suspended:
		return condition(v1alpha1.ProjectQuotaReasonSuspended,
			"The quota-ledger-suspend ConfigMap in d8-multitenancy-manager suspends the ledger: the sum over the namespaces is not checked. Delete the ConfigMap to resume.")
	case in.Config.WebhookMissing:
		return condition(v1alpha1.ProjectQuotaReasonWebhookMissing,
			"The d8-multitenancy-manager-quota-ledger webhook is missing or does not trust the certificate of the ledger, so the sum over the namespaces is not checked.")
	case in.Config.WebhookMisconfigured != "":
		return condition(v1alpha1.ProjectQuotaReasonWebhookMisconfigured,
			"The "+in.Config.WebhookMisconfigured+" of the d8-multitenancy-manager-quota-ledger webhook differs from what the module renders, "+
				"so writes of the d8-project-quota ResourceQuota do not all reach the ledger and the sum over the namespaces is not checked for them. "+
				"Deckhouse restores the webhook on the next run of the module.")
	case len(in.Config.WebhookNotCalledBy) > 0:
		return condition(v1alpha1.ProjectQuotaReasonWebhookNotCalled,
			"The API servers "+strings.Join(in.Config.WebhookNotCalledBy, ", ")+" admit objects without calling the d8-multitenancy-manager-quota-ledger webhook, "+
				"so the sum over the namespaces is not checked for what they admit. Check the rules, namespaceSelector and matchConditions of the webhook.")
	case !in.ProjectFound:
		return condition(v1alpha1.ProjectQuotaReasonProjectNotFound, "There is no project of this name; the quota applies once the project is created.")
	case len(in.NotServed) > 0:
		return condition(v1alpha1.ProjectQuotaReasonKeyNotServed,
			"Nothing counts these keys because the API server does not serve their resources yet: "+strings.Join(in.NotServed, ", ")+".")
	case len(in.NotCounted) > 0:
		return condition(v1alpha1.ProjectQuotaReasonKeyNotServed,
			"Nothing counts these keys because the module that serves their resources is disabled: "+strings.Join(in.NotCounted, ", ")+".")
	}

	var pending, problems []string
	for _, ns := range in.Namespaces {
		if !ns.Calculated || !ns.Labelled {
			pending = append(pending, ns.Name)
		}
		if ns.Problem != "" {
			problems = append(problems, ns.Name+": "+ns.Problem)
		}
	}
	if len(pending) > 0 {
		slices.Sort(pending)
		slices.Sort(problems)
		shown := pending[:min(rolloutNamespacesShown, len(pending))]
		message := "The quota is not applied yet in " + strings.Join(shown, ", ")
		if len(pending) > len(shown) {
			message += fmt.Sprintf(" and %d more namespaces", len(pending)-len(shown))
		}
		message += ": their d8-project-quota ResourceQuota is not there or not calculated with every limit of the quota yet."
		if len(problems) > 0 {
			message += " The controller fails to set it up in " + strings.Join(problems[:min(rolloutNamespacesShown, len(problems))], "; ") + "."
		}
		return condition(v1alpha1.ProjectQuotaReasonKeyRollout, message)
	}
	return metav1.Condition{Type: v1alpha1.ProjectQuotaConditionEnforced, Status: metav1.ConditionTrue, Reason: v1alpha1.ProjectQuotaReasonEnforced,
		Message: "The quota applies in every namespace of the project."}
}

// WriteDue tells whether the status should be written now and, when only the usage moved too little
// or too soon, after how long. A change of the limits, of the conditions or of the number of
// namespaces is written at once; a move of the usage is written once it reaches 1% of a limit and at
// most every StatusWriteInterval. The usage in the message of the Exceeded condition is not a change of
// the condition while the same keys stay exceeded: it moves with every object of the project, and the
// usage it shows is written as the usage is.
func WriteDue(current, desired *v1alpha1.ProjectQuotaStatus, lastWrite, now time.Time) (bool, time.Duration) {
	sameExceeded := slices.Equal(ExceededKeys(current.Hard, current.Used), ExceededKeys(desired.Hard, desired.Used))
	if current.ObservedGeneration != desired.ObservedGeneration ||
		current.NamespaceCount != desired.NamespaceCount ||
		!equality.Semantic.DeepEqual(current.Hard, desired.Hard) ||
		!sameConditions(current.Conditions, desired.Conditions, sameExceeded) {
		return true, 0
	}
	if !usageMoved(desired.Hard, current.Used, desired.Used) {
		return false, 0
	}
	if wait := StatusWriteInterval - now.Sub(lastWrite); wait > 0 {
		return false, wait
	}
	return true, 0
}

// usageMoved reports whether the usage of some key moved by at least usageStep of its limit, or moved
// at all on a zero limit.
func usageMoved(hard, before, after corev1.ResourceList) bool {
	for key, limit := range hard {
		b, a := before[key], after[key]
		if b.Cmp(a) == 0 {
			continue
		}
		l := limit.AsApproximateFloat64()
		if l <= 0 || math.Abs(a.AsApproximateFloat64()-b.AsApproximateFloat64()) >= usageStep*l {
			return true
		}
	}
	return false
}

// sameConditions compares the conditions without their timestamps.
func sameConditions(a, b []metav1.Condition, sameExceeded bool) bool {
	if len(a) != len(b) {
		return false
	}
	for _, c := range b {
		other := FindCondition(a, c.Type)
		if other == nil || other.Status != c.Status || other.Reason != c.Reason || other.ObservedGeneration != c.ObservedGeneration {
			return false
		}
		usageOnly := c.Type == v1alpha1.ProjectQuotaConditionExceeded && c.Status == metav1.ConditionTrue && sameExceeded
		if other.Message != c.Message && !usageOnly {
			return false
		}
	}
	return true
}

// FindCondition returns the condition of the type, nil when there is none.
func FindCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
