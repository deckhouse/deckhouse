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
	"fmt"
	"slices"
	"strings"
)

type (
	Operation      string
	OperationPhase string
)

// OperationSubPhase is an alias rather than a distinct type: a sub-phase is a tree node like
// any other, and every node stores its name as OperationPhase. The name is kept so that the
// existing CompleteSubPhase call sites and interface declarations keep reading as before.
type OperationSubPhase = OperationPhase

type ProgressAction string

func (a ProgressAction) IsZero() bool {
	return a == ""
}

// node is a tree node. The tree is the single source of truth for what an operation consists
// of; PhaseWithSubPhases below is only its projection onto the wire. Build trees with the
// unexported *Nodes constructors and project them with project.
type node struct {
	Name     OperationPhase
	Children []node

	// includeIf gates the node; nil means always included. Evaluated by gate, recursively:
	// a node that is gated out takes its children with it.
	includeIf func(opts phasesOpts) bool
}

// PhaseWithSubPhases is the projection of the tree onto the wire, and nothing else - no
// gating, no callbacks. It is marshalled verbatim into the progress JSONL that
// --progress-log-file-path promises to users, and copied field by field into the gRPC
// Progress message (server/rpc/dhctl/service.go), so its shape is a published contract.
type PhaseWithSubPhases struct {
	Phase     OperationPhase      `json:"phase"`
	Action    *ProgressAction     `json:"action,omitempty,omitzero"`
	SubPhases []OperationSubPhase `json:"subPhases,omitempty"`
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
//
// It has to stay comparable: ProgressTracker.SetClusterConfig tests it with ==.
type ClusterConfig struct {
	ClusterType string
	// HasClusterConfiguration is the second axis of the gates, independent of the cluster type:
	// a cluster whose control plane dhctl did not create carries no ClusterConfiguration, and
	// then the type is unknown rather than static. Fill it from MetaConfig.HasClusterConfiguration
	// at every call site - left at its zero value it silently drops the nodes gated on it.
	HasClusterConfiguration bool
}

// Define common operations phases for such operations as bootstrap, converge and destroy.
// Notice that each operation could define own phases (like attach operation do).
const (
	// metaphase for all
	PreparationPhase OperationPhase = "Preparation"
	// bootstrap and converge both
	BaseInfraPhase OperationPhase = "BaseInfra"
	// bootstrap only
	// FirstMasterPhase creates master-0. It is a phase of its own, and not the tail of BaseInfra,
	// because it is the last point at which nothing but cloud infrastructure exists: a bootstrap
	// stopped here can still be restarted with a corrected config (see the x-unsafe short circuit
	// in pkg/config/validation.go), which the phase reported as BaseInfra could not express.
	FirstMasterPhase                       OperationPhase = "FirstMaster"
	PreInfraPreflightsPhase                OperationPhase = "PreInfraPreflights"
	PostInfraPreflightsPhase               OperationPhase = "PostInfraPreflights"
	ParseResourcesPhase                    OperationPhase = "ParseResources"
	WaitForSSHOnMasterPhase                OperationPhase = "WaitForSSHOnMaster"
	InstallKubernetesPhase                 OperationPhase = "InstallKubernetes"
	InstallDeckhousePhase                  OperationPhase = "InstallDeckhouse"
	CreateResourcesPhase                   OperationPhase = "CreateResources"
	InstallAdditionalMastersAndStaticNodes OperationPhase = "InstallAdditionalMastersAndStaticNodes"
	// WaitForControlPlaneManagerReadinessPhase waits for control-plane-manager on every master.
	// It is a phase of its own, and not the tail of the one above, because it runs on every cluster
	// dhctl builds a control plane for while installing additional nodes is cloud-only: as a child
	// it would be gated out together with its parent on static clusters.
	WaitForControlPlaneManagerReadinessPhase OperationPhase = "WaitForControlPlaneManagerReadiness"
	DeleteResourcesPhase                     OperationPhase = "DeleteResources"
	ExecPostBootstrapPhase                   OperationPhase = "ExecPostBootstrap"
	// converge only
	ConvergeCheckPhase OperationPhase = "Check"
	AllNodesPhase      OperationPhase = "AllNodes"
	// ScaleToMultiMasterPhase is a converge state value, not an announced phase: it is
	// stored in and read from ConvergeState and never reported to the progress tracker.
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
	BaseInfraSubPhaseBaseInfra OperationSubPhase = "BaseInfra"
)

// install kubernetes sub phases
const (
	InstallKubernetesSubPhaseBundlePreparation     OperationSubPhase = "BashibleBundlePrepartion"
	InstallKubernetesSubPhaseRegistryPackagesProxy OperationSubPhase = "RegistryPackagesProxy"
	InstallKubernetesSubPhaseNodePreparation       OperationSubPhase = "NodePreparation"
	InstallKubernetesSubPhaseModulesPreparation    OperationSubPhase = "ModulesPreparation"
	InstallKubernetesSubPhaseExecuteBashibleBundle OperationSubPhase = "ExecuteBashibleBundle"
)

// InstallAdditionalMastersAndStaticNodes sub phases
const (
	InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters OperationSubPhase = "AdditionalMasters"
	InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes        OperationSubPhase = "StaticNodes"
)

func BootstrapPhases() []PhaseWithSubPhases { return project(bootstrapNodes()) }

func bootstrapNodes() []node {
	return []node{
		// Preparation is first and ungated by construction: it produces the cluster type every
		// other gate reads, so it cannot itself be gated on it, and the walk runs it before the
		// tree can be resolved at all (bootstrap.runPhases).
		{
			Name: PreparationPhase,
			Children: []node{
				{Name: PreparationSubPhaseImagesDownload},
				{Name: PreparationSubPhaseConfigValidation},
				{Name: PreparationSubPhaseCachePreparation},
				{Name: PreparationSubPhaseStatePreparation},
			},
		},
		{Name: PreInfraPreflightsPhase},
		{
			Name:      BaseInfraPhase,
			includeIf: ifCloud,
			Children: []node{
				{Name: BaseInfraSubPhaseBaseInfra},
			},
		},
		{Name: FirstMasterPhase, includeIf: ifCloud},
		// Preflight on the master dhctl just created, before kubeadm runs on it.
		{Name: PostInfraPreflightsPhase, includeIf: ifHasClusterConfiguration},
		// Split out of the node above so that gating it keeps the resource queues InstallDeckhouse
		// and CreateResources create from, which apply to every cluster.
		{Name: ParseResourcesPhase},
		// InstallKubernetes is the only consumer of the wait, and ungated it would open SSH to
		// whatever master host the shared state cache happens to hold - another cluster's.
		{Name: WaitForSSHOnMasterPhase, includeIf: ifHasClusterConfiguration},
		{
			Name:      InstallKubernetesPhase,
			includeIf: ifHasClusterConfiguration,
			Children: []node{
				{Name: InstallKubernetesSubPhaseBundlePreparation},
				{Name: InstallKubernetesSubPhaseRegistryPackagesProxy},
				{Name: InstallKubernetesSubPhaseNodePreparation},
				{Name: InstallKubernetesSubPhaseModulesPreparation},
				{Name: InstallKubernetesSubPhaseExecuteBashibleBundle},
			},
		},
		{
			Name: InstallDeckhousePhase,
			Children: []node{
				{Name: InstallDeckhouseSubPhaseConnect},
				{Name: InstallDeckhouseSubPhaseInstall},
				// Ungated, the wait settles on whichever node the API server lists first, and on a
				// cluster whose nodes have not registered it fails after Deckhouse is installed.
				{Name: InstallDeckhouseSubPhaseWait, includeIf: ifHasClusterConfiguration},
			},
		},
		{
			Name:      InstallAdditionalMastersAndStaticNodes,
			includeIf: ifCloud,
			Children: []node{
				{Name: InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters},
				{Name: InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes},
			},
		},
		// Gated because the checker reports "0 of 0 ready" as success; top-level rather than a child
		// of the cloud-only node above for the reason in its const doc.
		{Name: WaitForControlPlaneManagerReadinessPhase, includeIf: ifHasClusterConfiguration},
		{Name: CreateResourcesPhase},
		{Name: ExecPostBootstrapPhase},
		{Name: FinalizationPhase},
	}
}

// unskippableReason tells why --skip-phase refuses phase, or "" when the phase may be skipped.
// The reasons are the strings below (bootstrap.validatePostInfraPreflightsInputs is what refuses
// the second one downstream): skipping either aborts the walk, on a cloud cluster only after
// BaseInfra and FirstMaster have provisioned.
func unskippableReason(phase OperationPhase) string {
	switch phase {
	case PreparationPhase:
		return "it loads the cluster configuration and initializes the state cache every other phase reads"
	case PreInfraPreflightsPhase:
		return "it builds the preflight runner that PostInfraPreflights then runs, disable the checks themselves with --preflight-skip-check or --preflight-skip-all-checks"
	default:
		return ""
	}
}

// SkippablePhases names the top-level phases of declared that may be skipped, in declaration
// order; see unskippableReason for the ones left out.
func SkippablePhases(declared []PhaseWithSubPhases) []string {
	names := make([]string, 0, len(declared))

	for _, p := range declared {
		if unskippableReason(p.Phase) != "" {
			continue
		}

		names = append(names, string(p.Phase))
	}

	return names
}

// ResolveSkipPhases resolves the phase[/subphase] paths of --skip-phase against declared, which
// is the UNGATED tree on purpose: the cluster type is unknown while flags are parsed, and gating
// on an empty type would reject --skip-phase BaseInfra on a perfectly legal static config.
// Whether a resolved phase is part of one particular run is decided by the walk, once the type
// is known (bootstrap.refuseIfSkipExcluded).
func ResolveSkipPhases(declared []PhaseWithSubPhases, paths []string) ([]OperationPhase, error) {
	resolved := make([]OperationPhase, 0, len(paths))

	for _, path := range paths {
		phase, err := resolveSkipPhase(declared, path)
		if err != nil {
			return nil, err
		}

		resolved = append(resolved, phase)
	}

	return resolved, nil
}

func resolveSkipPhase(declared []PhaseWithSubPhases, path string) (OperationPhase, error) {
	name, _, addressesSubPhase := strings.Cut(path, "/")
	phase := OperationPhase(name)

	if !slices.ContainsFunc(declared, func(p PhaseWithSubPhases) bool { return p.Phase == phase }) {
		return "", fmt.Errorf("unknown phase %q, known phases: %s", path, strings.Join(SkippablePhases(declared), ", "))
	}

	// Checked before the sub-phase rule below so that a path into an un-skippable phase answers
	// with the reason at once, instead of advising the parent name that is itself refused next.
	if reason := unskippableReason(phase); reason != "" {
		return "", fmt.Errorf("phase %q cannot be skipped, %s", name, reason)
	}

	// A sub-phase is not a unit of execution: the work lives in the body of its parent, and only
	// top-level nodes carry a function at all, so skipping one would exit zero having done it.
	if addressesSubPhase {
		return "", fmt.Errorf("phase %q addresses a sub-phase, you can only skip a top-level phase, name %q instead", path, name)
	}

	return phase, nil
}

func ConvergePhases() []PhaseWithSubPhases { return project(convergeNodes()) }

func convergeNodes() []node {
	return []node{
		{
			Name: ConvergeCheckPhase,
			Children: []node{
				// Only converge gates this child: Checker.checkInfra is called for cloud clusters
				// only, and converge is the one caller that knows the cluster type before the tree
				// is read (converger.go, SetClusterConfig). The check, commander-attach and
				// commander-detach trees below keep it ungated - they never learn the type at all,
				// so a gate there would drop the node from every run while the code kept
				// announcing it.
				{Name: CheckInfra, includeIf: ifCloud},
				{Name: CheckConfiguration},
			},
		},
		{Name: BaseInfraPhase, includeIf: ifCloud},
		{Name: InstallDeckhousePhase},
		{Name: AllNodesPhase},
		{Name: DeckhouseConfigurationPhase},
	}
}

func CheckPhases() []PhaseWithSubPhases { return project(checkNodes()) }

func checkNodes() []node {
	return []node{
		{Name: CheckInfra},
		{Name: CheckConfiguration},
	}
}

func DestroyPhases() []PhaseWithSubPhases { return project(destroyNodes()) }

// destroyNodes declares the phases in announcement order, which for destroy is not the
// order of execution and cannot be made so. Three known deviations block the conversion of
// destroy to a declarative tree; each is a defect of destroy, not a property of the model:
//
//   - UpdateStaticDestroyerIPs executes inside AllNodes (static/destroyer.go:141 -> :310 -> :527)
//     but is declared before it. Declared after, it would be AllNodes that the tracker marks
//     skipped. As declared, UpdateStaticDestroyerIPs itself goes on the wire with Action=Skip
//     even though it ran.
//   - The same node is announced once per processed host, from two consecutive loops
//     (static/destroyer.go:236-253 and :256-278), so the Phase/SubPhase path is not unique
//     within a run. The tree does not express repeats.
//   - CommanderUUIDWasChecked is deliberately left undeclared: it is announced only in
//     commander mode and only on the first attempt, so declaring it without a gate would make
//     every CLI destroy report a phase that cannot have run (index -1 in FindLastCompletedPhase).
func destroyNodes() []node {
	return []node{
		{Name: CreateStaticDestroyerNodeUserPhase, includeIf: ifStatic},
		{Name: WaitStaticDestroyerNodeUserPhase, includeIf: ifStatic},
		{Name: DeleteResourcesPhase},
		{Name: SetDeckhouseResourcesDeletedPhase},
		{Name: UpdateStaticDestroyerIPs, includeIf: ifStatic},
		{Name: AllNodesPhase},
		{Name: BaseInfraPhase, includeIf: ifCloud},
	}
}

func CommanderAttachPhases() []PhaseWithSubPhases { return project(commanderAttachNodes()) }

func commanderAttachNodes() []node {
	return []node{
		{Name: CommanderAttachScanPhase},
		{Name: CommanderAttachCapturePhase},
		{
			Name: CommanderAttachCheckPhase,
			Children: []node{
				{Name: CheckInfra},
				{Name: CheckConfiguration},
			},
		},
	}
}

func CommanderDetachPhases() []PhaseWithSubPhases { return project(commanderDetachNodes()) }

func commanderDetachNodes() []node {
	return []node{
		{
			Name: CommanderDetachCheckPhase,
			Children: []node{
				{Name: CheckInfra},
				{Name: CheckConfiguration},
			},
		},
		{Name: CommanderDetachDetachPhase},
	}
}

type phasesOpts struct {
	clusterConfig ClusterConfig
}

func operationNodes(operation Operation) []node {
	switch operation {
	case OperationBootstrap:
		return bootstrapNodes()
	case OperationConverge:
		return convergeNodes()
	case OperationCheck:
		return checkNodes()
	case OperationDestroy:
		return destroyNodes()
	case OperationCommanderAttach:
		return commanderAttachNodes()
	case OperationCommanderDetach:
		return commanderDetachNodes()
	}

	return nil
}

// operationPhases returns what the operation announces for the given options: the tree,
// gated, projected onto the wire format.
func operationPhases(operation Operation, opts phasesOpts) []PhaseWithSubPhases {
	return project(gate(operationNodes(operation), opts))
}

// PhasesFor returns the nodes the operation runs for the given cluster config, in order.
//
// This is the same resolution the progress tracker does for itself in SetClusterConfig, and it
// is exported so that the code executing an operation walks the very tree it reports: the gates
// decide what runs, not a second list kept in step by hand. Resolve it only once the cluster
// type is known - gates read it, and an unset type is not a neutral value.
func PhasesFor(operation Operation, cfg ClusterConfig) []PhaseWithSubPhases {
	return operationPhases(operation, phasesOpts{clusterConfig: cfg})
}

// gate drops the nodes whose includeIf says no, recursively - a node that is gated out takes
// its children with it.
func gate(nodes []node, opts phasesOpts) []node {
	included := make([]node, 0, len(nodes))

	for _, n := range nodes {
		if n.includeIf != nil && !n.includeIf(opts) {
			continue
		}

		n.Children = gate(n.Children, opts)
		included = append(included, n)
	}

	return included
}

// project flattens the tree into the two-level wire format: top-level nodes become phases,
// their children become sub-phases. Grandchildren are not represented and are not collapsed
// either (decision #11): the wire holds two levels, no node in the code has a third, and how
// to fold a deeper tree is itself a wire decision to settle with Commander. TestPhaseTreeDepth
// asserts that no such node exists, so this is a guaranteed-complete projection, not a lossy one.
func project(nodes []node) []PhaseWithSubPhases {
	phases := make([]PhaseWithSubPhases, 0, len(nodes))

	for _, n := range nodes {
		phase := PhaseWithSubPhases{Phase: n.Name}
		for _, child := range n.Children {
			phase.SubPhases = append(phase.SubPhases, child.Name)
		}

		phases = append(phases, phase)
	}

	return phases
}

// ifCloud includes the node on cloud clusters only.
//
// It tests for Cloud rather than for "not Static" on purpose. The cluster type starts out empty
// and is only filled in by SetClusterConfig, so a negative test reads an unset type as a cloud
// one and declares cloud-only nodes on paths that never learn what they are working with. Every
// caller that reaches cloud work resolves the type first (bootstrap Preparation, converge,
// destroy); the paths that never do keep their gates imperative instead - see the comment on
// CheckInfra in convergeNodes.
func ifCloud(opts phasesOpts) bool {
	return opts.clusterConfig.ClusterType == "Cloud"
}

func ifStatic(opts phasesOpts) bool {
	return opts.clusterConfig.ClusterType == "Static"
}

// ifHasClusterConfiguration includes the node only on a cluster whose control plane dhctl creates
// itself. Like ifCloud it is a positive test, and for a stronger reason: the flag is false both on
// a managed cluster and on a caller that never filled it in, so only an explicit true may declare
// the nodes that build a control plane.
func ifHasClusterConfiguration(opts phasesOpts) bool {
	return opts.clusterConfig.HasClusterConfiguration
}
