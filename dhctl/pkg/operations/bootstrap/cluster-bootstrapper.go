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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/name212/govalue"
	otattribute "go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	proto "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol"
	libcon "github.com/deckhouse/lib-connection/pkg"
	sshconfig "github.com/deckhouse/lib-connection/pkg/ssh/config"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructureprovider/cloud"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/resources"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/infrastructure/hook/controlplane"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/converge/lock"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/helper"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
	utilcache "github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
)

const (
	bootstrapAbortInvalidCacheMessage = `Create cache %s:
	Error: %v
	The Kubernetes cluster was probably bootstrapped successfully.
	Use the "dhctl destroy" command to delete the cluster.
`
	bootstrapAbortCheckMessage = `You will be asked for approval multiple times.
If you are confident in your actions, you can use the flag "--yes-i-am-sane-and-i-understand-what-i-am-doing" to skip approvals.
`
	cacheMessage = `Create cache %s:
	Error: %v

	The Kubernetes cluster was probably bootstrapped successfully.
	If you want to continue, please delete the cache folder manually.
`
)

// Params carries everything ClusterBootstrapper needs that is not derived from
// the cluster configuration files themselves. The Options field replaces the
// previous package-level dhctl/pkg/app globals; callers must populate it with a
// fresh *options.Options per operation to avoid sharing state between
// concurrent requests.
type Params struct {
	SSHProviderInitializer     *providerinitializer.SSHProviderInitializer
	KubeProvider               libcon.KubeProvider
	InitialState               phases.DhctlState
	ResetInitialState          bool
	DisableBootstrapClearCache bool
	OnPhaseFunc                phases.DefaultOnPhaseFunc
	OnProgressFunc             phases.OnProgressFunc
	CommanderMode              bool
	CommanderUUID              uuid.UUID
	InfrastructureContext      *infrastructure.Context

	TmpDir  string
	IsDebug bool

	// Options is the per-operation parsed configuration. Required.
	Options *options.Options

	*client.KubernetesInitParams
}

type ClusterBootstrapper struct {
	*Params
	PhasedExecutionContext phases.DefaultPhasedExecutionContext
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	lastState phases.DhctlState
}

func (b *ClusterBootstrapper) applyCommanderModeConfig(cfg *config.DeckhouseInstaller) {
	if b.CommanderMode {
		// FIXME(dhctl-for-commander): commander uuid currently optional, make it required later
		// if b.CommanderUUID == uuid.Nil {
		//	panic("CommanderUUID required for bootstrap operation in commander mode!")
		// }
		cfg.CommanderMode = b.CommanderMode
		cfg.CommanderUUID = b.CommanderUUID
	}
}

func (b *ClusterBootstrapper) commanderModeAction(action func() error, fallback func() error) error {
	if b.CommanderMode {
		if action != nil {
			return action()
		}
		return nil
	}
	if fallback != nil {
		return fallback()
	}
	return nil
}

func NewClusterBootstrapper(ctx context.Context, params *Params) *ClusterBootstrapper {
	if params.Options != nil && params.Options.Global.ProgressFilePath != "" {
		params.OnProgressFunc = phases.WriteProgress(params.Options.Global.ProgressFilePath)
	}

	return &ClusterBootstrapper{
		Params: params,
		PhasedExecutionContext: phases.NewDefaultPhasedExecutionContext(
			phases.OperationBootstrap, params.OnPhaseFunc, params.OnProgressFunc,
		),
		lastState: params.InitialState,
	}
}

func (b *ClusterBootstrapper) getCleanupFunc(ctx context.Context, metaConfig *config.MetaConfig) (func(), error) {
	if b.InfrastructureContext == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "InfrastructureContext is nil. Skipping cleanup.")
		return func() {}, nil
	}

	// A cluster whose control plane dhctl did not build never unpacks a cloud provider, so there is
	// nothing here to release. Asking for one anyway fails in GetFullUUID (config/meta.go): such a
	// cluster carries no UUID until InstallDeckhouse reads one out of the cluster itself.
	if !metaConfig.HasClusterConfiguration() {
		return func() {}, nil
	}

	provider, err := b.InfrastructureContext.CloudProviderGetter()(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	return func() {
		err = provider.Cleanup()
		if err != nil {
			dhlog.FromContext(ctx).ErrorContext(ctx, fmt.Sprintf("Cannot clean up provider: %v", err))
		}
	}, nil
}

type bootstrapContext struct {
	// only is the single node this run is restricted to, wholeTree for a full bootstrap. Set by
	// runPhases before the Preparation node runs, which is where it is read.
	only phases.OperationPhase
	// skipPhases are the --skip-phase targets, resolved and checked against the gates by the
	// Preparation node, and read by the walk that follows it.
	skipPhases              []phases.OperationPhase
	masterAddressesForSSH   map[string]string
	metaConfig              *config.MetaConfig
	stateCache              state.Cache
	configHash              string
	deckhouseInstallConfig  *config.DeckhouseInstaller
	bootstrapState          *State
	nodeIP                  string
	devicePath              string
	resourcesTemplateData   map[string]any
	resourcesToCreateBefore template.Resources
	resourcesToCreateAfter  template.Resources
	installDeckhouseResult  *InstallDeckhouseResult
	cleanup                 func()
	registryStop            func()
	finishProgress          func()
	preflightRunner         *preflight.Preflight
}

// cacheOrGlobal is the cache to report phase state against. Until Preparation has run
// cache.InitWithOptions, stateCache is nil and reporting against it panics inside
// ExtractDhctlState; the global cache is a DummyCache at that point, so it reports nothing and
// becomes the very same cache as stateCache once Preparation has installed it.
func (c *bootstrapContext) cacheOrGlobal() state.Cache {
	if c.stateCache != nil {
		return c.stateCache
	}

	return cache.Global()
}

// isTerminal is input.IsTerminal behind a variable so the interactive branch below can be
// exercised. It reads os.Stdin, which under go test is never a terminal, so without the seam the
// one ordering guarantee interactive mode has - the phase context is replaced before the first
// phase is announced - would be untestable and could regress silently.
var isTerminal = input.IsTerminal

func (b *ClusterBootstrapper) Bootstrap(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap")
	defer span.End()

	if b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Found post-bootstrap script: %s", b.Options.Bootstrap.PostBootstrapScriptPath))
		if err := ValidateScriptFile(ctx, b.Options.Bootstrap.PostBootstrapScriptPath); err != nil {
			return err
		}
	}

	if b.Options.Bootstrap.ResourcesPath != "" {
		dhlog.FromContext(ctx).WarnContext(ctx, "--resources flag is deprecated. Please use the --config flag multiple times for logical resource separation")
		b.Options.Global.ConfigPaths = append(b.Options.Global.ConfigPaths, b.Options.Bootstrap.ResourcesPath)
	}

	bctx := &bootstrapContext{
		masterAddressesForSSH: make(map[string]string),
	}

	// Interactive mode replaces the phase context, so it has to happen before the first phase is
	// announced: a phase announced on the context about to be thrown away is reported as skipped
	// by its replacement, and the sub-phases completed on it are dropped. Nothing here reads the
	// config, so nothing forces it any later than this.
	interactive := isTerminal() && !b.Options.Global.ShowProgress

	printBanner(ctx)

	if interactive {
		progressCh, finishProgress := phases.InitProgress(ctx, dhlog.FromContext(ctx), "Bootstrap cluster")
		bctx.finishProgress = finishProgress

		onUpdateFunc := func(progress phases.Progress) error {
			// Non-blocking: the pipeline's deferred Finalize can emit after the consumer has
			// stopped and the channel is no longer drained; never block or panic on it.
			select {
			case progressCh <- progress:
			default:
			}
			return nil
		}

		b.PhasedExecutionContext = phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, b.OnPhaseFunc, onUpdateFunc)
	}

	defer func() {
		if err := b.PhasedExecutionContext.Finalize(ctx, bctx.cacheOrGlobal()); err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("failed to finalize phased execution context: %v", err))
		}
		if bctx.finishProgress != nil {
			bctx.finishProgress()
		}
		if bctx.cleanup != nil {
			bctx.cleanup()
		}
		// The bundle registry outlives the Preparation node that started it: in Local mode the
		// reverse tunnel to it is opened two phases later, from RunBashiblePipeline.
		if bctx.registryStop != nil {
			bctx.registryStop()
		}
	}()

	err := b.runPhases(ctx, bctx, b.bootstrapPhaseFuncs(), wholeTree)

	if m := bctx.metaConfig; m != nil {
		// Cloud context on the bootstrap operation span (one span in both CLI and
		// gRPC paths), set once the config is loaded.
		span.SetAttributes(telemetry.CloudSpanAttributes(m.ClusterType, m.OriginalProviderName, m.Layout, m.ClusterPrefix, m.UUID)...)
	}

	return err
}

