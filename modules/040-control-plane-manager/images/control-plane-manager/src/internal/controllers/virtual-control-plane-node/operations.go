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

package virtualcontrolplanenode

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

// componentState is the intended (from spec) versus actual (from status) state of a single component,
// enriched with certificate timing so the trigger predicates read from the state alone.
type componentState struct {
	component   controlplanev1alpha1.OperationComponent
	intended    controlplanev1alpha1.Checksums // Config, PKI, CA(=global)
	actual      controlplanev1alpha1.Checksums // Config, PKI, CA(per-component)
	certExpiry  time.Time                      // earliest observed certificate expiry; zero if unobserved
	lastObserve time.Time                      // last successful certificate observation; zero if never
}

func (s componentState) inSync() bool {
	return s.intended.Config == s.actual.Config &&
		(!hasPKI(s.component) || s.intended.PKI == s.actual.PKI) &&
		s.intended.CA == s.actual.CA
}

// certsChanged reports whether the component's PKI or CA fingerprints drifted.
func (s componentState) certsChanged() bool {
	return s.intended.PKI != s.actual.PKI || s.intended.CA != s.actual.CA
}

// certsExpiring reports whether the earliest observed certificate expires within the renewal threshold.
func (s componentState) certsExpiring() bool {
	return !s.certExpiry.IsZero() && time.Until(s.certExpiry) < constants.CertRenewalThreshold
}

// needsConverge reports whether the component must be (re)applied: spec drift or expiring certificates.
func (s componentState) needsConverge() bool {
	return !s.inSync() || s.certsExpiring()
}

// needsObserve reports whether a deployed component has not been observed within the observe interval.
func (s componentState) needsObserve() bool {
	if s.actual.Config == "" {
		return false // not deployed yet, nothing to observe
	}
	return s.lastObserve.IsZero() || time.Since(s.lastObserve) > constants.CertObserveInterval
}

// hasPKI reports whether the component owns leaf certificates that need renewal.
func hasPKI(c controlplanev1alpha1.OperationComponent) bool {
	return c == controlplanev1alpha1.OperationComponentEtcd ||
		c == controlplanev1alpha1.OperationComponentKubeAPIServer
}

// hasKubeconfigs reports whether the component uses kubeconfig credentials.
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
			st.certExpiry = minExpiration(cs.CertificatesExpirationTime)
			st.lastObserve = cs.LastCertObserveTime.Time
		}
		states = append(states, st)
	}
	return states
}

// buildSteps returns the convergence pipeline for a component, driven by its state and capabilities.
// Certificate steps are included when the certs drifted or are expiring. The disk/API difference lives in the executor.
func buildSteps(s componentState) []controlplanev1alpha1.StepName {
	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}

	if s.certsChanged() || s.certsExpiring() {
		steps = append(steps, controlplanev1alpha1.StepSyncCA)
		if hasPKI(s.component) {
			steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
		}
		if hasKubeconfigs(s.component) {
			steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
		}
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
			GenerateName: fmt.Sprintf("%s-", strings.ToLower(string(s.component))),
			Namespace:    cpn.Namespace,
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
	return op
}

// targetOperation is a desired operation paired with its dedup rule:
// isDuplicate self-duplicates check.
type targetOperation struct {
	op          *controlplanev1alpha1.ControlPlaneOperation
	isDuplicate func(active *controlplanev1alpha1.ControlPlaneOperation) bool
}

// buildTargetOperations builds the desired CPOs for a node, reacting to each component's state.
// A component needs at most one operation - a mutating converge (no-approval) or a read-only observe (auto-approval).
func buildTargetOperations(cpn *controlplanev1alpha1.ControlPlaneNode) []targetOperation {
	var targets []targetOperation
	for _, s := range computeComponentStates(cpn) {
		switch {
		case s.needsConverge():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, buildSteps(s)),
				isDuplicate: sameChecksums(s),
			})
		case s.needsObserve():
			targets = append(targets, targetOperation{
				op:          buildOperation(cpn, s, []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}),
				isDuplicate: isObserveOnly,
			})
		}
	}
	return targets
}

// sameChecksums matches an active operation that already targets the same desired checksums.
// Observe-only operations carry no desired checksums, so they never match.
func sameChecksums(s componentState) func(*controlplanev1alpha1.ControlPlaneOperation) bool {
	return func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
		return op.Spec.DesiredConfigChecksum == s.intended.Config &&
			op.Spec.DesiredPKIChecksum == s.intended.PKI &&
			op.Spec.DesiredCAChecksum == s.intended.CA
	}
}

// isObserveOnly matches an active observe-only operation.
func isObserveOnly(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.IsObserveOnlyOperation()
}

// selectOperationsToCreate returns targets after deduplication, this OPs should be created really.
func selectOperationsToCreate(current []controlplanev1alpha1.ControlPlaneOperation, targets []targetOperation) []*controlplanev1alpha1.ControlPlaneOperation {
	var selected []*controlplanev1alpha1.ControlPlaneOperation
	for _, t := range targets {
		if hasActiveOperation(current, t.op.Spec.Component, t.isDuplicate) {
			continue
		}
		selected = append(selected, t.op)
	}
	return selected
}

// hasActiveOperation reports whether the component has a non-terminal operation matching the predicate.
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

func isMaintenanceMode(cpn *controlplanev1alpha1.ControlPlaneNode) bool {
	_, ok := cpn.Labels[constants.MaintenanceModeLabelKey]
	return ok
}
