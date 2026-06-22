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
package cpn

import (
	"sort"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/checksum"
	"control-plane-manager/internal/constants"
)

// componentState is the intended (from spec) versus actual (from status) state of a single component.
type componentState struct {
	component       controlplanev1alpha1.OperationComponent
	intended        controlplanev1alpha1.Checksums // Config, PKI, CA(=global)
	actual          controlplanev1alpha1.Checksums // Config, PKI, CA(per-component)
	certExpiry      time.Time                      // earliest observed leaf-certificate expiry (excludes signature); zero if unobserved
	signatureExpiry time.Time                      // observed signature-key expiry (kube-apiserver, CSE); zero if unobserved
	lastObserve     time.Time                      // last successful certificate observation; zero if never
}

func (s componentState) inSync() bool {
	return s.intended.Config == s.actual.Config &&
		(!hasPKI(s.component) || s.intended.PKI == s.actual.PKI) &&
		s.intended.CA == s.actual.CA
}

func (s componentState) certsChanged() bool {
	return s.intended.PKI != s.actual.PKI || s.intended.CA != s.actual.CA
}

func (s componentState) needsConverge() bool {
	return !s.inSync()
}

func (s componentState) needsObserve() bool {
	if s.actual.Config == "" {
		return false // not deployed yet, nothing to observe
	}
	return s.lastObserve.IsZero() || time.Since(s.lastObserve) > constants.CertObserveInterval
}

// needsCertRenew reports whether leaf certificates expire soon and no converge is already reissuing them.
func (s componentState) needsCertRenew() bool {
	if s.certsChanged() {
		return false
	}
	return !s.certExpiry.IsZero() && time.Until(s.certExpiry) < constants.CertRenewalThreshold
}

// needsSignatureRenew reports whether the kube-apiserver signature key expires soon (CSE builds only).
func (s componentState) needsSignatureRenew() bool {
	if !constants.SignatureEnabled() || s.component != controlplanev1alpha1.OperationComponentKubeAPIServer {
		return false
	}
	return !s.signatureExpiry.IsZero() && time.Until(s.signatureExpiry) < constants.SignatureRenewalThreshold
}

// needsSignatureBootstrap reports whether the first kube-apiserver deploy must seed the signature keys (CSE builds only).
func (s componentState) needsSignatureBootstrap() bool {
	return constants.SignatureEnabled() &&
		s.component == controlplanev1alpha1.OperationComponentKubeAPIServer &&
		s.actual.Config == ""
}

func hasPKI(c controlplanev1alpha1.OperationComponent) bool {
	return c == controlplanev1alpha1.OperationComponentEtcd ||
		c == controlplanev1alpha1.OperationComponentKubeAPIServer
}

func hasKubeconfigs(c controlplanev1alpha1.OperationComponent) bool {
	return c != controlplanev1alpha1.OperationComponentEtcd
}

// computeComponentStates pairs intended (spec + global CA) with actual (status) per component, in a stable order.
func computeComponentStates(cpn *controlplanev1alpha1.ControlPlaneNode) []componentState {
	type entry struct {
		component controlplanev1alpha1.OperationComponent
		spec      controlplanev1alpha1.Checksums
	}
	entries := []entry{
		{controlplanev1alpha1.OperationComponentEtcd, cpn.Spec.Components.Etcd.Checksums},
		{controlplanev1alpha1.OperationComponentKubeAPIServer, cpn.Spec.Components.KubeAPIServer.Checksums},
		{controlplanev1alpha1.OperationComponentKubeControllerManager, cpn.Spec.Components.KubeControllerManager.Checksums},
		{controlplanev1alpha1.OperationComponentKubeScheduler, cpn.Spec.Components.KubeScheduler.Checksums},
	}

	states := make([]componentState, 0, len(entries))
	for _, e := range entries {
		if e.spec.Config == "" && e.spec.PKI == "" {
			continue // component not configured for this node
		}
		st := componentState{
			component: e.component,
			intended: controlplanev1alpha1.Checksums{
				Config: e.spec.Config,
				PKI:    e.spec.PKI,
				CA:     cpn.Spec.CAChecksum,
			},
		}
		if cs := cpn.Status.Components.Component(e.component); cs != nil {
			st.actual = cs.Checksums
			st.certExpiry = minExpirationExcluding(cs.CertificatesExpirationTime, constants.SignatureExpirationKey)
			st.signatureExpiry = cs.CertificatesExpirationTime[constants.SignatureExpirationKey].Time
			st.lastObserve = cs.LastCertObserveTime.Time
		}
		states = append(states, st)
	}
	return states
}