// bootstrapPhase is a top-level node of the bootstrap tree made runnable. The tree owns the
// order and the gates; this owns the body and whether the node is critical.
type bootstrapPhase struct {
	critical bool
	// validate reports whether the inputs the body dereferences are present, and names the phase
	// that produces every missing one. It belongs next to run rather than on the tree node because
	// what a node consumes is a property of the body: the tree is shared with converge and destroy
	// and knows nothing about bootstrapContext. Nodes that consume nothing leave it nil.
	validate func(ctx context.Context, bctx *bootstrapContext) error
	run      func(ctx context.Context, bctx *bootstrapContext) error
}

// bootstrapPhaseFuncs pairs every top-level node of the bootstrap tree with its function.
//
// The tree (pkg/operations/phases) stays the single source of truth for which nodes exist, in
// what order and under which gates; this map only says what each one does. Keying by name and
// not by position is what keeps the two from drifting apart: a declared node with no function
// here is a bug that runPhases reports, instead of a phase announced to Commander that nothing
// ever executes.
func (b *ClusterBootstrapper) bootstrapPhaseFuncs() map[phases.OperationPhase]bootstrapPhase {
	return map[phases.OperationPhase]bootstrapPhase{
		// Preparation carries no critical flag because nothing reads one: the flag marks an
		// announcement Commander may stop and resume from, and this is the one node announced
		// progress-only, so Commander is never told about it. Stopping before it would be
		// meaningless anyway - it produces the state cache a resume would have to read.
		phases.PreparationPhase:         {run: b.bootstrapPreparation},
		phases.PreInfraPreflightsPhase:  {critical: true, run: b.bootstrapPreflight},
		phases.BaseInfraPhase:           {run: b.bootstrapBaseInfra},
		phases.FirstMasterPhase:         {run: b.bootstrapFirstMaster},
		phases.PostInfraPreflightsPhase: {validate: b.validatePostInfraPreflightsInputs, run: b.bootstrapPostInfraPreflights},
		// Neither is critical: a critical announcement is a stop point Commander may resume from, and
		// both nodes are local, idempotent work that costs nothing to repeat - stopping there buys
		// the operator nothing the node before them does not already offer.
		phases.ParseResourcesPhase:                    {run: b.bootstrapParseResources},
		phases.WaitForSSHOnMasterPhase:                {run: b.bootstrapWaitForSSHOnMaster},
		phases.InstallKubernetesPhase:                 {validate: b.validateKubernetesInputs, run: b.bootstrapKubernetes},
		phases.InstallDeckhousePhase:                  {validate: b.validateDeckhouseInputs, run: b.bootstrapDeckhouse},
		phases.InstallAdditionalMastersAndStaticNodes: {critical: true, validate: b.validateKubeProviderInput, run: b.bootstrapAdditionalNodes},
		// Not critical, unlike the cloud-only node it was split out of: it is announced on every
		// cluster, and a critical announcement is a stop point for Commander, so marking it would
		// hand static bootstrap a stop point it never had.
		phases.WaitForControlPlaneManagerReadinessPhase: {validate: b.validateKubeProviderInput, run: b.bootstrapWaitControlPlaneManager},
		phases.CreateResourcesPhase:                     {validate: b.validateCreateResourcesInputs, run: b.bootstrapCreateResources},
		phases.ExecPostBootstrapPhase:                   {run: b.bootstrapPostBootstrap},
		phases.FinalizationPhase:                        {validate: b.validateFinalizeInputs, run: b.bootstrapFinalize},
	}
}

// wholeTree is the only value of runPhases' last parameter that runs a full bootstrap; any phase
// name restricts the walk to that single node, which is what the standalone phase commands are.
const wholeTree phases.OperationPhase = ""

// refuseIfExcluded refuses a phase a standalone phase command asked to run by name, but that the
// gates keep out of this particular run. Silence is the alternative worth avoiding: a command
// that announces nothing, does nothing and exits zero reports success for work it never did.
//
// Called from Preparation the moment the cluster type is known, and not only from the walk that
// follows it: everything the rest of Preparation does - the confirmation a static bootstrap asks
// for, the state cache, the cluster UUID - would otherwise happen on a run already destined to be
// refused, and would answer with the wrong message on top of that.
func refuseIfExcluded(only phases.OperationPhase, cfg phases.ClusterConfig) error {
	if only == wholeTree || !excludedFromBootstrap(only, cfg) {
		return nil
	}

	return fmt.Errorf("phase %q was requested explicitly, but it is excluded from the bootstrap of %s", only, clusterKind(cfg))
}

// clusterKind names the cluster the two refusals above and below were resolved against. A cluster
// dhctl did not create carries no ClusterConfiguration and therefore no type at all, and printing
// the empty string as one asks the user which "" cluster they meant.
func clusterKind(cfg phases.ClusterConfig) string {
	if cfg.ClusterType == "" {
		return "this cluster"
	}

	return fmt.Sprintf("a %q cluster", cfg.ClusterType)
}

// refuseIfSkipExcluded refuses a --skip-phase name the gates already keep out of this run. It
// says the opposite of the refusal above and has to: the user asked for the phase not to run, so
// telling them they requested it explicitly points them at the standalone phase commands they
// never invoked, and naming the flag is what makes the message actionable.
//
// Called from Preparation as well as from the walk, for the reason given above.
func refuseIfSkipExcluded(skip []phases.OperationPhase, cfg phases.ClusterConfig) error {
	for _, phase := range skip {
		if excludedFromBootstrap(phase, cfg) {
			return fmt.Errorf("--skip-phase %q names a phase that is not part of the bootstrap of %s", phase, clusterKind(cfg))
		}
	}

	return nil
}

func excludedFromBootstrap(phase phases.OperationPhase, cfg phases.ClusterConfig) bool {
	tree := phases.PhasesFor(phases.OperationBootstrap, cfg)

	return !slices.ContainsFunc(tree, func(n phases.PhaseWithSubPhases) bool { return n.Phase == phase })
}

// phaseClusterConfig is the one place the bootstrap package builds the gate inputs, so that the
// tree the walk resolves for itself cannot disagree with the one the progress tracker announces.
func phaseClusterConfig(metaConfig *config.MetaConfig) phases.ClusterConfig {
	return phases.ClusterConfig{
		ClusterType:             metaConfig.ClusterType,
		HasClusterConfiguration: metaConfig.HasClusterConfiguration(),
	}
}

// setControlPlaneInstallFlags follows the second gate axis, because both flags pin the Deckhouse
// Deployment to the control plane dhctl builds: a node-role.kubernetes.io/control-plane nodeSelector
// and the apiserver at the pod's own status.hostIP:6443 (manifests.ParameterizeDeckhouseDeployment).
// A cluster dhctl did not create carries neither that node label nor an apiserver on that address.
func setControlPlaneInstallFlags(installConfig *config.DeckhouseInstaller, metaConfig *config.MetaConfig) {
	installConfig.KubeadmBootstrap = metaConfig.HasClusterConfiguration()
	installConfig.MasterNodeSelector = metaConfig.HasClusterConfiguration()
}

