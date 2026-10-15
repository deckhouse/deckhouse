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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
	"control-plane-manager/internal/operations"
	"control-plane-manager/pkg/podstatus"
)

// ComputeStatusReport returns the target status.caChecksum and status.components; conditions
// are carried over untouched (see ComputeConditions). Does not mutate the input.
func ComputeStatusReport(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) controlplanev1alpha1.ControlPlaneNodeStatus {
	out := cpn.DeepCopy()
	applyObservedState(out, ops)
	applyCAChecksum(out, computeComponentStates(out))
	return out.Status
}

// ComputeConditions returns the target status.conditions from the spec, the node's static
// (mirror) pods by component (missing key = no pod), the already applied status.components and
// the operations. Semantics are described in docs/controller-control-plane-node-conditions.md.
// Does not mutate the input.
func ComputeConditions(cpn *controlplanev1alpha1.ControlPlaneNode, pods map[controlplanev1alpha1.OperationComponent]*corev1.Pod, ops []controlplanev1alpha1.ControlPlaneOperation) []metav1.Condition {
	out := cpn.DeepCopy()
	states := computeComponentStates(out)
	pruneComponentConditions(out, states)
	applyConditions(out, states, pods, ops)
	applyHealthyCondition(out, states)
	return out.Status.Conditions
}

// applyObservedState updates the per-component actual state from completed operations.
func applyObservedState(cpn *controlplanev1alpha1.ControlPlaneNode, ops []controlplanev1alpha1.ControlPlaneOperation) {
	applyCompletedChecksums(cpn, ops)
	applyCertDates(cpn, ops)
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
	snapshots := make([]snapshot, 0, len(ops))
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

// pruneComponentConditions removes PodReady/InSync/Ready conditions of components no longer in
// the spec; condition types of other writers are kept.
func pruneComponentConditions(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState) {
	wanted := make(map[string]bool, len(states)*3)
	for _, s := range states {
		for _, t := range componentConditionTypes(s.component) {
			wanted[t] = true
		}
	}
	for component := range controlplanev1alpha1.ComponentRegistry() {
		for _, t := range componentConditionTypes(component) {
			if !wanted[t] {
				meta.RemoveStatusCondition(&cpn.Status.Conditions, t)
			}
		}
	}
}

func applyConditions(cpn *controlplanev1alpha1.ControlPlaneNode, states []componentState, podsByComponent map[controlplanev1alpha1.OperationComponent]*corev1.Pod, ops []controlplanev1alpha1.ControlPlaneOperation) {
	for _, s := range states {
		podReady := computePodReadyCondition(s, podsByComponent[s.component], cpn.Generation)
		inSync := computeComponentInSyncCondition(s, ops, cpn.Generation)
		meta.SetStatusCondition(&cpn.Status.Conditions, podReady)
		meta.SetStatusCondition(&cpn.Status.Conditions, inSync)
		meta.SetStatusCondition(&cpn.Status.Conditions, computeComponentReadyCondition(s, podReady, inSync, cpn.Generation))
	}
}

// computeComponentReadyCondition summarizes PodReady and InSync (kubectl columns, dhctl).
// A failing PodReady wins over InSync; reason and message come from the failing one.
func computeComponentReadyCondition(s componentState, podReady, inSync metav1.Condition, gen int64) metav1.Condition {
	ct := readyType(s.component)
	switch {
	case podReady.Status != metav1.ConditionTrue:
		return condition(ct, metav1.ConditionFalse, podReady.Reason, gen, podReady.Message)
	case inSync.Status != metav1.ConditionTrue:
		return condition(ct, metav1.ConditionFalse, inSync.Reason, gen, inSync.Message)
	default:
		return condition(ct, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, gen, "")
	}
}

func computePodReadyCondition(s componentState, pod *corev1.Pod, gen int64) metav1.Condition {
	ct := podReadyType(s.component)
	switch {
	case pod == nil:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, "static pod not found")
	case pod.DeletionTimestamp != nil:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("static pod %s is terminating", pod.Name))
	case pod.Status.Phase != corev1.PodRunning:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("static pod phase is %s", pod.Status.Phase))
	case podstatus.IsCrashLooping(pod):
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("static pod %s is in CrashLoopBackOff", pod.Name))
	case !podstatus.IsReady(pod):
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonNotReady, gen, fmt.Sprintf("static pod %s is not ready", pod.Name))
	default:
		return condition(ct, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonReady, gen, "")
	}
}

