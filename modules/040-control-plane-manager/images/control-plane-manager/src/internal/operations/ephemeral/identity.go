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

package ephemeral

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
	"control-plane-manager/internal/constants"
)

type tenantIdentity struct {
	Namespace string
	VCPName   string
}

func tenantIdentityFromOperation(operation *controlplanev1alpha1.ControlPlaneOperation) tenantIdentity {
	return tenantIdentity{
		Namespace: operation.Namespace,
		VCPName:   operation.Labels[constants.VirtualControlPlaneScopeLabelKey],
	}
}

func (t tenantIdentity) pkiSecretName() string {
	return constants.VirtualResourceName(constants.VirtualPKISecretName, t.VCPName)
}

func (t tenantIdentity) configSecretName() string {
	return constants.VirtualResourceName(constants.VirtualRenderedConfigSecretName, t.VCPName)
}

func (t tenantIdentity) apiServerServiceName() string {
	return constants.VirtualResourceName(constants.VirtualAPIServerServiceName, t.VCPName)
}