// runPreparation runs the first node outside the walk below. It produces the cluster type every
// gate reads, so it cannot be selected by a tree that does not exist until it has run.
func (b *ClusterBootstrapper) runPreparation(ctx context.Context, bctx *bootstrapContext, funcs map[phases.OperationPhase]bootstrapPhase, only phases.OperationPhase) (bool, error) {
	preparation, ok := funcs[phases.PreparationPhase]
	if !ok {
		return false, fmt.Errorf("Internal error: bootstrap declares phase %q, but nothing implements it", phases.PreparationPhase)
	}

	// Progress only, unlike every other node: reporting a phase publishes a snapshot of the state
	// cache, and this node is what creates it - so reporting it hands Commander a state with no
	// cluster uuid in it, and Commander fails the bootstrap over that empty uuid before the node
	// that would have written it has run. The first phase Commander is told about stays
	// PreInfraPreflights, whose snapshot already carries the uuid, as it is on main.
	b.PhasedExecutionContext.StartPhaseProgressOnly(ctx, phases.PreparationPhase)

	// Read back by the node itself: it resolves the cluster type the restriction is checked
	// against, and refuses there rather than here, before the rest of it runs.
	bctx.only = only

	return false, preparation.run(ctx, bctx)
}

// runPhases walks the bootstrap tree: every node the gates keep is announced, run, and left
// completed by the announcement of the next one; the pipeline is completed once, after the last.
// Passing anything but wholeTree restricts the walk to that one node - Preparation still runs,
// because nothing downstream of it, the gates included, works without what it loads. The nodes
// named by --skip-phase are left out of the walk entirely, the way converge skips its own phases:
// a node that is never announced is reported skipped by the progress tracker on its own.
//
// Preparation is walked apart from the rest, and not by preference: the gates read the cluster
// type, and the cluster type is what Preparation produces. So the tree is not resolvable until
// its first node has run, and that first node is therefore ungated by construction. Everything
// the announcement itself needs - the phase context in its final, possibly interactive, form -
// is settled by Bootstrap before this is called.
func (b *ClusterBootstrapper) runPhases(ctx context.Context, bctx *bootstrapContext, funcs map[phases.OperationPhase]bootstrapPhase, only phases.OperationPhase) error {
	// Resolved before anything is announced, and not taken from the flag parser: the gRPC server
	// fills Options itself and never runs the PreAction that checks them, so for that path this
	// is the only check the values get.
	skip, err := phases.ResolveSkipPhases(phases.BootstrapPhases(), b.Options.Bootstrap.SkipPhases)
	if err != nil {
		return err
	}

	bctx.skipPhases = skip

	stop, err := b.runPreparation(ctx, bctx, funcs, only)
	if err != nil {
		return err
	}
	if stop {
		return nil
	}

	clusterConfig := phaseClusterConfig(bctx.metaConfig)

	tree := phases.PhasesFor(phases.OperationBootstrap, clusterConfig)
	if len(tree) == 0 || tree[0].Phase != phases.PreparationPhase {
		return fmt.Errorf("Internal error: the bootstrap tree must start with phase %q", phases.PreparationPhase)
	}

	if err := refuseIfExcluded(only, clusterConfig); err != nil {
		return err
	}

	if err := refuseIfSkipExcluded(bctx.skipPhases, clusterConfig); err != nil {
		return err
	}

	// requireSplitResources refuses this same pair of inputs, but only at the first consumer - on a
	// cloud cluster after BaseInfra and FirstMaster have provisioned. Both are known here, before
	// any node past Preparation has run, so the contradiction is answered before anything exists.
	if bctx.metaConfig.ResourcesYAML != "" && slices.Contains(bctx.skipPhases, phases.ParseResourcesPhase) {
		return fmt.Errorf(
			"--skip-phase %q leaves the configured resources with nothing to queue them: drop the flag or the resource document",
			phases.ParseResourcesPhase,
		)
	}

	// Preparation is already announced and open, so every remaining node closes its predecessor
	// with SwitchPhase and none of them starts the pipeline again.
	for _, declared := range tree[1:] {
		if only != wholeTree && declared.Phase != only {
			continue
		}

		if slices.Contains(bctx.skipPhases, declared.Phase) {
			continue
		}

		phase, ok := funcs[declared.Phase]
		if !ok {
			return fmt.Errorf("Internal error: bootstrap declares phase %q, but nothing implements it", declared.Phase)
		}

		stop, err := b.runDeclaredPhase(ctx, bctx, declared.Phase, phase)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}

	return b.PhasedExecutionContext.CompletePhaseAndPipeline(ctx, bctx.stateCache, nil)
}

// runDeclaredPhase announces one node of the walk, checks what it consumes and runs it. It
// reports whether the phase context asked the run to stop, which leaves the pipeline open on
// purpose: completing it would report a run that was asked to stop as finished.
func (b *ClusterBootstrapper) runDeclaredPhase(ctx context.Context, bctx *bootstrapContext, name phases.OperationPhase, phase bootstrapPhase) (bool, error) {
	shouldStop, err := b.PhasedExecutionContext.SwitchPhase(ctx, name, phase.critical, bctx.stateCache, nil)
	if err != nil {
		return false, err
	}
	if shouldStop {
		return true, nil
	}

	// Checked here and not in every node: the state cache is the one input all of them share,
	// and the exemption the check needs - Preparation produces it, so it may not demand it - is
	// exactly the split between this walk and the node run before it.
	if err := requireStateCache(bctx); err != nil {
		return false, fmt.Errorf("phase %q: %w", name, err)
	}

	if phase.validate != nil {
		if err := phase.validate(ctx, bctx); err != nil {
			return false, fmt.Errorf("phase %q: %w", name, err)
		}
	}

	return false, phase.run(ctx, bctx)
}

// requireStateCache rejects a run whose state cache is the DummyCache the process starts with.
// That cache accepts every write and returns nothing, so a node running against it rediscovers an
// empty world at every step and leaves nothing for the next process to resume from - a bootstrap
// that repeats infrastructure it has already created rather than one that fails.
func requireStateCache(bctx *bootstrapContext) error {
	_, dummy := bctx.stateCache.(*utilcache.DummyCache)
	if bctx.stateCache == nil || dummy {
		return fmt.Errorf("state cache is not initialized: phase %q produces it", phases.PreparationPhase)
	}

	return nil
}

// isCloudCluster also answers "is the cluster type known at all": the gates in the tree only ever
// let a cloud-only node through on a cloud cluster, and validate is called on a context whose
// metaConfig may be missing entirely.
func isCloudCluster(bctx *bootstrapContext) bool {
	return bctx.metaConfig != nil && bctx.metaConfig.ClusterType == config.CloudClusterType
}

// validateKubeProviderInput guards every node that opens a Kubernetes client. The provider has two
// different producers, which is why the message picks between them: on a cloud cluster it is built
// by FirstMaster around the master it has just created, on a static one it arrives ready-made in
// Params from the connection flags of the command line.
func (b *ClusterBootstrapper) validateKubeProviderInput(_ context.Context, bctx *bootstrapContext) error {
	if govalue.NotNil(b.KubeProvider) {
		return nil
	}

	producer := "the connection flags of the command line (--ssh-host, --kubeconfig)"
	if isCloudCluster(bctx) {
		producer = fmt.Sprintf("phase %q", phases.FirstMasterPhase)
	}

	return fmt.Errorf("kube provider is not initialized: %s produces it", producer)
}

