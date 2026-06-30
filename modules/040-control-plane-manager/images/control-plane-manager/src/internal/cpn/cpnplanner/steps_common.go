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
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
)

// targets returns the operations the component needs this reconcile, each paired with its deduplication rule.
//
// Two independent decisions per component:
//   - lifecycle: a mutating converge (spec drift) or a read-only observe — mutually exclusive;
//   - renewal: an expiring leaf certificate or signature key — runs in parallel to the lifecycle.
func targets(s componentState) []TargetOperation {
	var targets []TargetOperation
	switch {
	case s.needsConverge():
		targets = append(targets, mutatingTarget(s, convergeSteps(s), operations.MatchesChecksums(s.intended)))
	case s.needsObserve():
		targets = append(targets, observeTarget(s))
	}
	switch {
	case s.needsCertRenew():
		targets = append(targets, mutatingTarget(s, certRenewalSteps(s), operations.HasRenewalStep))
	case s.needsSignatureRenew():
		targets = append(targets, mutatingTarget(s, signatureRenewalSteps(s), operations.HasSignatureStep))
	}
	return targets
}

// mutatingTarget builds a target whose operation drives the component to its intended checksums (needs approval).
func mutatingTarget(s componentState, steps []controlplanev1alpha1.StepName, hasDuplicate func(*controlplanev1alpha1.ControlPlaneOperation) bool) TargetOperation {
	return TargetOperation{
		HasDuplicate: hasDuplicate,
		Build: func(node operations.NodeRef) *controlplanev1alpha1.ControlPlaneOperation {
			return operations.NewOperation(node, s.component, steps, s.intended)
		},
	}
}

// observeTarget builds a read-only, pre-approved observe target.
func observeTarget(s componentState) TargetOperation {
	return TargetOperation{
		HasDuplicate: operations.IsAnyActiveOperation,
		Build: func(node operations.NodeRef) *controlplanev1alpha1.ControlPlaneOperation {
			return operations.NewApprovedOperation(node, s.component, observeSteps())
		},
	}
}

func convergeSteps(s componentState) []controlplanev1alpha1.StepName {
	return pipeline(s, s.certsChanged() || s.certsExpireSoon(), s.needsSignatureBootstrap())
}

func certRenewalSteps(s componentState) []controlplanev1alpha1.StepName {
	return pipeline(s, true, false)
}

func signatureRenewalSteps(s componentState) []controlplanev1alpha1.StepName {
	return pipeline(s, false, true)
}

// pipeline is the apply/restart sequence, driven by the component's capabilities.
func pipeline(s componentState, renewCerts, renewSignature bool) []controlplanev1alpha1.StepName {
	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}
	if renewCerts {
		steps = append(steps, controlplanev1alpha1.StepSyncCA)
		if s.component.HasPKI() {
			steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
		}
		if s.component.HasKubeconfigs() {
			steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
		}
	}
	if renewSignature {
		steps = append(steps, controlplanev1alpha1.StepRenewSignature)
	}
	steps = append(steps, syncStep(s.component))
	return append(steps, controlplanev1alpha1.StepWaitPodReady, controlplanev1alpha1.StepCertObserve)
}

// observeSteps is the read-only pipeline: record the component's certificate expiry.
func observeSteps() []controlplanev1alpha1.StepName {
	return []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}
}

// syncStep is the component's step that converges it to its desired manifest and restarts it.
func syncStep(c controlplanev1alpha1.OperationComponent) controlplanev1alpha1.StepName {
	if c == controlplanev1alpha1.OperationComponentEtcd {
		return controlplanev1alpha1.StepJoinEtcdCluster
	}
	return controlplanev1alpha1.StepSyncManifests
}
