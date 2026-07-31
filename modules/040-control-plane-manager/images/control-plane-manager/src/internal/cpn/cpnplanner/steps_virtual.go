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

type VirtualOperationBuilder struct{}

func (VirtualOperationBuilder) Targets(s componentState) []TargetOperation {
	// TODO(virtual/test): using datastore for etcd on vcpc
	if s.component == controlplanev1alpha1.OperationComponentEtcd {
		return nil
	}
	return targets(s, virtualPipeline)
}

// virtualPipeline emits only the steps implemented by the ephemeral executor (VCPO):
// RenewPKICerts, SyncManifests, WaitPodReady, CertObserve.
func virtualPipeline(s componentState, renewCerts, _ bool) []controlplanev1alpha1.StepName {
	var steps []controlplanev1alpha1.StepName
	if renewCerts && s.component.HasPKI() {
		steps = append(steps, controlplanev1alpha1.StepRenewPKICerts)
	}
	steps = append(steps, syncStep(s.component))
	return append(steps, controlplanev1alpha1.StepWaitPodReady, controlplanev1alpha1.StepCertObserve)
}