// validatePostInfraPreflightsInputs checks the runner only. The other input of this node, the
// connection host it records as the first master, has no producer to name: a static cluster
// bootstrapped on the machine dhctl runs on legitimately has none, so the node guards the read by
// the length of the host list instead of demanding one.
func (b *ClusterBootstrapper) validatePostInfraPreflightsInputs(_ context.Context, bctx *bootstrapContext) error {
	if bctx.preflightRunner == nil {
		return fmt.Errorf("preflight runner is not initialized: phase %q produces it", phases.PreInfraPreflightsPhase)
	}

	return nil
}

// validateKubernetesInputs checks what bashible is pointed at. The node IP is cloud-only, and
// genuinely absent everywhere else: no cluster configuration carries one, so on a static cluster the
// bundle is rendered without it and bashible discovers the node's own IP on the node itself
// (RunBashiblePipeline refuses an empty discovered IP). The other input of the node, the kubernetes
// data device path, is not checked - an empty path is a normal outcome the bundle passes on to
// bashible, which then autodetects an unused disk.
func (b *ClusterBootstrapper) validateKubernetesInputs(_ context.Context, bctx *bootstrapContext) error {
	if !isCloudCluster(bctx) {
		return nil
	}

	if bctx.nodeIP == "" {
		return fmt.Errorf("master node IP is empty: phase %q produces it", phases.FirstMasterPhase)
	}

	return nil
}

func (b *ClusterBootstrapper) validateDeckhouseInputs(ctx context.Context, bctx *bootstrapContext) error {
	if err := b.validateKubeProviderInput(ctx, bctx); err != nil {
		return err
	}

	// An empty CloudDiscovery is not refused by the installer: it writes the discovery Secret
	// empty, and the cloud provider module then rolls out with nothing to discover.
	if isCloudCluster(bctx) && (bctx.deckhouseInstallConfig == nil || len(bctx.deckhouseInstallConfig.CloudDiscovery) == 0) {
		return fmt.Errorf("cloud discovery data is empty: phase %q produces it", phases.BaseInfraPhase)
	}

	return requireSplitResources(bctx)
}

func (b *ClusterBootstrapper) validateCreateResourcesInputs(ctx context.Context, bctx *bootstrapContext) error {
	if err := b.validateKubeProviderInput(ctx, bctx); err != nil {
		return err
	}

	if err := requireSplitResources(bctx); err != nil {
		return err
	}

	return requireInstallDeckhouseResult(bctx)
}

func (b *ClusterBootstrapper) validateFinalizeInputs(ctx context.Context, bctx *bootstrapContext) error {
	if err := b.validateKubeProviderInput(ctx, bctx); err != nil {
		return err
	}

	return requireInstallDeckhouseResult(bctx)
}

// requireSplitResources tells "the producer never ran" apart from "there is nothing to create".
// Empty queues are the normal state of a bootstrap without a resource document, so what is checked
// is not their length but the disagreement: a resource document that was never split.
func requireSplitResources(bctx *bootstrapContext) error {
	if bctx.metaConfig == nil || bctx.metaConfig.ResourcesYAML == "" {
		return nil
	}

	if len(bctx.resourcesToCreateBefore)+len(bctx.resourcesToCreateAfter) == 0 {
		return fmt.Errorf("resources are configured but none are queued for creation: phase %q produces the queues", phases.ParseResourcesPhase)
	}

	return nil
}

// requireInstallDeckhouseResult guards the two nodes that carry out what the installation left
// unfinished: without the result both silently do nothing at all, so the module configs that
// depend on Deckhouse being up are never applied and the bootstrap still reports success.
func requireInstallDeckhouseResult(bctx *bootstrapContext) error {
	if bctx.installDeckhouseResult == nil {
		return fmt.Errorf("deckhouse installation result is missing: phase %q produces it", phases.InstallDeckhousePhase)
	}

	return nil
}

// bootstrapPreparation is everything bootstrap does before it touches the cluster: fetch the
// images, parse and validate the config, open the state cache and the pipeline. It used to run
// unannounced, which is why a bootstrap that died here reported no phase at all.
func (b *ClusterBootstrapper) bootstrapPreparation(ctx context.Context, bctx *bootstrapContext) error {
	ctx, preparationSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.Preparation")
	defer preparationSpan.End()

	// Registry shoud run before LoadConfigFromFile
	registryStop, err := registry.InitFromConfig(
		ctx,
		dhlog.FromContext(ctx),
		b.Options.Global.ConfigPaths,
		b.Options.Registry.ImgBundlePath,
	)
	if err != nil {
		return err
	}
	// Parked on the context rather than stopped here: in Local mode this is the server holding
	// the image bundle, and the tunnel to it is opened two phases later, inside InstallKubernetes.
	// Bootstrap stops it when the whole operation is over.
	bctx.registryStop = registryStop

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.PreparationSubPhaseImagesDownload)

	// first, parse and check cluster config
	metaConfig, err := config.LoadConfigFromFile(
		ctx,
		b.Options.Global.ConfigPaths,
		infrastructureprovider.MetaConfigValidatorProvider(),
		&b.Options.Global,
		config.ValidateOptionValidateExtensions(true),
		config.ValidateOptionOperation(infrastructureprovider.DhctlOperationBootstrap),
	)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "MetaConfig was loaded")

	if err := config.ApplyCNIBootstrap(ctx, metaConfig, &b.Options.Global); err != nil {
		return fmt.Errorf("apply cni bootstrap: %w", err)
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.PreparationSubPhaseConfigValidation)

	clusterConfig := phaseClusterConfig(metaConfig)

	b.PhasedExecutionContext.SetClusterConfig(clusterConfig)

	if err := refuseIfExcluded(bctx.only, clusterConfig); err != nil {
		return err
	}

	if err := refuseIfSkipExcluded(bctx.skipPhases, clusterConfig); err != nil {
		return err
	}

	// Check if static cluster without ssh-host
	if metaConfig.IsStatic() && !b.SSHProviderInitializer.CheckHosts(ctx) {
		if input.IsTerminal() {
			confirmation := input.NewConfirmation().
				WithMessage("Do you really want to bootstrap the cluster on the current host?")
			if !confirmation.Ask() {
				return fmt.Errorf("Bootstrap canceled by user")
			}
		} else {
			return fmt.Errorf("Static cluster bootstrap requires --ssh-host option when not running in terminal. Please use --ssh-host option or pass --connection-config with SSHHost resource to bootstrap the cluster")
		}
	}

	providerGetter := infrastructureprovider.CloudProviderGetter(infrastructureprovider.CloudProviderGetterParams{
		TmpDir:           b.TmpDir,
		GlobalOptions:    &b.Options.Global,
		AdditionalParams: cloud.ProviderAdditionalParams{},
		IsDebug:          b.IsDebug,
	})

	b.InfrastructureContext = infrastructure.NewContextWithProvider(providerGetter).
		WithUseTfCache(b.Options.Cache.UseTfCache).
		WithDebug(b.Options.Global.IsDebug)

	// next init cache
	cachePath := cacheIdentity(ctx, metaConfig, connectionHosts(b.SSHProviderInitializer), b.Options.Kube, b.Options.Cache.Dir)
	if err = cache.InitWithOptions(ctx, cachePath, cache.CacheOptions{InitialState: b.InitialState, ResetInitialState: b.ResetInitialState, Cache: b.Options.Cache}); err != nil {
		// TODO: it's better to ask for confirmation here
		return fmt.Errorf(cacheMessage, cachePath, err)
	}

	stateCache := cache.Global()

	if b.Options.Cache.DropCache {
		stateCache.Clean(ctx)
		stateCache.Delete(ctx, state.TombstoneKey)
		dhlog.FromContext(ctx).DebugContext(ctx, "Cache was dropped")
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.PreparationSubPhaseCachePreparation)

	if err := b.PhasedExecutionContext.InitPipeline(ctx, stateCache); err != nil {
		return err
	}
	// TODO(dhctl-for-commander): pass stateCache externally using params as in Destroyer, this variable will be unneeded then
	b.lastState = nil

	configHash := state.ConfigHash(ctx, b.Options.Global.ConfigPaths)

	metaConfig.UUID, err = generateClusterUUID(ctx, stateCache, metaConfig)
	if err != nil {
		return err
	}

	// The cleanup releases the cloud provider this node has just built, so it belongs here rather
	// than in the preflight node it used to live in: a run restricted to a single phase leaves the
	// preflights out, and the provider root dir - the infrastructure utility and its plugins -
	// would then survive the process that unpacked it.
	cleanup, err := b.getCleanupFunc(ctx, metaConfig)
	if err != nil {
		return err
	}
	bctx.cleanup = cleanup

	metaConfig.ResourceManagementTimeout = b.Options.Cache.ResourceManagementTimeout

	deckhouseInstallConfig, err := config.PrepareDeckhouseInstallConfig(ctx, metaConfig, &b.Options.Global)
	if err != nil {
		return err
	}

	b.applyCommanderModeConfig(deckhouseInstallConfig)

	setControlPlaneInstallFlags(deckhouseInstallConfig, metaConfig)

	bootstrapState := NewBootstrapState(stateCache)

	bctx.metaConfig = metaConfig
	bctx.stateCache = stateCache
	bctx.configHash = configHash
	bctx.deckhouseInstallConfig = deckhouseInstallConfig
	bctx.bootstrapState = bootstrapState

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.PreparationSubPhaseStatePreparation)

	return nil
}

