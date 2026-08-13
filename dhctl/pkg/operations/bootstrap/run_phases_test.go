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

package bootstrap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

// phaseEvent is one thing that happened during a walk: a node announced ("start"/"switch"), a
// node body executed ("run"), or the pipeline completed ("complete"). Announcements and bodies
// land in the same slice on purpose - their interleaving is half of what the walk guarantees.
type phaseEvent struct {
	kind     string
	phase    phases.OperationPhase
	critical bool
}

// recordingPEC is a phase context that records instead of doing. Only the four methods the walk
// is allowed to call are implemented; the rest panic, so a walk that reaches for InitPipeline or
// CompletePhase fails loudly rather than passing quietly.
type recordingPEC struct {
	events []phaseEvent
	// stopAt is the index of the announcement that reports the stop condition, -1 for none.
	stopAt int
}

func (p *recordingPEC) announce(kind string, phase phases.OperationPhase, isCritical bool) bool {
	announcements := len(p.phasesOf("start", "switch"))
	p.events = append(p.events, phaseEvent{kind: kind, phase: phase, critical: isCritical})
	return announcements == p.stopAt
}

func (p *recordingPEC) phasesOf(kinds ...string) []phases.OperationPhase {
	found := make([]phases.OperationPhase, 0, len(p.events))
	for _, e := range p.events {
		if slices.Contains(kinds, e.kind) {
			found = append(found, e.phase)
		}
	}
	return found
}

func (p *recordingPEC) StartPhase(_ context.Context, phase phases.OperationPhase, isCritical bool, _ state.Cache) (bool, error) {
	return p.announce("start", phase, isCritical), nil
}

func (p *recordingPEC) SwitchPhase(_ context.Context, phase phases.OperationPhase, isCritical bool, _ state.Cache, _ any) (bool, error) {
	return p.announce("switch", phase, isCritical), nil
}

func (p *recordingPEC) CompletePhaseAndPipeline(_ context.Context, _ state.Cache, _ any) error {
	p.events = append(p.events, phaseEvent{kind: "complete"})
	return nil
}

func (p *recordingPEC) InitPipeline(_ context.Context, _ state.Cache) error { panic("unexpected call") }
func (p *recordingPEC) Finalize(_ context.Context, _ state.Cache) error     { panic("unexpected call") }
func (p *recordingPEC) CompletePhase(_ context.Context, _ state.Cache, _ any) error {
	panic("unexpected call")
}
func (p *recordingPEC) CompletePipeline(_ context.Context, _ state.Cache) error {
	panic("unexpected call")
}
func (p *recordingPEC) CompleteSubPhase(_ context.Context, _ phases.OperationSubPhase) {
	panic("unexpected call")
}
func (p *recordingPEC) GetLastState() phases.DhctlState         { panic("unexpected call") }
func (p *recordingPEC) SetClusterConfig(_ phases.ClusterConfig) { panic("unexpected call") }

// recordingPhaseFuncs keeps the real critical flags and replaces the bodies with recorders, so
// the walk is driven by the map bootstrap actually ships - only without the filesystem and the
// cluster behind it.
func recordingPhaseFuncs(pec *recordingPEC, failOn phases.OperationPhase, failErr error) map[phases.OperationPhase]bootstrapPhase {
	funcs := make(map[phases.OperationPhase]bootstrapPhase)

	for name, phase := range (&ClusterBootstrapper{}).bootstrapPhaseFuncs() {
		funcs[name] = bootstrapPhase{
			critical: phase.critical,
			run: func(context.Context, *bootstrapContext) error {
				pec.events = append(pec.events, phaseEvent{kind: "run", phase: name})
				if name == failOn {
					return failErr
				}
				return nil
			},
		}
	}

	return funcs
}

func declaredPhases(clusterType string) []phases.OperationPhase {
	declared := make([]phases.OperationPhase, 0)
	for _, n := range phases.PhasesFor(phases.OperationBootstrap, phases.ClusterConfig{ClusterType: clusterType}) {
		declared = append(declared, n.Phase)
	}
	return declared
}

