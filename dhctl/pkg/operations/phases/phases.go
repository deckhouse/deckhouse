// Copyright 2026 Flant JSC
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

type ProgressAction string

func (a ProgressAction) IsZero() bool {
	return a == ""
}

type PhaseWithSubPhases struct {
	Phase     OperationPhase      `json:"phase"`
	Action    *ProgressAction     `json:"action,omitempty,omitzero"`
	SubPhases []OperationSubPhase `json:"subPhases,omitempty"`

	includeIf func(opts phasesOpts) bool
}

const (
	ProgressActionDefault ProgressAction = ""
	ProgressActionSkip    ProgressAction = "skip"
)

const (
	OperationBootstrap       Operation = "Bootstrap"
	OperationConverge        Operation = "Converge"
	OperationCheck           Operation = "Check"
	OperationDestroy         Operation = "Destroy"
	OperationCommanderAttach Operation = "CommanderAttach"
	OperationCommanderDetach Operation = "CommanderDetach"
)

// ClusterConfig holds cluster parameters that affect phase list and progress.
// Pass via SetClusterConfig as soon as meta config is parsed, before any phase is reported.
// Extensible for future fields (e.g. cloud provider, features).
type ClusterConfig struct {
	ClusterType string
}

// Define common operations phases for such operations as bootstrap, converge and destroy.
// Notice that each operation could define own phases (like attach operation do).
const (
	// metaphase for all
	PreparationPhase OperationPhase = "Preparation"
	// bootstrap and converge both
	BaseInfraPhase OperationPhase = "BaseInfra"
	// bootstrap only
	PreInfraPreflightsPhase                OperationPhase = "PreInfraPreflights"
	PostInfraPreflightsPhase               OperationPhase = "PostInfraPreflights"
	InstallKubernetesPhase                 OperationPhase = "InstallKubernetes"
	InstallRegistryPhase                   OperationPhase = "InstallRegistry"
	InstallDeckhousePhase                  OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase                   OperationPhase = "CreateResources"
	InstallAdditionalMastersAndStaticNodes OperationPhase = "InstallAdditionalMastersAndStaticNodes"
	DeleteResourcesPhase                   OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase                 OperationPhase = "ExecPostBootstrap"
	// converge only
	ConvergeCheckPhase          OperationPhase = "Check"
	AllNodesPhase               OperationPhase = "AllNodes"
	ScaleToMultiMasterPhase     OperationPhase = "ScaleToMultiMaster"
	ScaleToSingleMasterPhase    OperationPhase = "ScaleToSingleMaster"
	DeckhouseConfigurationPhase OperationPhase = "DeckhouseConfiguration"
	// destroy only
	CreateStaticDestroyerNodeUserPhase OperationPhase = "CreateStaticDestroyerNodeUser"
	UpdateStaticDestroyerIPs           OperationPhase = "UpdateStaticDestroyerIPs"
	WaitStaticDestroyerNodeUserPhase   OperationPhase = "WaitStaticDestroyerNodeUser"
	SetDeckhouseResourcesDeletedPhase  OperationPhase = "SetDeckhouseResourcesDelete"
	CommanderUUIDWasChecked            OperationPhase = "CommanderUUIDWasChecked"
	// check only
	CheckInfra         OperationPhase = "CheckInfra"
	CheckConfiguration OperationPhase = "CheckConfiguration"
	// all
	FinalizationPhase OperationPhase = "Finalization"
)

// commander attach phases
const (
	CommanderAttachScanPhase    OperationPhase = "Scan"
	CommanderAttachCapturePhase OperationPhase = "Capture"
	CommanderAttachCheckPhase   OperationPhase = "Check"
)

// commander detach phases
const (
	CommanderDetachCheckPhase  OperationPhase = "Check"
	CommanderDetachDetachPhase OperationPhase = "Detach"
)

var ErrStopOperationCondition = errors.New("StopOperationCondition")

// bootstrap sub phases
const (
	InstallDeckhouseSubPhaseConnect OperationSubPhase = "ConnectToMaster"
	InstallDeckhouseSubPhaseInstall OperationSubPhase = "InstallDeckhouse"
	InstallDeckhouseSubPhaseWait    OperationSubPhase = "WaitForFirstMasterReady"
)

// preparation sub phases
const (
	PreparationSubPhaseImagesDownload   OperationSubPhase = "ImagesDownload"
	PreparationSubPhaseConfigValidation OperationSubPhase = "ConfigValidation"
	PreparationSubPhaseCachePreparation OperationSubPhase = "CachePreparation"
	PreparationSubPhaseStatePreparation OperationSubPhase = "StatePreparation"
)

// base infra sub phases
const (
	BaseInfraSubPhaseBaseInfra   OperationSubPhase = "BaseInfra"
	BaseInfraSubPhaseFirstMaster OperationSubPhase = "FirstMaster"
)

