// Copyright 2024 Flant JSC
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

package infrastructure

import (
	"context"
	"fmt"
	"sync"

	"github.com/name212/govalue"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
)

type CloudProviderGetter func(ctx context.Context, metaConfig *config.MetaConfig) (CloudProvider, error)

type Context struct {
	infrastructureRunnerByName    map[string]RunnerInterface
	infrastructureRunnerByNameMux sync.Mutex
	provider                      CloudProviderGetter
	stateChecker                  StateChecker
	logger                        log.Logger
}

func NewContextWithProvider(provider CloudProviderGetter, logger log.Logger) *Context {
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &Context{
		infrastructureRunnerByName: make(map[string]RunnerInterface),
		provider:                   provider,
		logger:                     logger,
	}
}

func NewContext(logger log.Logger) *Context {
	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &Context{
		infrastructureRunnerByName: make(map[string]RunnerInterface),
		logger:                     logger,
	}
}

func (f *Context) SetCloudProviderGetter(provider CloudProviderGetter) {
	f.provider = provider
}

func (f *Context) WithStateChecker(checker StateChecker) {
	f.stateChecker = checker
}

func (f *Context) CloudProviderGetter() CloudProviderGetter {
	return f.provider
}

func (f *Context) getCloudProvider(ctx context.Context, metaConfig *config.MetaConfig) (CloudProvider, error) {
	uuid, err := metaConfig.GetFullUUID()
	if err != nil {
		return nil, fmt.Errorf("Failed to get cloud provider: %w", err)
	}

	getter := f.CloudProviderGetter()
	if getter == nil {
		return nil, fmt.Errorf(
			"Failed to get cloud provider for %s/%s/%s. Cloud providerGetter should set",
			metaConfig.ClusterPrefix,
			uuid,
			metaConfig.ProviderName,
		)
	}

	return getter(ctx, metaConfig)
}

func applyAutomaticSettingsForChangesRunner(r *Runner, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(true).
		WithAutoApprove(false).
		WithSkipChangesOnDeny(true).
		WithAutoDismissChanges(true)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticSettings(r *Runner, settings AutomaticSettings, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(settings.AutoDismissDestructive).
		WithAutoApprove(settings.AutoApprove).
		WithAutoDismissChanges(settings.AutoDismissChanges)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticSettingsForBootstrap(r *Runner, stateChecker StateChecker) *Runner {
	r.WithAutoDismissDestructiveChanges(false).
		WithAutoApprove(true).
		WithAutoDismissChanges(false)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func applyAutomaticApproveSettings(r *Runner, settings AutoApproveSettings, stateChecker StateChecker) *Runner {
	r.WithAutoApprove(settings.AutoApprove)

	if stateChecker != nil {
		r.WithStateChecker(stateChecker)
	}

	return r
}

func addProviderAfterCleanupFuncForRunner(cloudProvider CloudProvider, group string, r RunnerInterface) {
	targetGroup := fmt.Sprintf("stopExecutorFor:%s", group)
	cloudProvider.AddAfterCleanupFunc(targetGroup, func(log.Logger) {
		r.Stop()
	})
}

func (f *Context) GetCheckBaseInfraRunner(ctx context.Context, metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	const group = "base-infrastructure"

	executor, err := cloudProvider.Executor(ctx, BaseInfraStep, f.logger)
	if err != nil {
		return nil, err
	}

	if opts.CommanderMode {
		r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
			WithVariables(metaConfig.MarshalConfig())

		r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

		addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
		return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
	}

	r := NewImmutableRunnerFromConfig(metaConfig, executor).
		WithVariables(metaConfig.MarshalConfig())
	if opts.ClusterState != nil {
		r.WithState(opts.ClusterState)
	}

	addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
	return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
}

func (f *Context) GetCheckNodeRunner(ctx context.Context, metaConfig *config.MetaConfig, opts NodeRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.NodeGroupStep, f.logger)
	if err != nil {
		return nil, err
	}

	group := opts.NodeName

	if opts.CommanderMode {
		r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
			WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
			WithName(opts.NodeName).
			WithHook(opts.Hook)

		r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

		addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
		return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
	}

	r := NewImmutableRunnerFromConfig(metaConfig, executor).
		WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
		WithState(opts.NodeState).
		WithName(opts.NodeName)

	addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
	return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
}

func (f *Context) GetCheckNodeDeleteRunner(ctx context.Context, metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.LayoutStep, f.logger)
	if err != nil {
		return nil, err
	}

	group := opts.NodeName

	if opts.CommanderMode {
		r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
			WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
			WithName(opts.NodeName).
			WithAllowedCachedState(true)

		r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

		addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
		return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
	}

	r := NewImmutableRunnerFromConfig(metaConfig, executor).
		WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
		WithName(opts.NodeName).
		WithState(opts.NodeState)

	addProviderAfterCleanupFuncForRunner(cloudProvider, group, r)
	return applyAutomaticSettingsForChangesRunner(r, f.stateChecker), nil
}

type BaseInfraRunnerOptions struct {
	CommanderMode                    bool
	StateCache                       dstate.Cache
	ClusterState                     []byte
	AdditionalStateSaverDestinations []SaverDestination
}

func (f *Context) GetConvergeBaseInfraRunner(ctx context.Context, metaConfig *config.MetaConfig, opts BaseInfraRunnerOptions, automaticSettings AutomaticSettings) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, BaseInfraStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
		WithSkipChangesOnDeny(true).
		WithVariables(metaConfig.MarshalConfig())
	if opts.ClusterState != nil {
		r = r.WithState(opts.ClusterState)
	}

	r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

	addProviderAfterCleanupFuncForRunner(cloudProvider, "base-infrastructure", r)
	return applyAutomaticSettings(r, automaticSettings, f.stateChecker), nil
}

