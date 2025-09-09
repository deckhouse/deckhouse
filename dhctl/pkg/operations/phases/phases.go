// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package phases

import (
	"errors"
)

type (
	Operation         string
	OperationPhase    string
	OperationSubPhase string
)

type PhaseAction string

func (a PhaseAction) IsZero() bool {
	return a == ""
}

const (
	OperationBootstrap       Operation = "Bootstrap"
	OperationConverge        Operation = "Converge"
	OperationCheck           Operation = "Check"
	OperationDestroy         Operation = "Destroy"
	OperationCommanderAttach Operation = "CommanderAttach"
	OperationCommanderDetach Operation = "CommanderDetach"
)

// Define common operations phases for such operations as bootstrap, converge and destroy.
// Notice that each operation could define own phases (like attach operation do).
const (
	// bootstrap and converge both
	BaseInfraPhase OperationPhase = "BaseInfra"
	// bootstrap only
	RegistryPackagesProxyPhase             OperationPhase = "RegistryPackagesProxyBundle"
	ExecuteBashibleBundlePhase             OperationPhase = "ExecuteBashibleBundle"
	InstallDeckhousePhase                  OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase                   OperationPhase = "CreateResources"
	InstallAdditionalMastersAndStaticNodes OperationPhase = "InstallAdditionalMastersAndStaticNodes"
	DeleteResourcesPhase                   OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase                 OperationPhase = "ExecPostBootstrap"
	// converge only
	AllNodesPhase               OperationPhase = "AllNodes"
	ScaleToMultiMasterPhase     OperationPhase = "ScaleToMultiMaster"
	ScaleToSingleMasterPhase    OperationPhase = "ScaleToSingleMaster"
	DeckhouseConfigurationPhase OperationPhase = "DeckhouseConfiguration"
	// all
	FinalizationPhase OperationPhase = "Finalization"
)

// commander attach phases
const (
	CommanderAttachScanPhase    OperationPhase = "Scan"
	CommanderAttachCapturePhase OperationPhase = "Capture"
	CommanderAttachCheckPhase   OperationPhase = "Check"
)

var (
	StopOperationCondition = errors.New("StopOperationCondition")
)

// bootstrap sub phases
const (
	InstallDeckhouseSubPhaseConnect OperationSubPhase = "ConnectToMaster"
	InstallDeckhouseSubPhaseInstall OperationSubPhase = "InstallDeckhouse"
	InstallDeckhouseSubPhaseWait    OperationSubPhase = "WaitForFirstMasterReady"
)

const (
	PhaseActionDefault PhaseAction = ""
	PhaseActionSkip    PhaseAction = "skip"
)

type PhaseWithSubPhases struct {
	Phase     OperationPhase      `json:"phase"`
	Action    *PhaseAction        `json:"action,omitempty,omitzero"`
	SubPhases []OperationSubPhase `json:"subPhases,omitempty"`
}

func BootstrapPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: BaseInfraPhase},
		{Phase: RegistryPackagesProxyPhase},
		{Phase: ExecuteBashibleBundlePhase},
		{
			Phase: InstallDeckhousePhase,
			SubPhases: []OperationSubPhase{
				InstallDeckhouseSubPhaseConnect,
				InstallDeckhouseSubPhaseInstall,
				InstallDeckhouseSubPhaseWait,
			},
		},
		{Phase: InstallAdditionalMastersAndStaticNodes},
		{Phase: CreateResourcesPhase},
		{Phase: ExecPostBootstrapPhase},
		{Phase: FinalizationPhase},
	}
}

func ConvergePhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: BaseInfraPhase},
		{Phase: InstallDeckhousePhase},
		{Phase: AllNodesPhase},
		{Phase: ScaleToMultiMasterPhase},
		{Phase: DeckhouseConfigurationPhase},
	}
}

func CheckPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{ // currently no phases for this operation
		{Phase: OperationPhase(OperationCheck)},
	}
}

func DestroyPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: DeleteResourcesPhase},
		{Phase: AllNodesPhase},
		{Phase: BaseInfraPhase},
	}
}

func CommanderAttachPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: CommanderAttachScanPhase},
		{Phase: CommanderAttachCheckPhase},
		{Phase: CommanderAttachCheckPhase},
	}
}

func CommanderDetachPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{ // currently no phases for this operation
		{Phase: OperationPhase(OperationCommanderDetach)},
	}
}

func operationPhases(operation Operation) ([]PhaseWithSubPhases, bool) {
	phase, ok := map[Operation][]PhaseWithSubPhases{
		OperationBootstrap:       BootstrapPhases(),
		OperationConverge:        ConvergePhases(),
		OperationCheck:           CheckPhases(),
		OperationDestroy:         DestroyPhases(),
		OperationCommanderAttach: CommanderAttachPhases(),
		OperationCommanderDetach: CommanderDetachPhases(),
	}[operation]

	return phase, ok
}
