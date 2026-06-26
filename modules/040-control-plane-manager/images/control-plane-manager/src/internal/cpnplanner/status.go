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

package cpnplanner

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
)

// ComputeStatusReport returns the target status computed from the current status and the operations.
// Pure: works on a copy, does not mutate the input.
func ComputeStatusReport(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) controlplanev1alpha1.ControlPlaneNodeStatus {
	out := cpn.DeepCopy()
	applyObservedState(out, ops)
	applyDerivedState(out, ops)
	return out.Status
}

// applyObservedState updates the per-component actual state from completed operations.
func applyObservedState(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) {
	applyCompletedChecksums(cpn, ops)
	applyCertDates(cpn, ops)
}

// applyDerivedState computes the global CAChecksum and conditions from intended vs the observed state.
func applyDerivedState(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) {
	states := computeComponentStates(cpn)
	applyCAChecksum(cpn, states)
	applyConditions(cpn, states, ops)
	applyHealthyCondition(cpn, states)
}

func applyCompletedChecksums(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) {
	for _, s := range computeComponentStates(cpn) {
		applied := latestAppliedOperation(ops, s.component)
		if applied == nil {
			continue
		}
		cs := cpn.Status.Components.Component(s.component)
		if cs == nil {
			continue
		}
		if applied.Spec.DesiredConfigChecksum != "" {
			cs.Checksums.Config = applied.Spec.DesiredConfigChecksum
		}
		if applied.Spec.DesiredPKIChecksum != "" {
			cs.Checksums.PKI = applied.Spec.DesiredPKIChecksum
		}
		if applied.Spec.DesiredCAChecksum != "" {
			cs.Checksums.CA = applied.Spec.DesiredCAChecksum
		}
	}
}

func applyCertDates(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) {
	type snapshot struct {
		at        metav1.Time
		component controlplanev1alpha1.OperationComponent
		state     controlplanev1alpha1.ObservedComponentState
	}
	var snapshots []snapshot
	for i := range ops {
		op := &ops[i]
		if !op.IsCompleted() || op.Status.ObservedState == nil || !op.HasStep(controlplanev1alpha1.StepCertObserve) {
			continue
		}
		at := operationObservedAt(op)
		if at.IsZero() {
			continue
		}
		snapshots = append(snapshots, snapshot{at: at, component: op.Spec.Component, state: *op.Status.ObservedState})
	}
	// Apply in monotonic observedAt order so later observations are not rolled back by earlier ones.
	sort.SliceStable(snapshots, func(i, j int) bool { return snapshots[i].at.Before(&snapshots[j].at) })
	for _, sn := range snapshots {
		cs := cpn.Status.Components.Component(sn.component)
		if cs == nil {
			continue
		}
		if len(sn.state.CertificatesExpirationTime) > 0 {
			// Clone: the source map belongs to a cached operation object and must not be aliased into status.
			cs.CertificatesExpirationTime = maps.Clone(sn.state.CertificatesExpirationTime)
		}
		if cs.LastCertObserveTime.IsZero() || sn.at.After(cs.LastCertObserveTime.Time) {
			cs.LastCertObserveTime = sn.at
		}
	}
}

func applyCAChecksum(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState) {
	if cpn.Spec.CAChecksum == "" || cpn.Status.CAChecksum == cpn.Spec.CAChecksum {
		return
	}
	for _, s := range states {
		if !s.component.IsStaticPodComponent() {
			continue
		}
		cs := cpn.Status.Components.Component(s.component)
		if cs == nil || cs.Checksums.CA != cpn.Spec.CAChecksum {
			return // not all static pod components applied the new CA yet
		}
	}
	cpn.Status.CAChecksum = cpn.Spec.CAChecksum
}

func applyConditions(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState, ops []controlplanev1alpha1.ControlPlaneOperation) {
	for _, s := range states {
		meta.SetStatusCondition(&cpn.Status.Conditions, computeComponentCondition(s, ops, cpn.Generation))
	}
}

func computeComponentCondition(s componentState, ops []controlplanev1alpha1.ControlPlaneOperation, gen int64) metav1.Condition {
	ct := conditionType(s.component)
	op := operationForComponent(ops, s)
	switch {
	case op == nil:
		if s.inSync() {
			return condition(ct, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, gen, "")
		}
		return condition(ct, metav1.ConditionUnknown, controlplanev1alpha1.CPNReasonNotReady, gen, "")
	case op.IsCompleted():
		return condition(ct, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, gen, "")
	case op.IsFailed():
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, op.FailureMessage())
	case op.Spec.Approved:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("operation %s in progress", op.Name))
	default:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("operation %s waiting", op.Name))
	}
}

func applyHealthyCondition(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState) {
	meta.SetStatusCondition(&cpn.Status.Conditions, computeHealthyCondition(cpn, states))
}

