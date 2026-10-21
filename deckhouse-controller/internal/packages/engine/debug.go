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

// The read-only data the debug API serves (api/handlers/root/v1). Every method is a snapshot and
// never blocks on lifecycle work.

package engine

import (
	"context"
	"errors"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/apps"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/modules"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/observed"
)

// packagesDump is the packages dump: every package the core knows, by kind.
type packagesDump struct {
	Apps    map[string]packageDump `json:"apps"`
	Modules map[string]packageDump `json:"modules"`
}

// packageDump is one package: its lifecycle report, its observation and the info of its activated
// object (nil while it has none).
type packageDump struct {
	Report   model.Report         `json:"report"`
	Observed observed.Observation `json:"observed"`
	Info     any                  `json:"info"`
	// Lanes is the short state of the hook lanes; the queues endpoint has the whole.
	Lanes []hooklane.LaneSummary `json:"lanes,omitempty"`
}

// snapshotter is the object's hook snapshots, by hook name.
type snapshotter interface {
	GetHookSnapshotsDump() map[string]any
}

// Dump returns every package the core knows, applications and modules apart.
func (e *Engine) Dump() any {
	return packagesDump{
		Apps:    e.dumpKind(model.KindApplication),
		Modules: e.dumpKind(model.KindModule),
	}
}

// DumpByName returns one package; nil for an unknown name.
func (e *Engine) DumpByName(name string) any {
	if name == schedule.GlobalNode {
		return nil
	}

	d, ok := e.dumpPackage(name)
	if !ok {
		return nil
	}

	return d
}

// DumpGlobal returns Global's report, observation and info; nil without EnableModuleV2.
func (e *Engine) DumpGlobal() any {
	if e.global == nil {
		return nil
	}

	report, _ := e.core.Report(schedule.GlobalNode)

	return packageDump{
		Report:   report,
		Observed: e.observed.Get(schedule.GlobalNode),
		Info:     e.global.GetInfo(),
		Lanes:    e.hookRouter.Summary(schedule.GlobalNode),
	}
}

// Snapshots returns the hook snapshots of name's activated object (or Global's); false when there
// is none.
func (e *Engine) Snapshots(name string) (any, bool) {
	if name == schedule.GlobalNode {
		if e.global == nil {
			return nil, false
		}

		return snapshotsOf(e.global), true
	}

	rev, ok := e.core.Activated(name)
	if !ok {
		return nil, false
	}

	return snapshotsOf(rev.Package), true
}

// Render renders the chart of name's activated object in its release namespace.
func (e *Engine) Render(ctx context.Context, name string) (string, error) {
	rev, ok := e.core.Activated(name)
	if !ok {
		return "", errors.New("no package found")
	}

	return e.nelm.Render(ctx, model.ReleaseNamespace(rev.Image.Kind, name, app.NamespaceDeckhouse), rev.Package)
}

// DumpQueues returns the hook lanes of every package and Global, or name's when it is set
// (hooklane.Router.Dump).
func (e *Engine) DumpQueues(name string) any {
	return e.hookRouter.Dump(name)
}

// DumpScheduler is schedule.Scheduler.Dump.
func (e *Engine) DumpScheduler() any {
	return e.scheduler.Dump()
}

// DumpSchedulerByName is schedule.Scheduler.DumpByName.
func (e *Engine) DumpSchedulerByName(name string) any {
	return e.scheduler.DumpByName(name)
}

// ConvergeDump is what the convergence waits for; Packages is empty once Converged.
type ConvergeDump struct {
	Converged bool                           `json:"converged" yaml:"converged"`
	Paused    bool                           `json:"paused,omitempty" yaml:"paused,omitempty"` // Paused: before the boot gate or stopped.
	Packages  map[string]ConvergePackageDump `json:"packages,omitempty" yaml:"packages,omitempty"`
}

// ConvergePackageDump is one unconverged node: what it waits for and the failure its report holds.
type ConvergePackageDump struct {
	Order    schedule.Order `json:"order" yaml:"order"` // Order is the effective order the tier rule gates by.
	WaitsFor string         `json:"waitsFor,omitempty" yaml:"waitsFor,omitempty"`
	Error    string         `json:"error,omitempty" yaml:"error,omitempty"`
}

// Converge returns the global and module nodes core.Engine.Converged waits for, each with its
// failure; the scheduler and the reports are read apart, so it is not one snapshot.
func (e *Engine) Converge() any {
	unconverged := e.scheduler.Unconverged()
	synced := e.core.GlobalSynced()

	d := ConvergeDump{Paused: unconverged.Paused, Packages: make(map[string]ConvergePackageDump)}
	for name, n := range unconverged.Nodes {
		w := ConvergePackageDump{Order: n.Order, WaitsFor: n.WaitsFor}
		if report, ok := e.core.Report(name); ok {
			w.Error = failureOf(report)
		}

		d.Packages[name] = w
	}

	if !synced {
		w := d.Packages[schedule.GlobalNode]
		if w.WaitsFor == "" {
			w.WaitsFor = "hooks synchronization"
		}

		d.Packages[schedule.GlobalNode] = w
	}

	d.Converged = !d.Paused && len(d.Packages) == 0

	return d
}

// failureOf is the first failure of the report as "reason: message"; "" when nothing failed.
func failureOf(r model.Report) string {
	for _, c := range []model.Condition{r.Current.Configured, r.Current.HooksProcessed, r.Current.ManifestsApplied} {
		// ApplyingManifests is a release in progress, not a failure.
		if c.Status == model.ConditionFalse && c.Reason != model.ReasonApplyingManifests {
			return c.Reason + ": " + c.Message
		}
	}

	if r.Candidate.Step != "" {
		return r.Candidate.Reason + ": " + r.Candidate.Message
	}

	if r.RemoveError != "" {
		return model.ReasonReleaseDeleteFailed + ": " + r.RemoveError
	}

	return ""
}

func (e *Engine) dumpKind(kind model.Kind) map[string]packageDump {
	names := e.core.Names(kind)

	dumps := make(map[string]packageDump, len(names))
	for _, name := range names {
		if d, ok := e.dumpPackage(name); ok {
			dumps[name] = d
		}
	}

	return dumps
}

func (e *Engine) dumpPackage(name string) (packageDump, bool) {
	report, ok := e.core.Report(name)
	if !ok {
		return packageDump{}, false
	}

	d := packageDump{Report: report, Observed: e.observed.Get(name), Lanes: e.hookRouter.Summary(name)}
	if rev, ok := e.core.Activated(name); ok {
		d.Info = infoOf(rev.Package)
	}

	return d, true
}

// infoOf is the object's own description (apps.Info, modules.Info); nil for another object.
func infoOf(object model.Package) any {
	switch o := object.(type) {
	case *apps.Application:
		return o.GetInfo()
	case *modules.Module:
		return o.GetInfo()
	default:
		return nil
	}
}

// snapshotsOf reads the snapshots of object; empty while its hooks are not initialized.
func snapshotsOf(object any) map[string]any {
	s, ok := object.(snapshotter)
	if !ok {
		return map[string]any{}
	}

	return s.GetHookSnapshotsDump()
}