func (b *ClusterBootstrapper) bootstrapPreflight(ctx context.Context, bctx *bootstrapContext) error {
	ctx, preflightSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.PreInfraPreflights")
	defer preflightSpan.End()

	checkSuites, err := b.preflightSuites(ctx, bctx)
	if err != nil {
		return err
	}

	// The single assignment of the runner: PostInfraPreflights runs it again for its own phase
	// (validatePostInfraPreflightsInputs), so a selection branch that builds one and forgets to
	// park it here nil-derefs a node later.
	preflightRunner := preflight.New(checkSuites...)
	preflightRunner.UseCache(bctx.bootstrapState)
	preflightRunner.SetCacheSalt(bctx.configHash)
	preflightRunner.DisableChecks(b.Options.Preflight.DisabledChecks()...)
	bctx.preflightRunner = preflightRunner

	return preflightRunner.Run(ctx, preflight.PhasePreInfra)
}

// preflightSuites picks the checks by what dhctl is about to build, which is why the second axis is
// asked first: a cluster with no ClusterConfiguration has a control plane dhctl did not create, so
// there is no infrastructure to check the parameters of and no node of ours to reach over SSH, and
// the global suite is all that applies. The static suite is not a safe default there - it is built
// over the node interface helper.GetNodeInterface hands back when no SSH provider can be made,
// which is the installer container itself, and its checks would create users and probe sudo in it.
func (b *ClusterBootstrapper) preflightSuites(ctx context.Context, bctx *bootstrapContext) ([]preflight.Suite, error) {
	globalSuite := suites.NewGlobalSuite(suites.GlobalDeps{
		MetaConfig:    bctx.metaConfig,
		InstallConfig: bctx.deckhouseInstallConfig,
		BuildInfo:     b.Options.BuildInfo,
	})

	if !bctx.metaConfig.HasClusterConfiguration() {
		return []preflight.Suite{globalSuite}, nil
	}

	if bctx.metaConfig.ClusterType != config.CloudClusterType {
		staticSuite, err := suites.NewStaticSuite(suites.StaticDeps{
			SSHProviderInitializer: b.SSHProviderInitializer,
			MetaConfig:             bctx.metaConfig,
			InstallConfig:          bctx.deckhouseInstallConfig,
			LegacyMode:             b.SSHProviderInitializer.IsLegacyMode(),
			GlobalOpts:             &b.Options.Global,
		}, ctx)
		if err != nil {
			return nil, err
		}

		return []preflight.Suite{globalSuite, staticSuite}, nil
	}

	sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil && !errors.Is(err, providerinitializer.ErrHostsFromCacheNotFound) {
		return nil, err
	}

	cloudSuite := suites.NewCloudSuite(suites.CloudDeps{
		InstallConfig:          bctx.deckhouseInstallConfig,
		MetaConfig:             bctx.metaConfig,
		SSHProviderInitializer: b.SSHProviderInitializer,
	})
	postCloudSuite := suites.NewPostCloudSuite(suites.PostCloudDeps{
		MetaConfig:  bctx.metaConfig,
		SSHProvider: sshProvider,
		LegacyMode:  b.SSHProviderInitializer.IsLegacyMode(),
	})

	return []preflight.Suite{globalSuite, cloudSuite, postCloudSuite}, nil
}

func (b *ClusterBootstrapper) bootstrapBaseInfra(ctx context.Context, bctx *bootstrapContext) error {
	ctx, baseInfraSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.BaseInfra")
	defer baseInfraSpan.End()

	// No cluster-type check here: the node is gated by includeIf in the tree, so on a static
	// cluster the walk never calls this at all.
	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Cloud infrastructure", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.CloudInfra")
		defer span.End()

		baseRunner, err := b.InfrastructureContext.GetBootstrapBaseInfraRunner(ctx, bctx.metaConfig, bctx.stateCache)
		if err != nil {
			return err
		}

		baseOutputs, err := infrastructure.ApplyPipeline(ctx, baseRunner, "Kubernetes cluster", &b.Options.Global, infrastructure.GetBaseInfraResult)
		if err != nil {
			return err
		}

		dhlog.FromContext(ctx).DebugContext(ctx, "Base infrastructure was created")
		b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.BaseInfraSubPhaseBaseInfra)

		var cloudDiscoveryData map[string]any
		err = json.Unmarshal(baseOutputs.CloudDiscovery, &cloudDiscoveryData)
		if err != nil {
			return err
		}

		bctx.resourcesTemplateData = map[string]any{
			"cloudDiscovery": cloudDiscoveryData,
		}

		bctx.deckhouseInstallConfig.CloudDiscovery = baseOutputs.CloudDiscovery
		bctx.deckhouseInstallConfig.InfrastructureState = baseOutputs.InfrastructureState

		if baseOutputs.BastionHost != "" {
			// The bastion stays here because baseOutputs does: it is an output of this node, not of
			// the next one. Writing it through is safe because GetConfig hands out the initializer's
			// own config pointer, so this is the very object FirstMaster passes to Reinitialize.
			connectionConfig := b.SSHProviderInitializer.GetConfig()
			connectionConfig.Config.BastionHost = baseOutputs.BastionHost

			if err := SaveBastionHostToCache(ctx, bctx.stateCache, baseOutputs.BastionHost); err != nil {
				dhlog.FromContext(ctx).WarnContext(ctx, fmt.Sprintf("Cannot save bastion host to cache %v", err))
			}
		}

		return nil
	})
}

