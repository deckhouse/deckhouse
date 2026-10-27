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

package current

import (
	"context"

	shtypes "github.com/flant/shell-operator/pkg/hook/types"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/hooklane"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/retry"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/resourcesize"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/addonutils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// Deps are the effects of every Current; shared by all of them and safe for concurrent use.
type Deps struct {
	Scheduler        publisher
	Nelm             nelmService
	Hooks            hookLanes
	Orphans          orphanPurger
	ModulesNamespace string
	Clock            retry.Clock // Clock nil means the wall clock.
	Logger           *log.Logger // Logger nil means a no-op logger.
}

// Config is what the package Reconciler passes to Start.
type Config struct {
	Name     string
	Kind     model.Kind
	Revision model.Revision // Revision is the revision to run; the package Reconciler owns its image.
	// Restored: started from a settings checkpoint, so the package has run (state.ran).
	Restored bool
}

// reportWriter is the only part of the Report Current may write.
type reportWriter interface {
	// UpdateCurrent is atomic: the loop and the apply step in the worker both call it.
	UpdateCurrent(f func(*model.CurrentReport))
}

type publisher interface {
	Publish(name string, facts schedule.Facts)
}

// nelmService is the part of nelm.Service Current uses: the release and its drift monitor.
type nelmService interface {
	// Upgrade: ErrPackageNotHelm without a chart; on cancellation it returns once an in-flight
	// release write is done. applying is called right before a changed release is applied;
	// drifted becomes the drift monitor's callback.
	Upgrade(ctx context.Context, namespace string, pkg nelm.Package, applying, drifted func()) error
	// Delete uninstalls the release; nil, and quick, when it does not exist.
	Delete(ctx context.Context, namespace, name string) error
	HasMonitor(name string) bool
	PauseMonitor(name string)
	ResumeMonitor(name string)
	RemoveMonitor(name string)
}

// hookLanes starts a package's hooks in their lanes (hooklane.Router.Start), where runtime hooks
// run in parallel with Current's operations.
type hookLanes interface {
	// Start routes the hooks' events to pkg's lanes, enables the bindings and waits (bounded by
	// ctx) for the synchronization; the lanes live under a child of parent. stop waits for the run
	// in flight; changed is not called after it. stop is non-nil on success: Current reads a nil one
	// as hooks not started and would sync again. On an error Start has stopped what it started.
	Start(ctx, parent context.Context, pkg hooklane.Object, changed func()) (func(), error)
}

// orphanPurger deletes what a removed application left beside its release.
type orphanPurger interface {
	// Purge is a no-op for a package that declares nothing or is not an application. An error is
	// retried with the uninstall.
	Purge(ctx context.Context, pkg model.Package) error
}

// packageObject is the package object as Current and its services use it.
type packageObject interface {
	nelm.Package
	hooklane.Object

	ValidateSettings(ctx context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	ApplySettings(settingsVersion int, settings addonutils.Values) error
	GetSettings() addonutils.Values
	SetMaintenance(state nelm.MaintenanceState)
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
}

// digested is implemented by objects that ship image digests (modules); their checksum is Current's
// Output.
type digested interface {
	GetDigests() map[string]string
}

// resourceSizer is implemented by objects with per-workload resource overrides (applications).
type resourceSizer interface {
	SetResourceRequests(requests []resourcesize.Request)
}
