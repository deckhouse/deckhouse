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

// TODO structure these functions into classes
// TODO move states saving to operations/bootstrap/state.go

package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/deckhouse/lib-dhctl/pkg/retry"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	registry_config "github.com/deckhouse/deckhouse/dhctl/pkg/config/registry"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	dhlog "github.com/deckhouse/deckhouse/dhctl/pkg/logger"
	"github.com/deckhouse/deckhouse/dhctl/pkg/telemetry"
)

type InstallDeckhouseResult struct {
	ManifestResult *deckhouse.ManifestsResult
}

type InstallDeckhouseParams struct {
	BeforeDeckhouseTask func() error
	State               *State
	DeckhouseTimeout    time.Duration
}

func InstallDeckhouse(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	config *config.DeckhouseInstaller,
	params InstallDeckhouseParams,
) (*InstallDeckhouseResult, error) {
	res := &InstallDeckhouseResult{}

	return res, dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Install Deckhouse", func(ctx context.Context) error {
		ctx, span := telemetry.StartSpan(ctx, "InstallDeckhouse")
		defer span.End()

		err := CheckPreventBreakAnotherBootstrappedCluster(ctx, kubeCl, config)
		if err != nil {
			return err
		}

		resManifests, err := deckhouse.CreateDeckhouseManifests(ctx, kubeCl, config, params.BeforeDeckhouseTask)
		if err != nil {
			return fmt.Errorf("Deckhouse create manifests: %w", err)
		}

		res.ManifestResult = resManifests

		if err := params.State.SaveManifestsCreated(ctx); err != nil {
			return fmt.Errorf("Set manifests in cluster flag to cache: %w", err)
		}

		err = deckhouse.WaitForReadiness(ctx, kubeCl, params.DeckhouseTimeout)
		if err != nil {
			return fmt.Errorf("Deckhouse not ready: %w", err)
		}

		// Warning! This function must be called at the end of the Deckhouse installation phase.
		// At the end of this function, the registry-init secret is deleted,
		// which is used during DeckhouseInstall for certain registry operation modes.
		err = registry_config.WaitForRegistryInitialization(ctx, kubeCl, config.Registry)
		if err != nil {
			return fmt.Errorf("registry initialization: %v", err)
		}

		return nil
	})
}

func applyPostBootstrapModuleConfigs(
	ctx context.Context,
	kubeCl *client.KubernetesClient,
	tasks []actions.ModuleConfigTask,
) error {
	ctx, span := telemetry.StartSpan(ctx, "applyPostBootstrapModuleConfigs") //nolint:ineffassign,staticcheck // ctx reassigned for span propagation to future calls
	defer span.End()

	for _, task := range tasks {
		p := retry.NewEmptyParams(
			retry.WithName("%s", task.Title),
			retry.WithAttempts(15),
			retry.WithWait(5*time.Second),
			retry.WithLogger(dhlog.NewLibdhctlAdapter(ctx)),
		)
		err := retry.NewLoopWithParams(p).
			Run(func() error {
				return task.Do(kubeCl)
			})
		if err != nil {
			return err
		}
	}

	return nil
}

func RunPostInstallTasks(ctx context.Context, kubeCl *client.KubernetesClient, result *InstallDeckhouseResult) error {
	ctx, span := telemetry.StartSpan(ctx, "RunPostInstallTasks")
	defer span.End()

	if result == nil {
		dhlog.FromContext(ctx).DebugContext(ctx, "Skip post install tasks because result is nil")
		return nil
	}

	return dhlog.RunProcess(ctx, dhlog.FromContext(ctx), "Run post bootstrap actions", func(ctx context.Context) error {
		return applyPostBootstrapModuleConfigs(ctx, kubeCl, result.ManifestResult.PostBootstrapMCTasks)
	})
}