// runPhasesFixture builds the state Preparation would have left behind: the walk refuses to run
// any node past it against the DummyCache the process starts with, so the context carries a cache
// that keeps what is written to it.
func runPhasesFixture(clusterType string, stopAt int) (*ClusterBootstrapper, *bootstrapContext, *recordingPEC) {
	pec := &recordingPEC{stopAt: stopAt}
	return &ClusterBootstrapper{PhasedExecutionContext: pec},
		&bootstrapContext{
			metaConfig: &config.MetaConfig{ClusterType: clusterType},
			stateCache: utilcache.NewTestCache(),
		},
		pec
}

// TestRunPhases_WalksDeclaredTree pins the walk against the tree it claims to follow: every node
// the gates keep is announced once and run once, in tree order, announcement before body; only
// the first node starts the pipeline, and the pipeline is completed exactly once, after the last
// body. The critical flag is checked here too - it decides whether Commander may resume from the
// phase, and nothing else in the codebase asserts which two nodes carry it.
func TestRunPhases_WalksDeclaredTree(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		clusterType string
		critical    []phases.OperationPhase
	}{
		{
			clusterType: "Cloud",
			critical: []phases.OperationPhase{
				phases.PreparationPhase,
				phases.PreInfraPreflightsPhase,
				phases.InstallAdditionalMastersAndStaticNodes,
			},
		},
		{
			// The node carrying the third flag is cloud-only, so a static bootstrap has two
			// critical phases and not three. The control-plane-manager wait split out of it is
			// deliberately not critical: a critical announcement is a stop point for Commander,
			// and static bootstrap never had one there.
			clusterType: "Static",
			critical: []phases.OperationPhase{
				phases.PreparationPhase,
				phases.PreInfraPreflightsPhase,
			},
		},
	} {
		t.Run(tt.clusterType, func(t *testing.T) {
			t.Parallel()

			clusterType := tt.clusterType
			declared := declaredPhases(clusterType)
			require.Equal(t, phases.PreparationPhase, declared[0],
				"the walk starts on Preparation: it produces the cluster type the other gates read",
			)

			b, bctx, pec := runPhasesFixture(clusterType, -1)

			require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), wholeTree))

			require.Len(t, pec.events, 2*len(declared)+1)

			for i, name := range declared {
				announced := pec.events[2*i]
				require.Equal(t, name, announced.phase)
				if i == 0 {
					require.Equal(t, "start", announced.kind)
				} else {
					require.Equal(t, "switch", announced.kind)
				}
				require.Equal(t, phaseEvent{kind: "run", phase: name}, pec.events[2*i+1])
			}

			critical := make([]phases.OperationPhase, 0)
			for _, e := range pec.events {
				if e.critical {
					critical = append(critical, e.phase)
				}
			}
			require.Equal(t, tt.critical, critical)

			require.Equal(t, "complete", pec.events[len(pec.events)-1].kind)
		})
	}
}

// TestRunPhases_CloudOnlyNodesAreGatedOut pins what moving the hardcoded cluster-type branches
// into includeIf actually bought. The gate is now the only thing between a static cluster and the
// cloud-only work: bootstrapBaseInfra and bootstrapAdditionalNodes no longer check the type
// themselves, so a node announced on a static cluster would run cloud work there. Neither is
// announced and neither runs; the control-plane-manager wait, split out of the second one for
// exactly this reason, still runs on both.
func TestRunPhases_CloudOnlyNodesAreGatedOut(t *testing.T) {
	t.Parallel()

	cloudOnly := []phases.OperationPhase{
		phases.BaseInfraPhase,
		phases.FirstMasterPhase,
		phases.InstallAdditionalMastersAndStaticNodes,
	}

	t.Run("Static", func(t *testing.T) {
		t.Parallel()

		b, bctx, pec := runPhasesFixture("Static", -1)

		require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), wholeTree))

		for _, phase := range cloudOnly {
			require.NotContains(t, pec.phasesOf("start", "switch"), phase, "announced on a static cluster")
			require.NotContains(t, pec.phasesOf("run"), phase, "executed on a static cluster")
		}

		require.Contains(t, pec.phasesOf("run"), phases.WaitForControlPlaneManagerReadinessPhase)
	})

	t.Run("Cloud", func(t *testing.T) {
		t.Parallel()

		b, bctx, pec := runPhasesFixture("Cloud", -1)

		require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), wholeTree))

		for _, phase := range append(cloudOnly, phases.WaitForControlPlaneManagerReadinessPhase) {
			require.Contains(t, pec.phasesOf("start", "switch"), phase)
			require.Contains(t, pec.phasesOf("run"), phase)
		}
	})
}

