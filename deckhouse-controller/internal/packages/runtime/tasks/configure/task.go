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

package configure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"go.opentelemetry.io/otel"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "configure"
)

// packageI abstracts the package operations needed for settings management.
type packageI interface {
	GetName() string
	// ApplySettings converts (if needed) and applies the package's configuration.
	// settingsVersion is the schema version from ModuleConfig.Spec.Version (0 if unset).
	ApplySettings(settingsVersion int, settings addonutils.Values) error
	// ValidateSettings converts (if needed) and validates settings against the
	// package's OpenAPI schema.
	ValidateSettings(ctx context.Context, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	// GetSettings returns the effective settings: user config merged with
	// config-schema defaults. Same payload exposed to templates as .Application.Settings.
	GetSettings() addonutils.Values
}

// task validates and applies new settings to a package.
// On success, sets ConditionSettingsValid to True.
// On failure, wraps errors with appropriate status conditions.
type task struct {
	pkg             packageI
	settings        addonutils.Values
	settingsVersion int

	status *status.Service

	logger *log.Logger
}

// NewTask creates a task that will validate and apply the given settings.
// settingsVersion is the schema version from ModuleConfig.Spec.Version (0 if unset).
func NewTask(pkg packageI, settings addonutils.Values, settingsVersion int, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		pkg:             pkg,
		settings:        settings,
		settingsVersion: settingsVersion,
		status:          status,
		logger:          logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "Configure"
}

// Execute validates settings and applies them to the package.
// Sets ConditionSettingsValid on success or delegates error handling to status service.
func (t *task) Execute(ctx context.Context) error {
	if err := t.applySettings(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), status.ConditionConfigured, err)
		return fmt.Errorf("configure: %w", err)
	}

	// Propagate the effective settings (user config + config-schema defaults)
	// to the internal status service. The CR status handler will later commit
	// them to Application.status.lastAppliedConfiguration alongside the version
	// when ConditionReadyInCluster becomes True.
	t.status.UpdateSettings(t.pkg.GetName(), t.pkg.GetSettings())

	return nil
}

// applySettings validates and applies settings to the package.
// Conversion to the latest schema version (if needed) is handled inside
// ValidateSettings and ApplySettings by the package itself.
func (t *task) applySettings(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "applySettings")
	defer span.End()

	t.logger.Debug("configure package")

	res, err := t.pkg.ValidateSettings(ctx, t.settingsVersion, t.settings)
	if err != nil {
		return status.NewError("ValidateFailed", err)
	}

	if !res.Valid {
		return status.NewError("ValidateFailed", errors.New(res.Message))
	}

	if err = t.pkg.ApplySettings(t.settingsVersion, t.settings); err != nil {
		return status.NewError("ConfigureFailed", err)
	}

	return nil
}