func (s componentState) convergeSteps() []controlplanev1alpha1.StepName {
	return s.buildSteps(s.certsChanged())
}

func (s componentState) certRenewalSteps() []controlplanev1alpha1.StepName {
	return s.buildSteps(true)
}

func signatureRenewalSteps() []controlplanev1alpha1.StepName {
	return []controlplanev1alpha1.StepName{
		controlplanev1alpha1.StepBackup,
		controlplanev1alpha1.StepRenewSignature,
		controlplanev1alpha1.StepSyncManifests,
		controlplanev1alpha1.StepWaitPodReady,
		controlplanev1alpha1.StepCertObserve,
	}
}

// buildSteps returns the apply/restart pipeline for a component, driven by its capabilities.
// The step names and set are mode-agnostic; the disk/API difference lives in the executor.
func (s componentState) buildSteps(renew bool) []controlplanev1alpha1.StepName {
	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}

	if renew {
		steps = append(steps, controlplanev1alpha1.StepSyncCA)
		if hasPKI(s.component) {
			steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
		}
		if hasKubeconfigs(s.component) {
			steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
		}
	}

	if s.needsSignatureBootstrap() {
		steps = append(steps, controlplanev1alpha1.StepRenewSignature)
	}

	if s.component == controlplanev1alpha1.OperationComponentEtcd {
		steps = append(steps, controlplanev1alpha1.StepJoinEtcdCluster)
	} else {
		steps = append(steps, controlplanev1alpha1.StepSyncManifests)
	}

	return append(steps, controlplanev1alpha1.StepWaitPodReady, controlplanev1alpha1.StepCertObserve)
}

func buildOperation(cpn *controlplanev1alpha1.ControlPlaneNode, s componentState, steps []controlplanev1alpha1.StepName) *controlplanev1alpha1.ControlPlaneOperation {
	op := &controlplanev1alpha1.ControlPlaneOperation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cpn.Namespace,
			Labels: map[string]string{
				constants.ControlPlaneNodeNameLabelKey:  cpn.Name,
				constants.ControlPlaneComponentLabelKey: s.component.LabelValue(),
				constants.ControlPlaneTypeLabelKey:      cpn.Labels[constants.ControlPlaneTypeLabelKey],
				constants.HeritageLabelKey:              constants.HeritageLabelValue,
			},
		},
		Spec: controlplanev1alpha1.ControlPlaneOperationSpec{
			NodeName:  cpn.Name,
			Component: s.component,
			Steps:     steps,
		},
	}
	// An observe-only operation is read-only - no desired state to converge to and no approval needed.
	if op.IsObserveOnlyOperation() {
		op.Spec.Approved = true
	} else {
		op.Spec.Approved = false
		op.Spec.DesiredConfigChecksum = s.intended.Config
		op.Spec.DesiredPKIChecksum = s.intended.PKI
		op.Spec.DesiredCAChecksum = s.intended.CA
	}
	op.GenerateName = operationGenerateName(op)
	return op
}

func operationGenerateName(op *controlplanev1alpha1.ControlPlaneOperation) string {
	name := strings.ToLower(string(op.Spec.Component))
	for _, ck := range []string{
		op.Spec.DesiredConfigChecksum,
		op.Spec.DesiredPKIChecksum,
		op.Spec.DesiredCAChecksum,
	} {
		if ck != "" {
			name += "-" + checksum.ShortChecksum(ck)
		}
	}
	return name + "-"
}

// targetOperation is a desired operation paired with its dedup rule:
// isDuplicate - predicate, reports whether a given active operation of the same component already covers this target.
type targetOperation struct {
	op          *controlplanev1alpha1.ControlPlaneOperation
	isDuplicate func(active *controlplanev1alpha1.ControlPlaneOperation) bool
}

// buildTargetOperations builds the desired CPOs for a node, reacting to each component's state.
//
// Two independent decisions per component:
//   - lifecycle: a mutating converge (spec drift) or a read-only observe — mutually exclusive;
//   - renewal: an expiring leaf certificate or signature key — runs in parallel switch case to the lifecycle.
func buildTargetOperations(cpn *controlplanev1alpha1.ControlPlaneNode) []targetOperation {
	var targets []targetOperation
	for _, s := range computeComponentStates(cpn) {
		switch {
		case s.needsConverge():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, s.convergeSteps()),
				isDuplicate: s.matchesChecksums,
			})
		case s.needsObserve():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}),
				isDuplicate: isAnyActive,
			})
		}

		switch {
		case s.needsCertRenew():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, s.certRenewalSteps()),
				isDuplicate: hasRenewalStep,
			})
		case s.needsSignatureRenew():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, signatureRenewalSteps()),
				isDuplicate: hasSignatureStep,
			})
		}
	}
	return targets
}