// TestRunPhases_FirstMasterIsAnnouncedBetweenBaseInfraAndPostInfraPreflights pins the split of
// BaseInfra. Creating master-0 used to be a sub-phase, and CompleteSubPhase leaves CurrentPhase
// alone, so the work announced itself as BaseInfra: it was neither a stop point Commander could
// offer nor a name the x-unsafe short circuit could be keyed on. It is a phase now, and it sits
// between the infrastructure that produces its inputs and the preflights that follow it.
func TestRunPhases_FirstMasterIsAnnouncedBetweenBaseInfraAndPostInfraPreflights(t *testing.T) {
	t.Parallel()

	b, bctx, pec := runPhasesFixture("Cloud", -1)

	require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), wholeTree))

	announced := pec.phasesOf("start", "switch")
	at := slices.Index(announced, phases.BaseInfraPhase)
	require.NotEqual(t, -1, at)
	require.GreaterOrEqual(t, len(announced), at+3)
	require.Equal(t, []phases.OperationPhase{
		phases.BaseInfraPhase,
		phases.FirstMasterPhase,
		phases.PostInfraPreflightsPhase,
	}, announced[at:at+3])
}

// TestRunPhases_RestrictedToOneNode covers the standalone phase commands, base-infra being the
// first of them: the walk runs Preparation - the config it loads is what the gates are resolved
// against - then the one node it was given and nothing else, and still completes the pipeline.
func TestRunPhases_RestrictedToOneNode(t *testing.T) {
	t.Parallel()

	b, bctx, pec := runPhasesFixture("Cloud", -1)

	require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), phases.BaseInfraPhase))

	expected := []phases.OperationPhase{phases.PreparationPhase, phases.BaseInfraPhase}
	require.Equal(t, expected, pec.phasesOf("start", "switch"))
	require.Equal(t, expected, pec.phasesOf("run"))
	require.Equal(t, "complete", pec.events[len(pec.events)-1].kind)
}

// TestRunPhases_RestrictedToAGatedOutNodeIsRefused is why base-infra on a static cluster still
// exits non-zero after losing its own cluster-type check. The node is gated out there, and a walk
// restricted to a node that is not in the tree would otherwise announce nothing, do nothing and
// report success for infrastructure it never created.
func TestRunPhases_RestrictedToAGatedOutNodeIsRefused(t *testing.T) {
	t.Parallel()

	b, bctx, pec := runPhasesFixture("Static", -1)

	err := b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), phases.BaseInfraPhase)

	require.ErrorContains(t, err, string(phases.BaseInfraPhase))
	require.ErrorContains(t, err, "Static")
	require.Equal(t, []phases.OperationPhase{phases.PreparationPhase}, pec.phasesOf("run"))
	require.Empty(t, pec.phasesOf("complete"))
}

