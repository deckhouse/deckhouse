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

// HasActiveOperation reports whether a non-terminal operation of the component matches the predicate.
func HasActiveOperation(current []controlplanev1alpha1.ControlPlaneOperation, component controlplanev1alpha1.OperationComponent, match func(*controlplanev1alpha1.ControlPlaneOperation) bool) bool {
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

func HasRenewalStep(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.HasStep(controlplanev1alpha1.StepRenewPKICerts) ||
		op.HasStep(controlplanev1alpha1.StepRenewKubeconfigs)
}

func HasSignatureStep(op *controlplanev1alpha1.ControlPlaneOperation) bool {
	return op.HasStep(controlplanev1alpha1.StepRenewSignature)
}

func MatchesChecksums(target controlplanev1alpha1.Checksums) func(*controlplanev1alpha1.ControlPlaneOperation) bool {
	return func(op *controlplanev1alpha1.ControlPlaneOperation) bool {
		return op.Spec.DesiredConfigChecksum == target.Config &&
			op.Spec.DesiredPKIChecksum == target.PKI &&
			op.Spec.DesiredCAChecksum == target.CA
	}
}

func IsAnyActiveOperation(*controlplanev1alpha1.ControlPlaneOperation) bool {
	return true
}