// computeComponentInSyncCondition: an active operation wins over the checksum comparison
// (several per component are possible, OperationInQueue wins over OperationInFlight);
// without one, the applied checksums are compared with the desired ones.
func computeComponentInSyncCondition(s componentState, ops []controlplanev1alpha1.ControlPlaneOperation, gen int64) metav1.Condition {
	ct := inSyncType(s.component)
	var active []*controlplanev1alpha1.ControlPlaneOperation
	inQueue := false
	for i := range ops {
		if isActiveOperation(&ops[i], s.component) {
			active = append(active, &ops[i])
			inQueue = inQueue || !ops[i].Spec.Approved
		}
	}
	switch {
	case inQueue:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInQueue, gen, operationNames(active))
	case len(active) > 0:
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOperationInFlight, gen, operationNames(active))
	case !s.inSync():
		return condition(ct, metav1.ConditionFalse, controlplanev1alpha1.CPNReasonOutOfSync, gen, strings.Join(outOfSyncChecksums(s), ", "))
	default:
		return condition(ct, metav1.ConditionTrue, controlplanev1alpha1.CPNReasonInSync, gen, "")
	}
}

// isActiveOperation: observe-only operations (CertObserve) do not change the component.
func isActiveOperation(op *controlplanev1alpha1.ControlPlaneOperation, component controlplanev1alpha1.OperationComponent) bool {
	return op.Spec.Component == component && !op.IsTerminal() && !op.IsObserveOnlyOperation()
}

// outOfSyncChecksums names the checksums that differ, mirroring componentState.inSync.
func outOfSyncChecksums(s componentState) []string {
	var parts []string
	if s.intended.Config != s.actual.Config {
		parts = append(parts, "config checksum differs")
	}
	if s.component.HasPKI() && s.intended.PKI != s.actual.PKI {
		parts = append(parts, "pki checksum differs")
	}
	if s.intended.CA != s.actual.CA {
		parts = append(parts, "ca checksum differs")
	}
	return parts
}

// operationNames sorts ops in place by creation time, then name, and joins their names.
func operationNames(ops []*controlplanev1alpha1.ControlPlaneOperation) string {
	sort.Slice(ops, func(i, j int) bool {
		if !ops[i].CreationTimestamp.Equal(&ops[j].CreationTimestamp) {
			return ops[i].CreationTimestamp.Before(&ops[j].CreationTimestamp)
		}
		return ops[i].Name < ops[j].Name
	})
	names := make([]string, len(ops))
	for i, op := range ops {
		names[i] = op.Name
	}
	return strings.Join(names, ", ")
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

func podReadyType(c controlplanev1alpha1.OperationComponent) string {
	switch c {
	case controlplanev1alpha1.OperationComponentEtcd:
		return controlplanev1alpha1.CPNConditionEtcdPodReady
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return controlplanev1alpha1.CPNConditionAPIServerPodReady
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		return controlplanev1alpha1.CPNConditionControllerManagerPodReady
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		return controlplanev1alpha1.CPNConditionSchedulerPodReady
	default:
		return ""
	}
}

func readyType(c controlplanev1alpha1.OperationComponent) string {
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

func componentConditionTypes(c controlplanev1alpha1.OperationComponent) []string {
	return []string{podReadyType(c), inSyncType(c), readyType(c)}
}

func inSyncType(c controlplanev1alpha1.OperationComponent) string {
	switch c {
	case controlplanev1alpha1.OperationComponentEtcd:
		return controlplanev1alpha1.CPNConditionEtcdInSync
	case controlplanev1alpha1.OperationComponentKubeAPIServer:
		return controlplanev1alpha1.CPNConditionAPIServerInSync
	case controlplanev1alpha1.OperationComponentKubeControllerManager:
		return controlplanev1alpha1.CPNConditionControllerManagerInSync
	case controlplanev1alpha1.OperationComponentKubeScheduler:
		return controlplanev1alpha1.CPNConditionSchedulerInSync
	default:
		return ""
	}
}

func condition(t string, status metav1.ConditionStatus, reason string, gen int64, msg string) metav1.Condition {
	return metav1.Condition{Type: t, Status: status, Reason: reason, ObservedGeneration: gen, Message: msg}
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