type NodeRunnerOptions struct {
	NodeName        string
	NodeGroupName   string
	NodeGroupStep   Step
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
	Hook                             InfraActionHook
}

func (f *Context) GetConvergeNodeRunner(ctx context.Context, metaConfig *config.MetaConfig, opts NodeRunnerOptions, automaticSettings AutomaticSettings) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.NodeGroupStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
		WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
		WithSkipChangesOnDeny(true).
		WithName(opts.NodeName).
		WithHook(opts.Hook)

	if opts.NodeState != nil {
		r = r.WithState(opts.NodeState)
	}

	r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

	addProviderAfterCleanupFuncForRunner(cloudProvider, opts.NodeName, r)
	return applyAutomaticSettings(r, automaticSettings, f.stateChecker), nil
}

type NodeDeleteRunnerOptions struct {
	NodeName        string
	NodeGroupName   string
	LayoutStep      Step
	NodeIndex       int
	NodeState       []byte
	NodeCloudConfig string

	CommanderMode                    bool
	StateCache                       dstate.Cache
	AdditionalStateSaverDestinations []SaverDestination
	Hook                             InfraActionHook
}

func (f *Context) GetConvergeNodeDeleteRunner(ctx context.Context, metaConfig *config.MetaConfig, opts NodeDeleteRunnerOptions, automaticSettings AutomaticSettings) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.LayoutStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, opts.StateCache, executor).
		WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)).
		WithName(opts.NodeName).
		WithAllowedCachedState(true).
		WithSkipChangesOnDeny(true).
		WithHook(opts.Hook)

	if opts.NodeState != nil {
		r = r.WithState(opts.NodeState)
	}

	r.WithAdditionalStateSaverDestination(opts.AdditionalStateSaverDestinations...)

	addProviderAfterCleanupFuncForRunner(cloudProvider, opts.NodeName, r)
	return applyAutomaticSettings(r, automaticSettings, f.stateChecker), nil
}

func (f *Context) GetBootstrapBaseInfraRunner(ctx context.Context, metaConfig *config.MetaConfig, stateCache dstate.Cache) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, BaseInfraStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, stateCache, executor).
		WithVariables(metaConfig.MarshalConfig())

	addProviderAfterCleanupFuncForRunner(cloudProvider, "base-infrastructure", r)

	return applyAutomaticSettingsForBootstrap(r, f.stateChecker), nil
}

type BootstrapNodeRunnerOptions struct {
	NodeName                         string
	NodeGroupName                    string
	NodeGroupStep                    Step
	NodeIndex                        int
	NodeCloudConfig                  string
	AdditionalStateSaverDestinations []SaverDestination
	RunnerLogger                     log.Logger
}

func (f *Context) GetBootstrapNodeRunner(ctx context.Context, metaConfig *config.MetaConfig, stateCache dstate.Cache, opts BootstrapNodeRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.NodeGroupStep, f.logger)
	if err != nil {
		return nil, err
	}

	nodeConfig := metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, opts.NodeCloudConfig)

	r := NewRunnerFromConfig(metaConfig, stateCache, executor).
		WithVariables(nodeConfig).
		WithName(opts.NodeName).
		WithLogger(opts.RunnerLogger)

	addProviderAfterCleanupFuncForRunner(cloudProvider, opts.NodeName, r)
	return applyAutomaticSettingsForBootstrap(r, f.stateChecker), nil
}

type DestroyBaseInfraRunnerOptions struct {
	AutoApproveSettings
}

func (f *Context) GetDestroyBaseInfraRunner(ctx context.Context, metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyBaseInfraRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, BaseInfraStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, stateCache, executor).
		WithVariables(metaConfig.MarshalConfig()).
		WithAllowedCachedState(true)

	addProviderAfterCleanupFuncForRunner(cloudProvider, "base-infrastructure", r)
	return applyAutomaticApproveSettings(r, opts.AutoApproveSettings, f.stateChecker), nil
}

type DestroyNodeRunnerOptions struct {
	AutoApproveSettings

	NodeName      string
	NodeGroupName string
	NodeGroupStep Step
	NodeIndex     int
}

func (f *Context) GetDestroyNodeRunner(ctx context.Context, metaConfig *config.MetaConfig, stateCache dstate.Cache, opts DestroyNodeRunnerOptions) (RunnerInterface, error) {
	cloudProvider, err := f.getCloudProvider(ctx, metaConfig)
	if err != nil {
		return nil, err
	}

	executor, err := cloudProvider.Executor(ctx, opts.NodeGroupStep, f.logger)
	if err != nil {
		return nil, err
	}

	r := NewRunnerFromConfig(metaConfig, stateCache, executor).
		WithVariables(metaConfig.NodeGroupConfig(opts.NodeGroupName, opts.NodeIndex, "")).
		WithName(opts.NodeName).
		WithAllowedCachedState(true)

	addProviderAfterCleanupFuncForRunner(cloudProvider, opts.NodeName, r)
	return applyAutomaticApproveSettings(r, opts.AutoApproveSettings, f.stateChecker), nil
}
