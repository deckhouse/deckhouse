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

// The step lists of Global's operations; the order is the contract. The steps run in the worker.

package global

import (
	"context"
	"fmt"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/configstate"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/operation"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
)

// Step names, in reports and logs.
const (
	stepStartHooks     = "startHooks"
	stepOnStartup      = "onStartup"
	stepBeforeAll      = "beforeAll"
	stepEnsureCRDs     = "ensureCRDs"
	stepPublish        = "publish"
	stepEnsureWebhooks = "ensureWebhooks"
)

// operationState is what the steps of one operation share: the loop state they read, snapshotted at
// start, plus their outputs. The steps touch only this, Global's immutable fields and the global
// module; the loop reads it back once the operation is done.
type operationState struct {
	// stopHooks stops the hooks startHooks started; a failed onStartup stops them and clears it.
	stopHooks func()

	// Of a run:

	// enabled is the event's EnabledModules: the identity and the view of every enabled module.
	enabled  []schedule.EnabledModule
	accepted *configstate.Accepted // accepted are the settings the global module holds for the whole run.
	// modules are the views ensureCRDs resolved from enabled, for the rest of the run.
	modules []model.ModuleView
	// names is the published global.enabledModules: every enabled module's name and, for a module
	// that ships CRDs, also <name>-crd.
	names  []string
	output string // output is the values checksum after the publish step; empty before it.
}

// syncSteps is the sync, once per process before any run: the hooks start (startHooks), then the
// OnStartup hooks set the values the boot gate and every package read (the Deckhouse version, the
// edition, the image digests), so they exist when Synced closes.
func (g *Global) syncSteps(st *operationState) []operation.Step {
	return []operation.Step{
		{Name: stepStartHooks, Condition: operation.HooksProcessed, Reason: model.ReasonHookInitializationFailed,
			Run: func(ctx context.Context) error { return g.startHooks(ctx, st) }},
		{Name: stepOnStartup, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed,
			Run: func(ctx context.Context) error { return g.onStartup(ctx, st) }},
	}
}

// runSteps is the global run; the order is the contract:
//   - beforeAll first: the hooks prepare the global values every module renders against;
//   - ensureCRDs before publish: the published capabilities are those of the ensured CRDs;
//   - publish before ensureWebhooks: a webhook render must see the values it is deployed with;
//   - ensureWebhooks inside the run: a module's conversions are registered before its release,
//     which waits behind Global, applies the resources they convert.
func (g *Global) runSteps(st *operationState) []operation.Step {
	return []operation.Step{
		{Name: stepBeforeAll, Condition: operation.HooksProcessed, Reason: model.ReasonHookFailed, Run: g.beforeAll},
		{Name: stepEnsureCRDs, Condition: operation.ManifestsApplied, Reason: model.ReasonCRDsApplyFailed,
			Run: func(ctx context.Context) error { return g.ensureCRDs(ctx, st) }},
		{Name: stepPublish, Condition: operation.ManifestsApplied, Reason: model.ReasonManifestsApplyFailed,
			Run: func(context.Context) error { return g.publish(st) }},
		{Name: stepEnsureWebhooks, Condition: operation.ManifestsApplied, Reason: model.ReasonWebhooksApplyFailed,
			Run: func(ctx context.Context) error { return g.ensureWebhooks(ctx, st) }},
	}
}

func (g *Global) beforeAll(ctx context.Context) error {
	if err := g.module.RunHooksByBinding(ctx, addontypes.BeforeAll); err != nil {
		return fmt.Errorf("run %s hooks: %w", addontypes.BeforeAll, err)
	}

	return nil
}

// publish publishes the enabled set and the capabilities into the global values and records
// their checksum as the run's Output, which the packages' inputs hash. The enabled modules' image
// digests are published beside the values, outside the Output: a module's own Output tells its
// subscribers when its digests changed.
func (g *Global) publish(st *operationState) error {
	g.module.SetEnabledModules(st.names)
	g.module.SetCapabilities(g.deps.CRDs.GetManagedGVKs(st.names))
	g.module.SetModulesImagesDigests(imagesDigests(st.modules))

	st.output = g.module.GetValuesChecksum()

	return nil
}

// startHooks starts the global hook bindings and waits for their synchronization. The bindings
// live under a child of Global's context, not of the operation's, because they run for as long as
// Global does. A failed attempt has stopped what it started (hooklane Start), so the retry starts
// from fresh hook controllers after its backoff.
func (g *Global) startHooks(ctx context.Context, st *operationState) error {
	stop, err := g.deps.Hooks.Start(ctx, g.ctx, g.module, g.changed)
	if err != nil {
		return err
	}

	st.stopHooks = stop

	return nil
}

// onStartup runs the OnStartup hooks. A failure stops the hooks startHooks started, as a failed
// startHooks does, so the retry starts them from scratch and exit finds nothing running.
func (g *Global) onStartup(ctx context.Context, st *operationState) error {
	if err := g.module.RunHooksByBinding(ctx, shtypes.OnStartup); err != nil {
		st.stopHooks()
		st.stopHooks = nil

		return fmt.Errorf("run %s hooks: %w", shtypes.OnStartup, err)
	}

	return nil
}

// ensureWebhooks ensures the conversion webhooks of every enabled module, one at a time.
func (g *Global) ensureWebhooks(ctx context.Context, st *operationState) error {
	for _, m := range st.modules {
		if err := g.deps.Webhooks.EnsureWebhooks(ctx, m); err != nil {
			return fmt.Errorf("ensure conversion webhooks of module %s@%s: %w", m.Name, m.Version, err)
		}
	}

	return nil
}

// imagesDigests are the image digests of the revisions the event names, by module name: exactly the
// running ones, never a Candidate attached ahead of its window.
func imagesDigests(modules []model.ModuleView) map[string]map[string]string {
	digests := make(map[string]map[string]string, len(modules))

	for _, view := range modules {
		if d, ok := view.Package.(digested); ok && len(d.GetDigests()) > 0 {
			digests[view.Name] = d.GetDigests()
		}
	}

	return digests
}
