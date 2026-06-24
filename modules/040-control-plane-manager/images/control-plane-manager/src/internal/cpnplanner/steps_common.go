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

// observeSteps is the read-only pipeline: record the component's certificate expiry
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
