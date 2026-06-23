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

package operations

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

type PhysicalBuilder struct{}

func (PhysicalBuilder) Build(node NodeRef, d Decision) *controlplanev1alpha1.ControlPlaneOperation {
	return buildOperation(node, d, physicalSteps(d))
}

type VirtualBuilder struct{}

func (VirtualBuilder) Build(node NodeRef, d Decision) *controlplanev1alpha1.ControlPlaneOperation {
	return buildOperation(node, d, virtualSteps(d))
}

func physicalSteps(d Decision) []controlplanev1alpha1.StepName {
	return baseSteps(d)
}

func virtualSteps(d Decision) []controlplanev1alpha1.StepName {
	return baseSteps(d)
}

func baseSteps(d Decision) []controlplanev1alpha1.StepName {
	switch d.kind {
	case KindObserve:
		return []controlplanev1alpha1.StepName{controlplanev1alpha1.StepCertObserve}
	case KindSignatureRenew:
		return signatureRenewalSteps()
	default:
		return applySteps(d.component, d.renewCertificates, d.seedSignature)
	}
}

// applySteps builds the apply/restart pipeline driven by component capabilities.
// The step names and set are mode-agnostic; the disk/API difference lives in the executor.
func applySteps(component controlplanev1alpha1.OperationComponent, renew, seedSignature bool) []controlplanev1alpha1.StepName {
	steps := []controlplanev1alpha1.StepName{controlplanev1alpha1.StepBackup}

	if renew {
		steps = append(steps, controlplanev1alpha1.StepSyncCA)
		if component.HasPKI() {
			steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
		}
		if component.HasKubeconfigs() {
			steps = append(steps, controlplanev1alpha1.StepRenewKubeconfigs)
		}
	}

	if seedSignature {
		steps = append(steps, controlplanev1alpha1.StepRenewSignature)
	}

	if component == controlplanev1alpha1.OperationComponentEtcd {
		steps = append(steps, controlplanev1alpha1.StepJoinEtcdCluster)
	} else {
		steps = append(steps, controlplanev1alpha1.StepSyncManifests)
	}

	return append(steps, controlplanev1alpha1.StepWaitPodReady, controlplanev1alpha1.StepCertObserve)
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
