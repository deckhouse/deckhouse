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

type VirtualStepBuilder struct{}

// Targets returns the operations the component needs this reconcile, each paired with its deduplication rule.
//
// Two independent decisions per component:
//   - lifecycle: a mutating converge (spec drift) or a read-only observe — mutually exclusive;
//   - renewal: an expiring leaf certificate or signature key — runs in parallel to the lifecycle.
func (VirtualStepBuilder) TargetSteps(s componentState) []TargetSteps {
	var targets []TargetSteps
	switch {
	case s.needsConverge():
		targets = append(targets, TargetSteps{Steps: convergeSteps(s), HasDuplicate: operations.MatchesChecksums(s.intended)})
	case s.needsObserve():
		targets = append(targets, TargetSteps{Steps: observeSteps(), HasDuplicate: operations.IsAnyActiveOperation})
	}
	switch {
	case s.needsCertRenew():
		targets = append(targets, TargetSteps{Steps: certRenewalSteps(s), HasDuplicate: operations.HasRenewalStep})
	case s.needsSignatureRenew():
		targets = append(targets, TargetSteps{Steps: signatureRenewalSteps(s), HasDuplicate: operations.HasSignatureStep})
	}
	return targets
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
