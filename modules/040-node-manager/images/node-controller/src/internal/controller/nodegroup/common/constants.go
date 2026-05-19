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

package common

import nodecommon "github.com/deckhouse/node-controller/internal/common"

const (
	ConditionTypeReady                        = "Ready"
	ConditionTypeUpdating                     = "Updating"
	ConditionTypeWaitingForDisruptiveApproval = "WaitingForDisruptiveApproval"
	ConditionTypeError                        = "Error"
	ConditionTypeScaling                      = "Scaling"
	ConditionTypeFrozen                       = "Frozen"
	CloudProviderSecretName                   = "d8-node-manager-cloud-provider"

	// Re-exported from internal/common.
	NodeGroupLabel                   = nodecommon.NodeGroupLabel
	ConfigurationChecksumAnnotation  = nodecommon.ConfigurationChecksumAnnotation
	MachineNamespace                 = nodecommon.MachineNamespace
	ConfigurationChecksumsSecretName = nodecommon.ConfigurationChecksumsSecretName
	DisruptionRequiredAnnotation     = nodecommon.DisruptionRequiredAnnotation
	ApprovedAnnotation               = nodecommon.ApprovedAnnotation
)