// bootstrapFirstMaster creates master-0 and rebuilds the SSH and Kubernetes providers around it.
//
// It is a node of its own rather than the tail of BaseInfra because it is the last point at which
// nothing but cloud infrastructure exists. Reported as a sub-phase it was invisible as a stop
// point - CompleteSubPhase leaves CurrentPhase alone, so master creation announced itself as
// BaseInfra - and its infrastructure spans hung off the base-infrastructure span next door.
func (b *ClusterBootstrapper) bootstrapFirstMaster(ctx context.Context, bctx *bootstrapContext) error {
	ctx, firstMasterSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.FirstMaster")
	defer firstMasterSpan.End()

	masterNodeName := fmt.Sprintf("%s-master-0", bctx.metaConfig.ClusterPrefix)
	masterRunner, err := b.Params.InfrastructureContext.GetBootstrapNodeRunner(ctx, bctx.metaConfig, bctx.stateCache, infrastructure.BootstrapNodeRunnerOptions{
		NodeName:        masterNodeName,
		NodeGroupStep:   infrastructure.MasterNodeStep,
		NodeGroupName:   "master",
		NodeIndex:       0,
		NodeCloudConfig: "",
	})
	if err != nil {
		return err
	}

	masterOutputs, err := infrastructure.ApplyPipeline(ctx, masterRunner, masterNodeName, &b.Options.Global, infrastructure.GetMasterNodeResult)
	if err != nil {
		return err
	}

	dhlog.FromContext(ctx).DebugContext(ctx, "First control-plane node was created")

	// providers should be reinitialized here
	baseSettings := b.SSHProviderInitializer.GetSettings()
	connectionConfig := b.SSHProviderInitializer.GetConfig()
	connectionConfig.Hosts = append(connectionConfig.Hosts, sshconfig.Host{Host: masterOutputs.MasterIPForSSH})

	b.SSHProviderInitializer.Reinitialize(
		ctx,
		baseSettings,
		connectionConfig,
	)
	b.KubeProvider = b.SSHProviderInitializer.GetKubeProvider(ctx)

	bctx.nodeIP = masterOutputs.NodeInternalIP
	bctx.devicePath = masterOutputs.KubeDataDevicePath

	bctx.deckhouseInstallConfig.NodesInfrastructureState = make(map[string][]byte)
	bctx.deckhouseInstallConfig.NodesInfrastructureState[masterNodeName] = masterOutputs.InfrastructureState

	bctx.masterAddressesForSSH[masterNodeName] = masterOutputs.MasterIPForSSH
	state.SaveMasterHostsToCache(ctx, bctx.stateCache, bctx.masterAddressesForSSH)

	interactive := isTerminal() && !b.Options.Global.ShowProgress
	if interactive {
		sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
		if err != nil {
			return err
		}
		sshClient, err := sshProvider.Client(ctx)
		if err != nil {
			return err
		}
		sshString := sshClient.Session().String()
		dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("First master connection string: %s", sshString))
	}

	return nil
}

// bootstrapPostInfraPreflights runs the post-infrastructure checks and, on a static cluster,
// records the connection they were run over. The preflight run is unconditional: it used to sit
// identically in both arms of a cluster-type branch, and the runner it consumes already carries the
// suites that differ between cloud and static (bootstrapPreflight).
func (b *ClusterBootstrapper) bootstrapPostInfraPreflights(ctx context.Context, bctx *bootstrapContext) error {
	if err := bctx.preflightRunner.Run(ctx, preflight.PhasePostInfra); err != nil {
		return err
	}

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		return nil
	}

	return saveStaticConnectionToCache(ctx, bctx.stateCache, b.SSHProviderInitializer.GetConfig())
}

// bootstrapParseResources renders the resource document and splits it into what is created before
// Deckhouse and what is created after. It is a node of its own rather than the tail of the
// preflight node it was split out of because it is the sole producer of both queues
// (requireSplitResources): folded into a node that can be left out of a run, it would take the
// user's resources with it, and createResources reports an empty queue as success.
func (b *ClusterBootstrapper) bootstrapParseResources(ctx context.Context, bctx *bootstrapContext) error {
	if bctx.metaConfig.ResourcesYAML == "" {
		return nil
	}

	parsedResources, err := template.ParseResourcesContent(ctx, bctx.metaConfig.ResourcesYAML, bctx.resourcesTemplateData)
	if err != nil {
		return err
	}

	bctx.resourcesToCreateBefore, bctx.resourcesToCreateAfter = splitResourcesOnPreAndPostDeckhouseInstall(ctx, parsedResources)

	return nil
}

// bootstrapWaitForSSHOnMaster waits until the master answers SSH, which is what InstallKubernetes
// then runs the bashible bundle over. A bootstrap with no connection hosts skips it: a static
// cluster bootstrapped on the machine dhctl runs on has no master to wait for.
func (b *ClusterBootstrapper) bootstrapWaitForSSHOnMaster(ctx context.Context, _ *bootstrapContext) error {
	if !b.SSHProviderInitializer.CheckHosts(ctx) {
		return nil
	}

	sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
	if err != nil {
		return err
	}

	sshClient, err := sshProvider.Client(ctx)
	if err != nil {
		return err
	}

	if err := WaitForSSHConnectionOnMaster(ctx, sshClient); err != nil {
		return fmt.Errorf("failed to wait for SSH connection on master: %w", err)
	}

	return nil
}

// saveStaticConnectionToCache records the connection a static bootstrap was given, which is the only
// record of it there is: nothing on a static cluster discovers master hosts the way the cloud flow
// does, so abort has nowhere else to learn which hosts this bootstrap touched. Every host goes in,
// not just the first one - the ones left out keep a half-installed control plane running and never
// get cleaned up. A failed write is returned rather than logged for the same reason.
//
// Guarded by the length of the host slice and not by CheckHosts: CheckHosts is also true when the
// hosts were found in the state cache and never landed in the connection config
// (providerinitializer lateHosts). There is nothing to record in that case anyway - the cache is
// where this writes to.
func saveStaticConnectionToCache(ctx context.Context, stateCache state.Cache, connectionConfig *sshconfig.ConnectionConfig) error {
	if connectionConfig == nil || len(connectionConfig.Hosts) == 0 {
		return nil
	}

	if connectionConfig.Config.BastionHost != "" {
		if err := SaveBastionHostToCache(ctx, stateCache, connectionConfig.Config.BastionHost); err != nil {
			return fmt.Errorf("save bastion host to cache: %w", err)
		}
	}

	hosts := make(map[string]string, len(connectionConfig.Hosts))

	for i, host := range connectionConfig.Hosts {
		name := fmt.Sprintf("master-%d", i)
		if i == 0 {
			name = "first-master"
		}

		hosts[name] = host.Host
	}

	if err := state.SaveMasterHosts(ctx, stateCache, hosts); err != nil {
		return fmt.Errorf("save master hosts to cache: %w", err)
	}

	return nil
}

// bashibleNodeInterface picks the node the bundle is executed on, without the silent fall back to
// the local one.
//
// helper.GetNodeInterface returns the local node interface whenever the SSH provider cannot be
// built, which is correct only where local execution was actually asked for: a static cluster with
// no connection hosts, bootstrapped on the machine dhctl runs on after the confirmation in
// Preparation. Everywhere else that fallback installs the whole Kubernetes bundle inside the
// installer container instead of on the master, and reports it as a successful bootstrap. The gate
// is here rather than in the helper because every other call site of the helper is a preflight
// check, where running locally produces a check result, not a cluster built in the wrong place.
func (b *ClusterBootstrapper) bashibleNodeInterface(ctx context.Context, bctx *bootstrapContext) (libcon.Interface, error) {
	if isCloudCluster(bctx) || b.SSHProviderInitializer.CheckHosts(ctx) {
		if _, err := b.SSHProviderInitializer.GetSSHProvider(ctx); err != nil {
			return nil, fmt.Errorf("get ssh provider: %w", err)
		}
	}

	return helper.GetNodeInterface(ctx, b.SSHProviderInitializer, b.SSHProviderInitializer.GetSettings())
}

func (b *ClusterBootstrapper) bootstrapKubernetes(ctx context.Context, bctx *bootstrapContext) error {
	ctx, bashibleBundleSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.BashibleBundle")
	defer bashibleBundleSpan.End()

	nodeInterface, err := b.bashibleNodeInterface(ctx, bctx)
	if err != nil {
		return fmt.Errorf("Could not get NodeInterface: %w", err)
	}

	// Taking the method value below dereferences the interface, so a nil one panics here rather
	// than reaching the guard in BashiblePipelineParams.Validate that exists for exactly this.
	pec := b.PhasedExecutionContext
	if govalue.IsNil(pec) {
		return fmt.Errorf("Internal error: ClusterBootstrapper PhasedExecutionContext is nil")
	}

	err = RunBashiblePipeline(ctx, &BashiblePipelineParams{
		Node:             nodeInterface,
		NodeIP:           bctx.nodeIP,
		DevicePath:       bctx.devicePath,
		MetaConfig:       bctx.metaConfig,
		CommanderMode:    b.CommanderMode,
		GlobalOpts:       &b.Options.Global,
		CompleteSubPhase: pec.CompleteSubPhase,
	})

	if err != nil {
		return err
	}

	bashibleBundleSpan.End()

	return nil
}

