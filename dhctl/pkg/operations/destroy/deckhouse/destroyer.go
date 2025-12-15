// Copyright 2025 Flant JSC
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

package deckhouse

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/commander"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

type DestroyerParams struct {
	CommanderMode bool
	CommanderUUID uuid.UUID
	SkipResources bool

	State *State

	LoggerProvider       log.LoggerProvider
	KubeProvider         kubernetes.KubeClientProviderWithCtx
	PhasedActionProvider phases.DefaultActionProvider
}

type Destroyer struct {
	DestroyerParams
}

func NewDestroyer(opts DestroyerParams) *Destroyer {
	return &Destroyer{
		DestroyerParams: opts,
	}
}

func (g *Destroyer) CheckCommanderUUID(ctx context.Context) error {
	if !g.CommanderMode {
		return nil
	}

	if g.isSkipResources("CheckCommanderUUID") {
		return nil
	}

	kubeCl, err := g.KubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	_, err = commander.CheckShouldUpdateCommanderUUID(ctx, kubeCl, g.CommanderUUID)
	if err != nil {
		return fmt.Errorf("UUID consistency check failed: %w", err)
	}

	return nil
}

func (g *Destroyer) CheckAndDeleteResources(ctx context.Context) error {
	logger := g.logger()

	if g.isSkipResources("DeleteResources") {
		return nil
	}

	return g.PhasedActionProvider().Run(phases.DeleteResourcesPhase, false, func() (phases.DefaultContextType, error) {
		return nil, g.deleteResources(ctx, logger)
	})
}

func (g *Destroyer) Finalize(context.Context) error {
	if g.isSkipResources("Finalize") {
		return nil
	}

	alreadyDestroyed, err := g.State.IsResourcesDestroyed()
	if err != nil {
		return err
	}

	logger := g.logger()

	if alreadyDestroyed {
		logger.LogDebugLn("Resources already destroyed. Skip set as destroyed")
		return nil
	}

	err = g.PhasedActionProvider().Run(phases.SetDeckhouseResourcesDeletedPhase, false, func() (phases.DefaultContextType, error) {
		return nil, g.State.SetResourcesDestroyed()
	})

	if err != nil {
		return err
	}

	logger.LogDebugLn("Resources were destroyed set")
	return nil
}

func (g *Destroyer) deleteResources(ctx context.Context, logger log.Logger) error {
	resourcesDestroyed, err := g.State.IsResourcesDestroyed()
	if err != nil {
		return err
	}

	if resourcesDestroyed {
		logger.LogWarnLn("Resources was destroyed. Skip it")
		return nil
	}

	kubeCl, err := g.KubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	return logger.LogProcess("common", "Delete resources from the Kubernetes cluster", func() error {
		return g.deleteEntities(ctx, kubeCl)
	})
}

func (g *Destroyer) deleteEntities(ctx context.Context, kubeCl *client.KubernetesClient) error {
	err := deckhouse.DeleteDeckhouseDeployment(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForDeckhouseDeploymentDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePDBs(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteServices(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForServicesDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteAllD8StorageResources(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteStorageClasses(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePVC(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeletePods(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVCDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.WaitForPVDeletion(ctx, kubeCl)
	if err != nil {
		return err
	}

	err = deckhouse.DeleteMachinesIfResourcesExist(ctx, kubeCl)
	if err != nil {
		return err
	}

	return nil
}

func (g *Destroyer) isSkipResources(phase string) bool {
	if g.SkipResources {
		g.logger().LogInfoF("Deckhouse resources destroyer '%s': skipped by flag\n", phase)
		return true
	}

	return false
}

func (g *Destroyer) logger() log.Logger {
	return log.SafeProvideLogger(g.LoggerProvider)
}
