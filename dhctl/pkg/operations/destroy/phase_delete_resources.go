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

package destroy

import (
	"context"

	"github.com/google/uuid"

	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/kube"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// deleteResourcesPhase is the second phase: drain deckhouse-managed
// kubernetes resources from the cluster. The infra destroyer's own
// pre/post hooks (setupAccess for static, lock for cloud) wrap the
// deckhouse-level deletion. Cluster access is released at the end via
// defer — both on the happy path and if any step inside fails.
type deleteResourcesPhase struct {
	deckhouseState       *deckhouse.State
	kubeProvider         kube.ClientProviderWithCleanup
	phasedActionProvider phases.DefaultActionProvider
	loggerProvider       log.LoggerProvider
	commanderMode        bool
	commanderUUID        uuid.UUID
	skipResources        bool
}

func (p *deleteResourcesPhase) run(ctx context.Context, prep prepared) (err error) {
	defer p.closeAccess(ctx, prep)

	if err := prep.destroyer.Prepare(ctx); err != nil {
		return err
	}

	if err := deckhouse.DeleteResources(ctx, p.deckhouseParams()); err != nil {
		return err
	}

	if err := prep.destroyer.AfterResourcesDelete(ctx); err != nil {
		return err
	}

	return deckhouse.MarkResourcesDeleted(ctx, p.deckhouseParams())
}

// closeAccess releases the k8s-API access (kube proxy + any SSH used to
// reach the API) once we are done with cluster-level work. destroyInfra
// does not need k8s access; for static it opens its own SSH session
// directly to the masters via SSHClientProvider.
func (p *deleteResourcesPhase) closeAccess(ctx context.Context, prep prepared) {
	if err := prep.destroyer.CleanupBeforeDestroy(ctx); err != nil {
		log.SafeProvideLogger(p.loggerProvider).LogWarnF("cleanup before destroy: %v\n", err)
	}
}

func (p *deleteResourcesPhase) deckhouseParams() deckhouse.Params {
	return deckhouse.Params{
		CommanderMode:        p.commanderMode,
		CommanderUUID:        p.commanderUUID,
		SkipResources:        p.skipResources,
		State:                p.deckhouseState,
		LoggerProvider:       p.loggerProvider,
		KubeProvider:         p.kubeProvider,
		PhasedActionProvider: p.phasedActionProvider,
	}
}