func (b *ClusterBootstrapper) bootstrapDeckhouse(ctx context.Context, bctx *bootstrapContext) error {
	ctx, installDeckhouseSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.InstallDeckhouse")
	defer installDeckhouseSpan.End()

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseConnect)

	installParams := InstallDeckhouseParams{
		BeforeDeckhouseTask: func() error {
			return createResources(
				ctx,
				&client.KubernetesClient{KubeClient: kubeCl},
				bctx.resourcesToCreateBefore,
				nil,
				true,
				b.Options.Bootstrap.ResourcesTimeout,
			)
		},
		DeckhouseTimeout: b.Options.Bootstrap.DeckhouseTimeout,
	}

	installDeckhouseResult, err := InstallDeckhouse(ctx, &client.KubernetesClient{KubeClient: kubeCl}, bctx.deckhouseInstallConfig, installParams)
	if err != nil {
		return err
	}
	bctx.installDeckhouseResult = installDeckhouseResult

	// Preparation mints no UUID for a cluster dhctl did not create, so the cache holds none, and
	// commander.ParseMetaConfig reads exactly this key on every later operation. InstallDeckhouse
	// has just settled the real one against kube-system/d8-cluster-uuid, which is the earliest
	// point a value here is the cluster's own rather than whatever the run arrived carrying.
	if !bctx.metaConfig.HasClusterConfiguration() {
		if err := bctx.stateCache.Save(ctx, "uuid", []byte(bctx.deckhouseInstallConfig.UUID)); err != nil {
			return fmt.Errorf("save cluster uuid to the state cache: %w", err)
		}
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseInstall)

	// Paired with the gate on the sub-phase: announcing a wait that cannot happen is what the
	// second axis is for, and completing a sub-phase the tree does not declare warns instead.
	if !bctx.metaConfig.HasClusterConfiguration() {
		return nil
	}

	if err := WaitForFirstMasterNodeBecomeReady(ctx, &client.KubernetesClient{KubeClient: kubeCl}); err != nil {
		return err
	}

	b.PhasedExecutionContext.CompleteSubPhase(ctx, phases.InstallDeckhouseSubPhaseWait)

	return nil
}

// bootstrapAdditionalNodes creates the remaining masters and the cloud-permanent nodes. It is
// cloud-only work, and the cluster-type check that used to wrap it is now the includeIf on its
// node: on a static cluster the walk does not reach this function at all.
func (b *ClusterBootstrapper) bootstrapAdditionalNodes(ctx context.Context, bctx *bootstrapContext) error {
	ctx, additionalNodesSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodes")
	defer additionalNodesSpan.End()

	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	localBootstraper := func(action func() error) error {
		if b.CommanderMode {
			return action()
		}

		return lock.NewInLockLocalRunner(
			ctx,
			kubernetes.NewSimpleKubeClientGetter(&client.KubernetesClient{KubeClient: kubeCl}),
			"local-bootstraper",
			b.Options.SSH.User,
		).Run(ctx, action)
	}

	return localBootstraper(func() error {
		return bootstrapAdditionalNodesForCloudCluster(
			ctx,
			&client.KubernetesClient{KubeClient: kubeCl},
			bctx.metaConfig,
			bctx.masterAddressesForSSH,
			b.InfrastructureContext,
			&b.Options.Global,
			b.PhasedExecutionContext,
		)
	})
}

// bootstrapWaitControlPlaneManager waits for control-plane-manager to become ready on every
// master. It is a node of its own rather than the tail of bootstrapAdditionalNodes because it runs
// on every cluster while that node is cloud-only: as a sub-phase it was completed on static
// clusters against a phase that had never started.
func (b *ClusterBootstrapper) bootstrapWaitControlPlaneManager(ctx context.Context, _ *bootstrapContext) error {
	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	return controlplane.
		NewManagerReadinessChecker(kubernetes.NewSimpleKubeClientGetter(&client.KubernetesClient{KubeClient: kubeCl})).
		IsReadyAll(ctx)
}

func (b *ClusterBootstrapper) bootstrapCreateResources(ctx context.Context, bctx *bootstrapContext) error {
	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	err = createResources(
		ctx,
		&client.KubernetesClient{KubeClient: kubeCl},
		bctx.resourcesToCreateAfter,
		bctx.installDeckhouseResult,
		false,
		b.Options.Bootstrap.ResourcesTimeout,
	)
	if err != nil {
		return err
	}

	return nil
}
func (b *ClusterBootstrapper) bootstrapPostBootstrap(ctx context.Context, bctx *bootstrapContext) error {
	if b.SSHProviderInitializer.CheckHosts(ctx) && b.Options.Bootstrap.PostBootstrapScriptPath != "" {
		ctx, postBootstrapSpan := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.PostBootstrap")
		defer postBootstrapSpan.End()

		postScriptExecutor := NewPostBootstrapScriptExecutor(b.SSHProviderInitializer, b.Options.Bootstrap.PostBootstrapScriptPath, bctx.bootstrapState).
			WithTimeout(b.Options.Bootstrap.PostBootstrapScriptTimeout)

		if err := postScriptExecutor.Execute(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (b *ClusterBootstrapper) bootstrapFinalize(ctx context.Context, bctx *bootstrapContext) error {
	kubeCl, err := b.KubeProvider.Client(ctx)
	if err != nil {
		return err
	}

	if err := RunPostInstallTasks(ctx, &client.KubernetesClient{KubeClient: kubeCl}, bctx.installDeckhouseResult); err != nil {
		return err
	}

	if !b.DisableBootstrapClearCache {
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Clear cache", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.ClearCache")
			defer span.End()

			cache.Global().CleanWithExceptions(
				ctx,
				state.MasterHostsCacheKey,
				BastionHostCacheKey,
				PostBootstrapResultCacheKey,
			)
			dhlog.FromContext(ctx).WarnContext(ctx, `Next run of "dhctl bootstrap" will create a new Kubernetes cluster.`)

			return nil
		})
	}

	dhlog.FromContext(ctx).InfoContext(ctx, "Deckhouse cluster created successfully!", dhlog.ShowInCompacted())

	if bctx.metaConfig.ClusterType == config.CloudClusterType {
		_ = dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Kubernetes Master Node addresses for SSH", func(ctx context.Context) error {
			ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.KubernetesMasterNodeAddressesForSSH")
			defer span.End()

			sshProvider, err := b.SSHProviderInitializer.GetSSHProvider(ctx)
			if err != nil {
				return err
			}

			sshClient, err := sshProvider.Client(ctx)
			if err != nil {
				return err
			}
			for nodeName, address := range bctx.masterAddressesForSSH {
				fakeSession := sshClient.Session().Copy()
				fakeSession.SetAvailableHosts([]session.Host{{Host: address, Name: nodeName}})
				dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("%s | %s", nodeName, fakeSession.String()), dhlog.ShowInCompacted())
			}

			return nil
		})
	}

	return nil
}

func (b *ClusterBootstrapper) GetLastState() phases.DhctlState {
	if b.lastState != nil {
		return b.lastState
	}

	return b.PhasedExecutionContext.GetLastState()
}

func printBanner(ctx context.Context) {
	dhlog.PrintBanner(ctx)
}

