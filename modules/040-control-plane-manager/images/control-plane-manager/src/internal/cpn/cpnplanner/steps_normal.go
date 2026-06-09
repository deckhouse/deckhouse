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
)

type NormalOperationBuilder struct{}

func (NormalOperationBuilder) Targets(s componentState) []TargetOperation {
	return targets(s, normalPipeline)
}

func normalPipeline(s componentState, renewCerts, renewSignature bool) []controlplanev1alpha1.StepName {
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
