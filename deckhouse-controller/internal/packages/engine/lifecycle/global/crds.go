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

// The CRDs of the enabled modules, ensured by every global run.

package global

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
)

// crdParallelism bounds the concurrent CRD installs of one run. Parallel, because every package
// waits behind the barrier and the first run after a start installs every module's CRDs; bounded,
// because the installs share one client and its rate limiter.
const crdParallelism = 12

// ensureCRDs ensures the CRDs of every enabled module. A first pass, in the event's order, takes
// each module's view from the event and records the names publish needs. Then the CRDs are
// installed in parallel; the first error cancels the rest. Invariant: no two modules ship the same
// CRD, so concurrent installs never write one object.
//
// Every run asks for every enabled module's CRDs; the service installs only a path it has not
// installed in this process, so a CRD deleted or edited by hand returns with the module's next
// version or the next start, as under addon-operator. A broken CRD is reported on Global's
// ManifestsApplied, not on the module that ships it.
func (g *Global) ensureCRDs(ctx context.Context, st *operationState) error {
	for _, m := range st.enabled {
		// The one place the attachment is asserted: the package Reconciler attaches the ModuleView
		// of the node's revision to every module node (lifecycle.schedulerNode).
		view, ok := m.Attachment.(model.ModuleView)
		if !ok {
			return fmt.Errorf("module %s carries no view", m.ID)
		}

		st.names = append(st.names, view.Name)
		if err := g.deps.CRDs.HasCRDs(view.Path); err == nil {
			st.names = append(st.names, view.Name+"-crd")
		}

		st.modules = append(st.modules, view)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(crdParallelism)

	for _, view := range st.modules {
		group.Go(func() error {
			// Installs started after the first failure skip the API call.
			if err := ctx.Err(); err != nil {
				return err
			}

			if err := g.deps.CRDs.Install(ctx, view.Name, view.Path); err != nil {
				return fmt.Errorf("ensure CRDs of module %s@%s: %w", view.Name, view.Version, err)
			}

			return nil
		})
	}

	return group.Wait()
}
