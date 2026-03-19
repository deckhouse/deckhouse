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

package applysettings

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
	taskTracer = "apply-settings"
)

// packageI abstracts the package operations needed for settings management.
type packageI interface {
	GetName() string
	// ApplySettings updates the package's in-memory configuration.
	ApplySettings(settings addonutils.Values) error
	// ValidateSettings checks settings against package-defined constraints.
	ValidateSettings(ctx context.Context, settings addonutils.Values) (settingscheck.Result, error)
}

// statusService provides condition updates and error handling for package status.
type statusService interface {
	SetConditionTrue(name string, cond status.ConditionType)
	HandleError(name string, err error)
}

// task validates and applies new settings to a package.
// On success, sets ConditionSettingsValid to True.
// On failure, wraps errors with appropriate status conditions.
type task struct {
	pkg      packageI
	settings addonutils.Values

	status statusService

	logger *log.Logger
}

// NewTask creates a task that will validate and apply the given settings.
func NewTask(pkg packageI, settings addonutils.Values, status statusService, logger *log.Logger) queue.Task {
	return &task{
		pkg:      pkg,
		settings: settings,
		status:   status,
		logger:   logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "ApplySettings"
}

// Execute validates settings and applies them to the package.
// Sets ConditionSettingsValid on success or delegates error handling to status service.
func (t *task) Execute(ctx context.Context) error {
	if err := t.applySettings(ctx); err != nil {
		t.status.HandleError(t.pkg.GetName(), err)
		return fmt.Errorf("apply settings: %w", err)
	}

	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionSettingsValid)

	return nil
}

// applySettings validates and applies settings to application
func (t *task) applySettings(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "ApplySettings")
	defer span.End()

	t.logger.Debug("apply settings")

	res, err := t.pkg.ValidateSettings(ctx, t.settings)
	if err != nil {
		return newApplySettingsErr(err)
	}

	if !res.Valid {
		return newApplySettingsErr(errors.New(res.Message))
	}

	if err = t.pkg.ApplySettings(t.settings); err != nil {
		return newApplySettingsErr(err)
	}

	return nil
}
