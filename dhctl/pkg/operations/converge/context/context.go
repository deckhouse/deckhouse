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

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/terraform"
)

type Context struct {
	kubeClientMu sync.RWMutex
	kubeClient   *client.KubernetesClient

	stateCache dstate.Cache
	// yes we want to save context in struct,
	// but it is not recommended https://go.dev/wiki/CodeReviewComments#contexts
	ctx              context.Context
	phaseContext     phases.DefaultPhasedExecutionContext
	terraformContext *terraform.TerraformContext
	commanderParams  *commander.CommanderModeParams
	changeParams     terraform.ChangeActionSettings
	stateStore       stateStore
}

func newContext(ctx context.Context, kubeClient *client.KubernetesClient, cache dstate.Cache, changeParams terraform.ChangeActionSettings) *Context {
	return &Context{
		kubeClient:   kubeClient,
		stateCache:   cache,
		changeParams: changeParams,
		ctx:          ctx,

		terraformContext: terraform.NewTerraformContext(),
		stateStore:       newInSecretStateStore(),
	}
}

func NewContext(ctx context.Context, kubeClient *client.KubernetesClient, cache dstate.Cache, changeParams terraform.ChangeActionSettings) *Context {
	return newContext(ctx, kubeClient, cache, changeParams)
}

func NewCommanderContext(ctx context.Context, kubeClient *client.KubernetesClient, cache dstate.Cache, params *commander.CommanderModeParams, changeParams terraform.ChangeActionSettings) *Context {
	c := newContext(ctx, kubeClient, cache, changeParams)
	c.commanderParams = params
	return c
}

func (c *Context) WithPhaseContext(phaseContext phases.DefaultPhasedExecutionContext) *Context {
	c.phaseContext = phaseContext
	return c
}

func (c *Context) WithTerraformContext(ctx *terraform.TerraformContext) *Context {
	c.terraformContext = ctx
	return c
}

func (c *Context) KubeClient() *client.KubernetesClient {
	c.kubeClientMu.RLock()
	defer c.kubeClientMu.RUnlock()

	return c.kubeClient
}

func (c *Context) Terraform() *terraform.TerraformContext {
	return c.terraformContext
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

func (c *Context) StarExecutionPhase(phase phases.OperationPhase, isCritical bool) (bool, error) {
	if c.phaseContext == nil {
		return false, nil
	}

	return c.phaseContext.StartPhase(phase, isCritical, c.stateCache)
}

func (c *Context) CompleteExecutionPhase(data any) error {
	if c.phaseContext == nil {
		return nil
	}

	return c.phaseContext.CompletePhase(c.stateCache, data)
}

func (c *Context) MetaConfig() (*config.MetaConfig, error) {
	if c.CommanderMode() {
		metaConfig, err := commander.ParseMetaConfig(c.stateCache, c.commanderParams)
		if err != nil {
			return nil, fmt.Errorf("unable to parse meta configuration: %w", err)
		}

		return metaConfig, nil
	}

	metaConfig, err := entity.GetMetaConfig(c.ctx, c.kubeClient)
	if err != nil {
		return nil, err
	}

	return metaConfig, nil
}

func (c *Context) ChangesSettings() terraform.ChangeActionSettings {
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

func (c *Context) setKubeClient(newKubeClient *client.KubernetesClient) {
	c.kubeClientMu.Lock()
	defer c.kubeClientMu.Unlock()

	c.kubeClient = newKubeClient
}