// matchesChecksums reports whether op already targets this component's desired checksums.
// Observe-only operations carry no desired checksums, so they never match.
func (s componentState) matchesChecksums(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.Spec.DesiredConfigChecksum == s.intended.Config &&
		op.Spec.DesiredPKIChecksum == s.intended.PKI &&
		op.Spec.DesiredCAChecksum == s.intended.CA
}

// isAnyActive matches any active operation of the component - observe-only operation is the lowest-priority filler.
// Every other pipeline has CertObserve step, so observation is only needed when nothing else is running for the component.
func isAnyActive(*controlplanev1alpha1.ControlPlaneOperation) bool {
	return true
}

func hasRenewalStep(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.HasStep(controlplanev1alpha1.StepRenewPKICerts) ||
		op.HasStep(controlplanev1alpha1.StepRenewKubeconfigs)
}

func hasSignatureStep(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.HasStep(controlplanev1alpha1.StepRenewSignature)
}

// BuildOperations returns the operations the node needs that are not already covered by an active operation of the same component.
func BuildOperations(cpn *controlplanev1alpha1.ControlPlaneNode, current []controlplanev1alpha1.ControlPlaneOperation) []*controlplanev1alpha1.ControlPlaneOperation {
	targets := buildTargetOperations(cpn)
	var selected []*controlplanev1alpha1.ControlPlaneOperation
	for _, t := range targets {
		if hasActiveOperation(current, t.op.Spec.Component, t.isDuplicate) {
			continue
		}
		selected = append(selected, t.op)
	}
	return selected
}

// FilterOperationsOwnedByCPN keeps only operations whose controller owner reference is this exact CPN (name + UID).
func FilterOperationsOwnedByCPN(ops []controlplanev1alpha1.ControlPlaneOperation, cpn *controlplanev1alpha1.ControlPlaneNode) []controlplanev1alpha1.ControlPlaneOperation {
	filtered := make([]controlplanev1alpha1.ControlPlaneOperation, 0, len(ops))
	for i := range ops {
		if isOperationOwnedByCPN(&ops[i], cpn) {
			filtered = append(filtered, ops[i])
		}
	}
	return filtered
}

func isOperationOwnedByCPN(op *controlplanev1alpha1.ControlPlaneOperation, cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	for i := range op.OwnerReferences {
		ref := op.OwnerReferences[i]
		if ref.APIVersion != controlplanev1alpha1.GroupVersion.String() || ref.Kind != "ControlPlaneNode" {
			continue
		}
		if ref.Name != cpn.Name || ref.UID != cpn.UID {
			continue
		}
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

func hasActiveOperation(current []controlplanev1alpha1.ControlPlaneOperation, component controlplanev1alpha1.OperationComponent, match func(*controlplanev1alpha1.ControlPlaneOperation) bool) bool {
	for i := range current {
		op := &current[i]
		if op.IsTerminal() || op.Spec.Component != component {
			continue
		}
		if match(op) {
			return true
		}
	}
	return false
}

// ComputeOperationsToRotate returns the names of terminal operations exceeding the per-component retention limit, oldest first.
// Active operations are never rotated.
func ComputeOperationsToRotate(current []controlplanev1alpha1.ControlPlaneOperation, keepPerComponent int) []string {
	if keepPerComponent <= 0 {
		return nil
	}

	terminalByComponent := make(map[controlplanev1alpha1.OperationComponent][]*controlplanev1alpha1.ControlPlaneOperation)
	for i := range current {
		op := &current[i]
		if op.IsTerminal() {
			terminalByComponent[op.Spec.Component] = append(terminalByComponent[op.Spec.Component], op)
		}
	}

	var toDelete []string
	for _, ops := range terminalByComponent {
		if len(ops) <= keepPerComponent {
			continue
		}
		sort.SliceStable(ops, func(i, j int) bool {
			ti, tj := ops[i].CreationTimestamp.Time, ops[j].CreationTimestamp.Time
			if ti.Equal(tj) {
				return ops[i].Name < ops[j].Name
			}
			return ti.Before(tj)
		})
		for _, op := range ops[:len(ops)-keepPerComponent] {
			toDelete = append(toDelete, op.Name)
		}
	}

	sort.Strings(toDelete) // deterministic output
	return toDelete
}

// IsMaintenanceMode reports whether operation planning is paused for the node.
func IsMaintenanceMode(cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	_, ok := cpn.Labels[constants.MaintenanceModeLabelKey]
	return ok
}
