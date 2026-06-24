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
	"slices"

	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/operations"
)

type VirtualStepBuilder struct{}

func (VirtualStepBuilder) Steps(s componentState, current []controlplanev1alpha1.ControlPlaneOperation) []controlplanev1alpha1.StepName {
	return buildSteps(s, current)
}

func buildSteps(s componentState, current []controlplanev1alpha1.ControlPlaneOperation) []controlplanev1alpha1.StepName {
	uncovered := uncoveredSteps(current, s)
	if len(uncovered) == 0 {
		return nil
	}
	if isObserveOnly(uncovered) {
		return []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}
	}

	renewCerts := slices.Contains(uncovered, controlplanev1alpha1.StepRenewPKICerts)
	renewKubeconfigs := slices.Contains(uncovered, controlplanev1alpha1.StepRenewKubeconfigs)
	renewSignature := slices.Contains(uncovered, controlplanev1alpha1.StepRenewSignature)

	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}
	if renewCerts || renewKubeconfigs {
		steps = append(steps, controlplanev1alpha1.StepSyncCA)
	}
	if renewCerts {
		steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
	}
	if renewKubeconfigs {
		steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
	}
	if renewSignature {
		steps = append(steps, controlplanev1alpha1.StepRenewSignature)
	}
	steps = append(steps, syncStep(s.component))
	return append(steps, controlplanev1alpha1.StepWaitPodReady, controlplanev1alpha1.StepCertObserve)
}

func neededSteps(s componentState) []controlplanev1alpha1.StepName {
	var steps []controlplanev1alpha1.StepName
	if s.needsCertRenew() {
		if s.component.HasPKI() {
			steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
		}
		if s.component.HasKubeconfigs() {
			steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
		}
	}
	if s.needsSignatureRenew() {
		steps = append(steps, controlplanev1alpha1.StepRenewSignature)
	}
	if s.needsConverge() {
		steps = append(steps, syncStep(s.component))
	}
	if len(steps) == 0 && s.needsObserve() {
		steps = append(steps, controlplanev1alpha1.StepCertObserve)
	}
	return steps
}

func uncoveredSteps(current []controlplanev1alpha1.ControlPlaneOperation, s componentState) []controlplanev1alpha1.StepName {
	var out []controlplanev1alpha1.StepName
	for _, step := range neededSteps(s) {
		if operations.StepCoveredByActiveOperation(current, s.component, step) {
			continue
		}
		out = append(out, step)
	}
	return out
}

func syncStep(c controlplanev1alpha1.OperationComponent) controlplanev1alpha1.StepName {
	if c == controlplanev1alpha1.OperationComponentEtcd {
		return controlplanev1alpha1.StepJoinEtcdCluster
	}
	return controlplanev1alpha1.StepSyncManifests
}

func isObserveOnly(significant []controlplanev1alpha1.StepName) bool {
	return len(significant) == 1 && significant[0] == controlplanev1alpha1.StepCertObserve
}
