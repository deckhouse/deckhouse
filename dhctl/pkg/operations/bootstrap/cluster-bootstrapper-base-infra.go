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

package bootstrap

import (
	"context"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// BaseInfrastructure is the standalone `bootstrap-phase base-infra` command: the bootstrap walk
// restricted to the BaseInfra node. Preparation runs ahead of it, as it does in a full bootstrap -
// it loads the config the gates are resolved against and opens the state cache the node writes the
// infrastructure state into.
//
// Running the node instead of a copy of it is what keeps the two paths from drifting: the config is
// now loaded exactly as a full bootstrap loads it, x-rules included, and a cluster type that has no
// base infrastructure is refused by the gate rather than by a check of its own.
func (b *ClusterBootstrapper) BaseInfrastructure(ctx context.Context) error {
	bctx := &bootstrapContext{}

	defer func() {
		if err := b.PhasedExecutionContext.Finalize(ctx, bctx.cacheOrGlobal()); err != nil {
			dhlog.FromContext(ctx).WarnContext(ctx, "failed to finalize phased execution context", "error", err)
		}
		if bctx.cleanup != nil {
			bctx.cleanup()
		}
		if bctx.registryStop != nil {
			bctx.registryStop()
		}
	}()

	return b.runPhases(ctx, bctx, b.bootstrapPhaseFuncs(), phases.BaseInfraPhase)
}