// install kubernetes sub phases
const (
	InstallKubernetesSubPhaseBundlePreparation     OperationSubPhase = "BashibleBundlePrepartion"
	InstallKubernetesSubPhaseRegistryPackagesProxy OperationSubPhase = "RegistryPackagesProxy"
	InstallKubernetesSubPhaseNodePreparation       OperationSubPhase = "NodePreparation"
	InstallKubernetesSubPhaseExecuteBashibleBundle OperationSubPhase = "ExecuteBashibleBundle"
)

// InstallAdditionalMastersAndStaticNodes sub phases
const (
	InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters OperationSubPhase = "AdditionalMasters"
	InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes        OperationSubPhase = "StaticNodes"
	InstallAdditionalMastersAndStaticNodesSubPhaseWait              OperationSubPhase = "WaitForControlPlaneManagerReadiness"
)

func BootstrapPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		// PreparationPhase is not implemented yet
		// {
		// 	Phase: PreparationPhase,
		// 	SubPhases: []OperationSubPhase{
		// 		PreparationSubPhaseImagesDownload,
		// 		PreparationSubPhaseConfigValidation,
		// 		PreparationSubPhaseCachePreparation,
		// 		PreparationSubPhaseStatePreparation,
		// 	},
		// },
		{Phase: PreInfraPreflightsPhase},
		{
			Phase:     BaseInfraPhase,
			includeIf: ifNotStatic,
			SubPhases: []OperationSubPhase{
				BaseInfraSubPhaseBaseInfra,
				BaseInfraSubPhaseFirstMaster,
			},
		},
		{Phase: PostInfraPreflightsPhase},
		{
			Phase: InstallKubernetesPhase,
			SubPhases: []OperationSubPhase{
				InstallKubernetesSubPhaseBundlePreparation,
				InstallKubernetesSubPhaseRegistryPackagesProxy,
				InstallKubernetesSubPhaseNodePreparation,
				InstallKubernetesSubPhaseExecuteBashibleBundle,
			},
		},
		{Phase: InstallRegistryPhase},
		{
			Phase: InstallDeckhousePhase,
			SubPhases: []OperationSubPhase{
				InstallDeckhouseSubPhaseConnect,
				InstallDeckhouseSubPhaseInstall,
				InstallDeckhouseSubPhaseWait,
			},
		},
		{
			Phase: InstallAdditionalMastersAndStaticNodes,
			SubPhases: []OperationSubPhase{
				InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters,
				InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes,
				InstallAdditionalMastersAndStaticNodesSubPhaseWait,
			},
		},
		{Phase: CreateResourcesPhase},
		{Phase: ExecPostBootstrapPhase},
		{Phase: FinalizationPhase},
	}
}

func ConvergePhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{
			Phase: ConvergeCheckPhase,
			SubPhases: []OperationSubPhase{
				OperationSubPhase(CheckInfra),
				OperationSubPhase(CheckConfiguration),
			},
		},
		{Phase: BaseInfraPhase, includeIf: ifNotStatic},
		{Phase: InstallDeckhousePhase},
		{Phase: AllNodesPhase},
		{Phase: ScaleToMultiMasterPhase},
		{Phase: DeckhouseConfigurationPhase},
	}
}

func CheckPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: CheckInfra},
		{Phase: CheckConfiguration},
	}
}

func DestroyPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: DeleteResourcesPhase},
		{Phase: AllNodesPhase},
		{Phase: BaseInfraPhase, includeIf: ifNotStatic},
	}
}

func CommanderAttachPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{Phase: CommanderAttachScanPhase},
		{Phase: CommanderAttachCapturePhase},
		{
			Phase: CommanderAttachCheckPhase,
			SubPhases: []OperationSubPhase{
				OperationSubPhase(CheckInfra),
				OperationSubPhase(CheckConfiguration),
			},
		},
	}
}

func CommanderDetachPhases() []PhaseWithSubPhases {
	return []PhaseWithSubPhases{
		{
			Phase: CommanderDetachCheckPhase,
			SubPhases: []OperationSubPhase{
				OperationSubPhase(CheckInfra),
				OperationSubPhase(CheckConfiguration),
			},
		},
		{Phase: CommanderDetachDetachPhase},
	}
}

type phasesOpts struct {
	clusterConfig ClusterConfig
}

func operationPhases(operation Operation, opts phasesOpts) []PhaseWithSubPhases {
	p := map[Operation][]PhaseWithSubPhases{
		OperationBootstrap:       BootstrapPhases(),
		OperationConverge:        ConvergePhases(),
		OperationCheck:           CheckPhases(),
		OperationDestroy:         DestroyPhases(),
		OperationCommanderAttach: CommanderAttachPhases(),
		OperationCommanderDetach: CommanderDetachPhases(),
	}[operation]

	phases := make([]PhaseWithSubPhases, 0, len(p))
	for _, phase := range p {
		if phase.includeIf == nil || phase.includeIf(opts) {
			phases = append(phases, phase)
		}
	}

	return phases
}

func ifNotStatic(opts phasesOpts) bool {
	return opts.clusterConfig.ClusterType != "Static"
}