// TestRunPhases_RestrictionIsReadableFromInsidePreparation pins the half of that refusal the walk
// cannot do itself. The refusal needs the cluster type, and the cluster type is resolved halfway
// through Preparation - so the restriction has to be readable from inside the node, or the rest of
// it runs first: a static bootstrap is asked whether to run on the current host, and outside a
// terminal it is refused with a message naming an --ssh-host flag the standalone phase commands do
// not register, both on a run the gates were going to refuse anyway.
func TestRunPhases_RestrictionIsReadableFromInsidePreparation(t *testing.T) {
	t.Parallel()

	pec := &recordingPEC{stopAt: -1}
	b := &ClusterBootstrapper{PhasedExecutionContext: pec}

	err := b.runPhases(context.Background(), &bootstrapContext{}, map[phases.OperationPhase]bootstrapPhase{
		// Stands in for the point bootstrapPreparation reaches once the config is loaded: the
		// cluster type is known, and nothing past this line has run yet.
		phases.PreparationPhase: {run: func(_ context.Context, bctx *bootstrapContext) error {
			return refuseIfExcluded(bctx.only, "Static")
		}},
	}, phases.BaseInfraPhase)

	require.ErrorContains(t, err, string(phases.BaseInfraPhase))
	require.ErrorContains(t, err, "Static")
	require.Empty(t, pec.phasesOf("complete"))
}

// TestRunPhases_StopConditionHaltsWalk covers --skip-phase and the Commander stop condition: the
// node that reports the stop is announced but not run, nothing after it happens, and the pipeline
// is left uncompleted - completing it would report a run that was asked to stop as finished.
func TestRunPhases_StopConditionHaltsWalk(t *testing.T) {
	t.Parallel()

	declared := declaredPhases("Cloud")
	b, bctx, pec := runPhasesFixture("Cloud", 2)

	require.NoError(t, b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, "", nil), wholeTree))

	require.Equal(t, declared[:3], pec.phasesOf("start", "switch"))
	require.Equal(t, declared[:2], pec.phasesOf("run"))
	require.Empty(t, pec.phasesOf("complete"))
}

// TestRunPhases_AnnouncesPreparationBeforeTheStateCacheExists pins what the walk may announce
// Preparation against. The state cache is what Preparation creates, so bctx.stateCache is still
// nil when the phase is announced, and StartPhase reads the cache unconditionally to snapshot the
// state - against a nil one it panics inside ExtractDhctlState. The global cache is a DummyCache
// until then, which is why it is the cache the announcement uses.
//
// Driven by a real phase context on purpose: a recorder ignores the cache it is handed and would
// pass whatever is passed to it.
func TestRunPhases_AnnouncesPreparationBeforeTheStateCacheExists(t *testing.T) {
	t.Parallel()

	// Returned from the node, so the walk stops before resolving the tree - which reads the
	// cluster type this stand-in node never produces.
	halt := errors.New("preparation stopped the walk")

	b := &ClusterBootstrapper{
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, nil, nil),
	}

	err := b.runPhases(context.Background(), &bootstrapContext{}, map[phases.OperationPhase]bootstrapPhase{
		phases.PreparationPhase: {run: func(context.Context, *bootstrapContext) error { return halt }},
	}, wholeTree)

	require.Equal(t, halt, err)
}

// preparationPEC records what the Preparation node reports. It is the walk's recorder plus the
// two calls the node itself makes - a sub-phase completion and the cluster config - which the
// walk is not allowed to make and which recordingPEC therefore panics on.
type preparationPEC struct {
	recordingPEC
	completedSubPhases []phases.OperationSubPhase
	clusterConfig      phases.ClusterConfig
}

func (p *preparationPEC) CompleteSubPhase(_ context.Context, completed phases.OperationSubPhase) {
	p.completedSubPhases = append(p.completedSubPhases, completed)
}

func (p *preparationPEC) SetClusterConfig(cfg phases.ClusterConfig) { p.clusterConfig = cfg }

