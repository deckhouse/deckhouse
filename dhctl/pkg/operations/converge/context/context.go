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

package context

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/name212/govalue"

	libcon "github.com/deckhouse/lib-connection/pkg"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config/directoryconfig"
	"github.com/deckhouse/deckhouse/dhctl/pkg/infrastructure"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/providerinitializer"
)

type MultiMasterClientSwitcher interface {
	SwitchToFirstMaster(ctx context.Context) error
	SwitchToNotFirstMaster(ctx context.Context) error
	SwitchWhenDecreaseMastersIfNeed(ctx context.Context, ngName string, nodesToDeleteInfo []*NodeState) error
	SwitchClientsToAnotherNodeIfNeed(ctx context.Context, nodeName, ip string) error
}

type Context struct {
	kubeClientMu           sync.RWMutex
	kubeProvider           libcon.KubeProvider
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer

	stateCache dstate.Cache
	// yes we want to save context in struct,
	// but it is not recommended https://go.dev/wiki/CodeReviewComments#contexts
	ctx                   context.Context
	phaseContext          phases.DefaultPhasedExecutionContext
	infrastructureContext *infrastructure.Context
	commanderParams       *commander.CommanderModeParams
	changeParams          infrastructure.ChangeActionSettings
	stateStore            stateStore
	stateChecker          infrastructure.StateChecker
	clientSwitcher        MultiMasterClientSwitcher

	providerGetter infrastructure.CloudProviderGetter

	logger          log.Logger
	directoryConfig *directoryconfig.DirectoryConfig
}

type Params struct {
	KubeProvider           libcon.KubeProvider
	SSHProviderInitializer *providerinitializer.SSHProviderInitializer
	Cache                  dstate.Cache
	ChangeParams           infrastructure.ChangeActionSettings
	ProviderGetter         infrastructure.CloudProviderGetter
	Logger                 log.Logger
	ClientSwitcher         MultiMasterClientSwitcher
	DirectoryConfig        *directoryconfig.DirectoryConfig
}

func newContext(ctx context.Context, params Params) *Context {
	logger := params.Logger

	if govalue.IsNil(logger) {
		logger = log.GetDefaultLogger()
	}

	return &Context{
		providerGetter:         params.ProviderGetter,
		kubeProvider:           params.KubeProvider,
		SSHProviderInitializer: params.SSHProviderInitializer,
		stateCache:             params.Cache,
		changeParams:           params.ChangeParams,
		ctx:                    ctx,
		logger:                 logger,
		clientSwitcher:         params.ClientSwitcher,
		directoryConfig:        params.DirectoryConfig,

		stateStore: newInSecretStateStore(),
	}
}

func NewContext(ctx context.Context, params Params) *Context {
	return newContext(ctx, params)
}

func NewCommanderContext(ctx context.Context, contextParams Params, params *commander.CommanderModeParams) *Context {
	c := newContext(ctx, contextParams)
	c.commanderParams = params
	return c
}

func (c *Context) WithPhaseContext(phaseContext phases.DefaultPhasedExecutionContext) *Context {
	c.phaseContext = phaseContext
	return c
}

func (c *Context) WithStateChecker(checker infrastructure.StateChecker) *Context {
	c.stateChecker = checker
	return c
}

func (c *Context) WithInfrastructureContext(ctx *infrastructure.Context) *Context {
	c.infrastructureContext = ctx
	return c
}

func (c *Context) WithProviderGetter(getter infrastructure.CloudProviderGetter) *Context {
	c.providerGetter = getter
	return c
}

func (c *Context) KubeProvider() kubernetes.KubeClientProvider {
	return c
}

func (c *Context) KubeClient() *client.KubernetesClient {
	c.kubeClientMu.RLock()
	defer c.kubeClientMu.RUnlock()

	kubeClient, _ := c.kubeProvider.Client(c.ctx)

	return &client.KubernetesClient{KubeClient: kubeClient}
}

func (c *Context) KubeClientCtx(ctx context.Context) (*client.KubernetesClient, error) {
	kubeClient, err := c.kubeProvider.Client(c.ctx)
	return &client.KubernetesClient{KubeClient: kubeClient}, err
}

func (c *Context) ClientSwitcher() MultiMasterClientSwitcher {
	return c.clientSwitcher
}

// SetClientSwitcher
// Warning! do not use in controllers only in initialization!
func (c *Context) SetClientSwitcher(s MultiMasterClientSwitcher) {
	c.clientSwitcher = s
}

func (c *Context) InfrastructureContext(metaConfig *config.MetaConfig) *infrastructure.Context {
	var ctx *infrastructure.Context
	if c.infrastructureContext != nil {
		ctx = c.infrastructureContext
	} else {
		ctx = infrastructure.NewContextWithProvider(c.providerGetter, c.Logger())
	}

	ctx.WithStateChecker(c.stateChecker)

	return ctx
}

func (c *Context) Ctx() context.Context {
	return c.ctx
}

func (c *Context) WithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(c.ctx, timeout)
}

func (c *Context) StateCache() dstate.Cache {
	return c.stateCache
}

func (c *Context) CommanderMode() bool {
	return c.commanderParams != nil
}

func (c *Context) ProviderGetter() infrastructure.CloudProviderGetter {
	return c.providerGetter
}

func (c *Context) StarExecutionPhase(ctx context.Context, phase phases.OperationPhase, isCritical bool) (bool, error) {
	if c.phaseContext == nil {
		return false, nil
	}

	return c.phaseContext.StartPhase(ctx, phase, isCritical, c.stateCache)
}

func (c *Context) CompleteExecutionPhase(ctx context.Context, data any) error {
	if c.phaseContext == nil {
		return nil
	}

	return c.phaseContext.CompletePhase(ctx, c.stateCache, data)
}

func (c *Context) MetaConfig() (*config.MetaConfig, error) {
	if c.CommanderMode() {
		metaConfig, err := commander.ParseMetaConfig(c.Ctx(), c.stateCache, c.commanderParams, c.logger)
		if err != nil {
			return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
		}

		return metaConfig, nil
	}

	metaConfig, err := entity.GetMetaConfig(c.ctx, c.KubeClient(), c.logger, c.directoryConfig)
	if err != nil {
		return nil, err
	}

	return metaConfig, nil
}

func (c *Context) Logger() log.Logger {
	return c.logger
}

func (c *Context) ChangesSettings() infrastructure.ChangeActionSettings {
	return c.changeParams
}

func (c *Context) SetConvergeState(state *State) error {
	return c.stateStore.SetState(c, state)
}

func (c *Context) ConvergeState() (*State, error) {
	return c.stateStore.GetState(c)
}

func (c *Context) deleteConvergeState() error {
	return c.stateStore.Delete(c)
}
