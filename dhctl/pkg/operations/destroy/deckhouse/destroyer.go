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

// Params bundles the shared inputs used by every deckhouse-level destroy
// step (UUID check, resource deletion, finalize marker). Callers fill it
// once and pass to whichever functions they need.
type Params struct {
	CommanderMode bool
	CommanderUUID uuid.UUID
	SkipResources bool

	State *State

	LoggerProvider       log.LoggerProvider
	KubeProvider         kubernetes.KubeClientProviderWithCtx
	PhasedActionProvider phases.DefaultActionProvider
}

// CheckCommanderUUID validates that the commander UUID supplied for the
// destroy run matches the one persisted in the cluster (or records it on
// first run).
func CheckCommanderUUID(ctx context.Context, p Params) error {
	logger := log.SafeProvideLogger(p.LoggerProvider)

	if !p.CommanderMode {
		logger.LogDebugF("Check commander UUID skipped. No in commander mode\n")
		return nil
	}

	if skipResources(logger, p.SkipResources, "CheckCommanderUUID") {
		return nil
	}

	uuidInCache, err := p.State.CommanderUUID(ctx)
	if err != nil {
		return err
	}

	passedUUID := p.CommanderUUID.String()

	if uuidInCache != "" {
		if uuidInCache == passedUUID {
			logger.LogDebugF("Commander UUID found and correct. Skipping commander UUID check\n")
			return nil
		}

		return fmt.Errorf("Commander UUID found but incorrect. UUID in cache '%s' - UUID passed '%s'\n", uuidInCache, passedUUID)
	}

	kubeCl, err := p.KubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	_, err = commander.CheckShouldUpdateCommanderUUID(ctx, kubeCl, p.CommanderUUID)
	if err != nil {
		return fmt.Errorf("UUID consistency check failed: %w", err)
	}

	return p.PhasedActionProvider().Run(ctx, phases.CommanderUUIDWasChecked, false, func() (phases.DefaultContextType, error) {
		return nil, p.State.SetCommanderUUID(ctx, passedUUID)
	})
}

// DeleteResources removes deckhouse-managed kubernetes resources
// (services, PVCs, validating webhooks, etc.) from the cluster.
// Idempotent: if the state cache already records that resources are
// destroyed, the call is a no-op.
func DeleteResources(ctx context.Context, p Params) error {
	logger := log.SafeProvideLogger(p.LoggerProvider)

	if skipResources(logger, p.SkipResources, "DeleteResources") {
		return nil
	}

	return p.PhasedActionProvider().Run(ctx, phases.DeleteResourcesPhase, false, func() (phases.DefaultContextType, error) {
		return nil, runDelete(ctx, p, logger)
	})
}

// MarkResourcesDeleted writes the "resources destroyed" marker into the
// state cache so a subsequent destroy run skips the deletion step.
func MarkResourcesDeleted(ctx context.Context, p Params) error {
	logger := log.SafeProvideLogger(p.LoggerProvider)

	if skipResources(logger, p.SkipResources, "Finalize") {
		return nil
	}

	alreadyDestroyed, err := p.State.IsResourcesDestroyed(ctx)
	if err != nil {
		return err
	}

	if alreadyDestroyed {
		logger.LogDebugLn("Resources already destroyed. Skip set as destroyed")
		return nil
	}

	if err := p.PhasedActionProvider().Run(ctx, phases.SetDeckhouseResourcesDeletedPhase, false, func() (phases.DefaultContextType, error) {
		return nil, p.State.SetResourcesDestroyed(ctx)
	}); err != nil {
		return err
	}

	logger.LogDebugLn("Resources were destroyed set")
	return nil
}

func runDelete(ctx context.Context, p Params, logger log.Logger) error {
	resourcesDestroyed, err := p.State.IsResourcesDestroyed(ctx)
	if err != nil {
		return err
	}

	if resourcesDestroyed {
		logger.LogWarnLn("Resources was destroyed. Skip it")
		return nil
	}

	kubeCl, err := p.KubeProvider.KubeClientCtx(ctx)
	if err != nil {
		return err
	}

	return logger.LogProcessCtx(ctx, "common", "Delete resources from the Kubernetes cluster", func(ctx context.Context) error {
		return deleteEntities(ctx, kubeCl)
	})
}

func deleteEntities(ctx context.Context, kubeCl *client.KubernetesClient) error {
	steps := []func(context.Context, *client.KubernetesClient) error{
		deckhouse.DeleteValidatingWebhookConfigurations,
		deckhouse.DeleteDeckhouseDeployment,
		deckhouse.WaitForDeckhouseDeploymentDeletion,
		deckhouse.DeletePDBs,
		deckhouse.DeleteServices,
		deckhouse.WaitForServicesDeletion,
		deckhouse.DeleteAllD8StorageResources,
		deckhouse.DeleteStorageClasses,
		deckhouse.DeletePVC,
		deckhouse.DeletePods,
		deckhouse.WaitForPVCDeletion,
		deckhouse.WaitForPVDeletion,
		deckhouse.DeleteMachinesIfResourcesExist,
		deckhouse.DeleteValidatingWebhookConfigurations,
	}
	for _, step := range steps {
		if err := step(ctx, kubeCl); err != nil {
			return err
		}
	}
	return nil
}

func skipResources(logger log.Logger, skip bool, name string) bool {
	if skip {
		logger.LogInfoF("Deckhouse resources destroyer '%s': skipped by flag\n", name)
		return true
	}
	return false
}