// TestBootstrapPreparation_ConfigFailureIsReportedInsidePreparation covers what the phase exists
// for: a bootstrap that dies parsing the config used to report no phase at all. Now the failure
// happens inside Preparation, after its first sub-phase and before ConfigValidation.
//
// The same run pins the first trap of the node's shape: the registry Stop handle must outlive the
// node - in Local mode it is the server holding the image bundle, and the tunnel to it is opened
// two phases later - so it is parked on the context instead of deferred here. The node also does
// not replace the phase context on this path, though that alone proves little here: the only
// replacement bootstrap has is the interactive one, and it is gated on a terminal this run does
// not have. Where that block lives is pinned by
// TestBootstrap_ReplacesThePhaseContextBeforeAnnouncingPreparation instead.
func TestBootstrapPreparation_ConfigFailureIsReportedInsidePreparation(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterType: Static\n",
	), 0o600))

	pec := &preparationPEC{}
	b := &ClusterBootstrapper{Params: &Params{Options: &options.Options{}}, PhasedExecutionContext: pec}
	b.Options.Global.ConfigPaths = []string{configPath}

	bctx := &bootstrapContext{}
	require.Error(t, b.bootstrapPreparation(context.Background(), bctx),
		"an incomplete ClusterConfiguration is rejected while the config is being validated",
	)

	require.Equal(t, []phases.OperationSubPhase{phases.PreparationSubPhaseImagesDownload}, pec.completedSubPhases)
	require.NotNil(t, bctx.registryStop, "the registry Stop handle must outlive the node that started it")
	require.Same(t, pec, b.PhasedExecutionContext, "the phase context must not be replaced inside the node")
}

// TestBootstrap_ReplacesThePhaseContextBeforeAnnouncingPreparation pins the second trap of the
// node's shape - the one only a terminal exposes. Interactive mode replaces the phase context, and
// a phase announced on the context about to be thrown away is reported as skipped by whatever
// replaces it, with the sub-phases already completed on it dropped. So the replacement belongs
// before the first announcement and not inside the node being announced.
//
// Not parallel: it swaps the terminal seam, and top-level parallel tests only resume once the
// sequential ones are done.
func TestBootstrap_ReplacesThePhaseContextBeforeAnnouncingPreparation(t *testing.T) {
	isTerminal = func() bool { return true }

	t.Cleanup(func() { isTerminal = input.IsTerminal })

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(
		"apiVersion: deckhouse.io/v1\nkind: ClusterConfiguration\nclusterType: Static\n",
	), 0o600))

	// The context bootstrap starts with, and the one interactive mode is expected to discard
	// untouched: it records rather than reports, so anything announced on it is visible.
	discarded := &recordingPEC{stopAt: -1}
	b := &ClusterBootstrapper{
		Params:                 &Params{Options: &options.Options{}},
		PhasedExecutionContext: discarded,
	}
	b.Options.Global.ConfigPaths = []string{configPath}

	announcedOn := make([]phases.DefaultPhasedExecutionContext, 0)
	announced := make([]phases.OperationPhase, 0)
	b.OnPhaseFunc = func(data phases.OnPhaseFuncData[phases.DefaultContextType]) error {
		announced = append(announced, data.NextPhase)
		announcedOn = append(announcedOn, b.PhasedExecutionContext)
		return nil
	}

	require.Error(t, b.Bootstrap(context.Background()),
		"the ClusterConfiguration is incomplete, so the walk stops inside Preparation",
	)

	require.Empty(t, discarded.events,
		"interactive mode discards the initial phase context: nothing may be announced on it",
	)
	require.Equal(t, []phases.OperationPhase{phases.PreparationPhase}, announced)
	require.Same(t, b.PhasedExecutionContext, announcedOn[0],
		"Preparation must be announced on the context that outlives it, not on one replaced afterwards",
	)
}

// TestRunPhases_PhaseErrorHaltsWalk asserts a failed node stops the walk with its own error
// unwrapped, and leaves the pipeline uncompleted.
func TestRunPhases_PhaseErrorHaltsWalk(t *testing.T) {
	t.Parallel()

	declared := declaredPhases("Cloud")
	b, bctx, pec := runPhasesFixture("Cloud", -1)
	failure := errors.New("phase failed")

	err := b.runPhases(context.Background(), bctx, recordingPhaseFuncs(pec, declared[2], failure), wholeTree)

	require.Equal(t, failure, err)
	require.Equal(t, declared[:3], pec.phasesOf("start", "switch"))
	require.Equal(t, declared[:3], pec.phasesOf("run"))
	require.Empty(t, pec.phasesOf("complete"))
}