// generateClusterUUID returns the UUID dhctl owns for this bootstrap, minting one on the first run
// and reusing the cached one on every next. A cluster whose control plane dhctl did not create
// gets none - see resolveClusterUUID, which owns that rule.
func generateClusterUUID(ctx context.Context, stateCache state.Cache, m *config.MetaConfig) (string, error) {
	if !m.HasClusterConfiguration() {
		return "", nil
	}

	var clusterUUID string

	return clusterUUID, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Cluster UUID", func(ctx context.Context) error {
		ok, err := stateCache.InCache(ctx, "uuid")
		if err != nil {
			return err
		}

		if !ok {
			genClusterUUID, err := uuid.NewRandom()
			if err != nil {
				return fmt.Errorf("can't create cluster UUID: %w", err)
			}

			clusterUUID = genClusterUUID.String()
			err = stateCache.Save(ctx, "uuid", []byte(clusterUUID))
			if err != nil {
				return err
			}
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Generated cluster UUID: %s", clusterUUID))
		} else {
			clusterUUIDBytes, err := stateCache.Load(ctx, "uuid")
			if err != nil {
				return err
			}
			clusterUUID = string(clusterUUIDBytes)
			dhlog.FromContext(ctx).InfoContext(ctx, fmt.Sprintf("Cluster UUID from cache: %s", clusterUUID))
		}
		return nil
	})
}

func bootstrapAdditionalNodesForCloudCluster(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	metaConfig *config.MetaConfig,
	masterAddressesForSSH map[string]string,
	infrastructureContext *infrastructure.Context,
	globalOptions *options.GlobalOptions,
	pec phases.DefaultPhasedExecutionContext,
) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodesForCloudCluster")
	defer span.End()

	if err := BootstrapAdditionalMasterNodes(ctx, kubeCl, metaConfig, masterAddressesForSSH, infrastructureContext, cache.Global(), globalOptions); err != nil {
		return err
	}

	terraNodeGroups := metaConfig.GetTerraNodeGroups()
	bootstrapAdditionalTerraNodeGroups := BootstrapTerraNodes
	if operations.IsSequentialNodesBootstrap(metaConfig) {
		bootstrapAdditionalTerraNodeGroups = operations.BootstrapSequentialTerraNodes
	}

	pec.CompleteSubPhase(ctx, phases.InstallAdditionalMastersAndStaticNodesSubPhaseAdditionalMasters)

	if err := bootstrapAdditionalTerraNodeGroups(ctx, kubeCl, metaConfig, terraNodeGroups, infrastructureContext, globalOptions); err != nil {
		return err
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Waiting for node groups to become ready", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.AdditionalNodesForCloudCluster.WaitForNodesBecomeReady")
		defer span.End()

		ngs := map[string]int{"master": metaConfig.MasterNodeGroupSpec.Replicas}
		for _, ng := range terraNodeGroups {
			if ng.Replicas > 0 {
				ngs[ng.Name] = ng.Replicas
			}
		}
		if err := entity.WaitForNodesBecomeReady(ctx, kubeCl, ngs); err != nil {
			return err
		}

		pec.CompleteSubPhase(ctx, phases.InstallAdditionalMastersAndStaticNodeSubPhaseStaticNodes)

		return nil
	})
}

func splitResourcesOnPreAndPostDeckhouseInstall(ctx context.Context, resourcesToCreate template.Resources) (template.Resources, template.Resources) {
	before := make(template.Resources, 0, len(resourcesToCreate))
	after := make(template.Resources, 0, len(resourcesToCreate))

	for _, resource := range resourcesToCreate {
		annotations := resource.Object.GetAnnotations()
		hasBeforeAnnotation := annotations != nil && annotations["dhctl.deckhouse.io/bootstrap-resource-place"] == "before-deckhouse"

		if hasBeforeAnnotation || isCloudProviderCredentialSecret(resource) {
			dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Add resource %s - %s to after queue", resource.String(), resource.Object.GetName()))
			before = append(before, resource)
			continue
		}

		dhlog.FromContext(ctx).DebugContext(ctx, fmt.Sprintf("Add resource %s - %s to before queue", resource.String(), resource.Object.GetName()))
		after = append(after, resource)
	}

	before = prependMissingNamespaces(before)

	return before, after
}

// isCloudProviderCredentialSecret returns true for Secret resources carrying
// the cloud-provider.deckhouse.io/credentials type. Such Secrets must reach
// the cluster before the cloud-provider-<name> module rolls out, otherwise the
// module's credentials hook sees an empty snapshot and the cloud-controller
// boots with empty credentials.
func isCloudProviderCredentialSecret(resource *template.Resource) bool {
	if resource.GVK.GroupKind() != (schema.GroupKind{Group: "", Kind: "Secret"}) {
		return false
	}
	secretType, _, _ := unstructured.NestedString(resource.Object.Object, "type")
	return secretType == proto.CredentialsSecretType
}

// prependMissingNamespaces inserts a minimal Namespace stub for every distinct
// namespace referenced by the given resources, unless a Namespace with that
// name is already present in the slice. The cloud-provider-<name> module owns
// the namespace via its helm chart; the stub created here gets enriched with
// labels via merge-patch when the module rolls out.
func prependMissingNamespaces(resources template.Resources) template.Resources {
	needed := make(map[string]struct{})
	present := make(map[string]struct{})
	for _, r := range resources {
		if r.GVK.GroupKind() == (schema.GroupKind{Group: "", Kind: "Namespace"}) {
			present[r.Object.GetName()] = struct{}{}
			continue
		}
		if ns := r.Object.GetNamespace(); ns != "" {
			needed[ns] = struct{}{}
		}
	}

	missing := make([]string, 0, len(needed))
	for ns := range needed {
		if _, ok := present[ns]; ok {
			continue
		}
		missing = append(missing, ns)
	}
	// Deterministic order: ranging a map yields a random namespace-stub order.
	sort.Strings(missing)

	stubs := make(template.Resources, 0, len(missing))
	for _, ns := range missing {
		stub := unstructured.Unstructured{}
		stub.SetAPIVersion("v1")
		stub.SetKind("Namespace")
		stub.SetName(ns)
		stubs = append(stubs, &template.Resource{
			GVK:    schema.FromAPIVersionAndKind("v1", "Namespace"),
			Object: stub,
		})
	}

	return append(stubs, resources...)
}

func createResources(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	resourcesToCreate template.Resources,
	result *InstallDeckhouseResult,
	skipChecks bool,
	timeout time.Duration,
) error {
	ctx, span := telemetry.StartSpan(ctx, "ClusterBootstrapper.Bootstrap.createResources")
	defer span.End()

	tasks := make([]actions.ModuleConfigTask, 0)
	if result != nil {
		dhlog.FromContext(ctx).WarnContext(
			ctx,
			"Core module deckhouse has been installed.\n"+
				"The resources provided by your configuration will now be applied.",
		)

		tasks = result.ManifestResult.WithResourcesMCTasks

		span.SetAttributes(otattribute.Int("tasks_count", len(tasks)))

		if len(resourcesToCreate) == 0 {
			for _, task := range tasks {
				loopParams := retry.NewEmptyParams(
					retry.WithName("%s", task.Title),
					retry.WithAttempts(300),
					retry.WithWait(1*time.Second),
					retry.WithWhitelist(actions.ErrManifestTaskTransient),
				)

				return retry.NewLoopWithParams(loopParams).RunContext(ctx, func() error {
					return task.Do(kubeCl)
				})
			}

			return nil
		}
	}

	if len(resourcesToCreate) == 0 {
		return nil
	}

	span.SetAttributes(otattribute.Int("resources_count", len(resourcesToCreate)))

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Create Resources", func(ctx context.Context) error {
		var err error
		checkers := make([]resources.Checker, 0)
		if !skipChecks {
			checkers, err = resources.GetCheckers(ctx, kubeCl, resourcesToCreate, nil)
			if err != nil {
				return err
			}
		}

		return resources.CreateResourcesLoop(ctx, kubeCl, resourcesToCreate, checkers, tasks, timeout)
	})
}