func computeHealthyCondition(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState) metav1.Condition {
	gen := cpn.Generation
	if len(states) == 0 {
		return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionUnknown, controlplanev1alpha1.CPNReasonUnknown, gen, "no components")
	}
	var caOutOfSync, expiring []string
	for _, s := range states {
		if s.intended.CA != "" && s.actual.CA != s.intended.CA {
			caOutOfSync = append(caOutOfSync, string(s.component))
			continue
		}
		dates := certDates(cpn, s.component)
		if len(dates) == 0 {
			return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionUnknown, controlplanev1alpha1.CPNReasonUnknown, gen, string(s.component))
		}
		m := minExpiration(dates)
		if m.IsZero() {
			return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionUnknown, controlplanev1alpha1.CPNReasonUnknown, gen, string(s.component))
		}
		if time.Until(m) < constants.CertRenewalThreshold {
			expiring = append(expiring, string(s.component))
		}
	}
	if len(caOutOfSync) > 0 {
		sort.Strings(caOutOfSync)
		return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonWaitingForComponents, gen, strings.Join(caOutOfSync, ", "))
	}
	if len(expiring) > 0 {
		sort.Strings(expiring)
		return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonExpiredSoon, gen, strings.Join(expiring, ", "))
	}
	return condition(controlplanev1alpha1.CPNConditionCertificatesHealthy, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonHealthy, gen, "")
}

func conditionType(c controlplanev1alpha1.OperationComponent) string {
	switch c {
	case controlplanev1alpha1.OperationComponentEtcd:
		return controlplanev1alpha1.CPNConditionEtcdReady
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return controlplanev1alpha1.CPNConditionAPIServerReady
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		return controlplanev1alpha1.CPNConditionControllerManagerReady
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		return controlplanev1alpha1.CPNConditionSchedulerReady
	default:
		return ""
	}
}

func condition(t string, status metav1.ConditionStatus, reason string, gen int64, msg string) metav1.Condition {
	return metav1.Condition{Type: t, Status: status, Reason: reason, ObservedGeneration: gen, Message: msg}
}

func operationForComponent(ops []controlplanev1alpha1.ControlPlaneOperation, s componentState) *controlplanev1alpha1.ControlPlaneOperation {
	matches := operations.MatchesChecksums(s.intended)
	var running, pending, completed, terminal *controlplanev1alpha1.ControlPlaneOperation
	for i := range ops {
		op := &ops[i]
		if op.Spec.Component != s.component || !matches(op) {
			continue
		}
		if !op.IsTerminal() {
			if op.Spec.Approved {
				running = later(running, op)
			} else {
				pending = later(pending, op)
			}
			continue
		}
		terminal = later(terminal, op)
		if op.IsCompleted() {
			completed = later(completed, op)
		}
	}
	switch {
	case running != nil:
		return running
	case pending != nil:
		return pending
	case completed != nil:
		return completed
	default:
		return terminal
	}
}

func latestAppliedOperation(ops []controlplanev1alpha1.ControlPlaneOperation, c controlplanev1alpha1.OperationComponent) *controlplanev1alpha1.ControlPlaneOperation {
	var latest *controlplanev1alpha1.ControlPlaneOperation
	for i := range ops {
		op := &ops[i]
		if op.Spec.Component != c || op.IsObserveOnlyOperation() {
			continue
		}
		if !op.IsTerminal() || (!op.IsCompleted() && !operations.HasCommitPoint(op)) {
			continue
		}
		latest = later(latest, op)
	}
	return latest
}

// later returns the latest operation by creation timestamp; current may be nil (first seen wins).
func later(current, op *controlplanev1alpha1.ControlPlaneOperation) *controlplanev1alpha1.ControlPlaneOperation {
	if current == nil || op.CreationTimestamp.After(current.CreationTimestamp.Time) {
		return op
	}
	return current
}

func operationObservedAt(op *controlplanev1alpha1.ControlPlaneOperation) metav1.Time {
	c := meta.FindStatusCondition(op.Status.Conditions, controlplanev1alpha1.CPOConditionCompleted)
	if c != nil && c.Status == metav1.ConditionTrue && !c.LastTransitionTime.IsZero() {
		return c.LastTransitionTime
	}
	if !op.CreationTimestamp.IsZero() {
		return metav1.NewTime(op.CreationTimestamp.Time)
	}
	return metav1.Time{}
}

func certDates(cpn *controlplanev1alpha1.ControlPlaneNode, c controlplanev1alpha1.OperationComponent) map[string]metav1.Time {
	cs := cpn.Status.Components.Component(c)
	if cs == nil {
		return nil
	}
	return cs.CertificatesExpirationTime
}
