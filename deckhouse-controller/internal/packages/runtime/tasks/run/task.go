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

package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	addontypes "github.com/flant/addon-operator/pkg/hook/types"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	shtypes "github.com/flant/shell-operator/pkg/hook/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-run"
)

// packageI abstracts package operations needed for the run cycle.
type packageI interface {
	GetName() string
	// GetValuesChecksum returns hash of current values to detect changes by hooks.
	GetValuesChecksum() string
	GetPath() string
	GetNelmValues() addonutils.Values
	GetExtraNelmValues() string
	// RunHooksByBinding executes hooks for BeforeHelm/AfterHelm bindings.
	RunHooksByBinding(ctx context.Context, binding shtypes.BindingType) error
}

// nelmI abstracts Helm operations and release monitoring.
type nelmI interface {
	HasMonitor(name string) bool
	// PauseMonitor stops monitoring during hooks (hooks may delete resources).
	PauseMonitor(name string)
	// ResumeMonitor restarts monitoring after run cycle completes.
	ResumeMonitor(name string)
	// Upgrade installs or upgrades the Helm release.
	Upgrade(ctx context.Context, namespace string, pkg nelm.Package) error
}

// statusService provides condition updates and error handling.
type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

// task executes the main package lifecycle: hooks and Helm release management.
// On success, sets HelmApplied, HooksProcessed, ReadyInRuntime, and ReadyInCluster.
type task struct {
	pkg       packageI
	namespace string

	nelm   nelmI
	status statusService

	logger *log.Logger
}

// NewTask creates a run task for executing the package's Helm lifecycle.
// This is the final task in the installation flow after Startup.
func NewTask(pkg packageI, ns string, nelm nelmI, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:       pkg,
		namespace: ns,
		nelm:      nelm,
		status:    status,
		logger:    logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "Run"
}

// Execute runs the full package lifecycle and sets success conditions.
// Lifecycle: BeforeHelm hooks → Helm upgrade → AfterHelm hooks → (re-upgrade if values changed).
func (t *task) Execute(ctx context.Context) error {
	t.logger.Debug("run package")
	if err := t.runPackage(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), err)
		return fmt.Errorf("run package: %w", err)
	}

	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionHelmApplied)
	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionHooksProcessed)
	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionReadyInRuntime)
	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionReadyInCluster)

	return nil
}

// runPackage executes the full package run cycle: BeforeHelm → Install/Upgrade → AfterHelm.
//
// Process:
//  1. Pause Helm resource monitoring
//  2. Run BeforeHelm hooks (can modify values or prepare resources)
//  3. Install or upgrade Helm release
//  4. Run AfterHelm hooks
//  5. If values changed during AfterHelm, trigger Helm upgrade
//  6. Resume Helm resource monitoring
func (t *task) runPackage(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "RunPackage")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	// monitor may not be created by this time
	if t.nelm.HasMonitor(t.pkg.GetName()) {
		t.logger.Debug("pause helm monitor")
		// Hooks can delete release resources, so pause resources monitor before run hooks.
		t.nelm.PauseMonitor(t.pkg.GetName())
		defer t.nelm.ResumeMonitor(t.pkg.GetName())
	}

	t.logger.Debug("run before helm hooks")

	if err := t.pkg.RunHooksByBinding(ctx, addontypes.BeforeHelm); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newBeforeHelmHookErr(err)
	}

	t.logger.Debug("run nelm upgrade")
	if err := t.nelm.Upgrade(ctx, t.namespace, t.pkg); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
		span.SetStatus(codes.Error, err.Error())
		return newHelmUpgradeErr(err)
	}

	t.logger.Debug("run after helm hooks")

	// Check if AfterHelm hooks modified values (would require nelm upgrade)
	oldChecksum := t.pkg.GetValuesChecksum()
	if err := t.pkg.RunHooksByBinding(ctx, addontypes.AfterHelm); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return newAfterHelmHookErr(err)
	}

	if oldChecksum != t.pkg.GetValuesChecksum() {
		if err := t.nelm.Upgrade(ctx, t.namespace, t.pkg); err != nil && !errors.Is(err, nelm.ErrPackageNotHelm) {
			span.SetStatus(codes.Error, err.Error())
			return newHelmUpgradeErr(err)
		}
	}

	return nil
}
