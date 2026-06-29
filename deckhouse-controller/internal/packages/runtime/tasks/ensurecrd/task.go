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

package ensurecrd

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "ensurecrd"
)

// packageI is the minimal view of a package the installer needs: a name for
// logging and a filesystem path under which CRD manifests are located.
type packageI interface {
	// GetName returns the package name, used for logging.
	GetName() string
	// GetPath returns the package root path that contains the crds directory.
	GetPath() string
}

// task applies CRDs for the given package.
// On success, sets ConditionCustomResourcesApplied to True.
// On failure, wraps errors with appropriate status conditions.
type task struct {
	pkg packageI

	install install
	status  *status.Service

	logger *log.Logger
}

// install is a function that installs CRDs.
type install func(ctx context.Context, name, path string) error

// NewTask creates a task that installs CRDs for the given package.
func NewTask(pkg packageI, install install, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		pkg:     pkg,
		install: install,
		status:  status,
		logger:  logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "EnsureCRDs"
}

// Execute installs CRDs for the package.
// On success, sets ConditionCustomResourcesApplied to True.
// On failure, wraps errors with appropriate status conditions.
func (t *task) Execute(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "EnsureCRDs")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	if t.status.IsConditionStatusTrue(t.pkg.GetName(), status.ConditionCustomResourcesApplied) {
		t.logger.Debug("custom resources already ensured", slog.String("name", t.pkg.GetName()))
		return nil
	}

	t.logger.Debug("ensure CRDs", slog.String("name", t.pkg.GetName()))

	if err := t.install(ctx, t.pkg.GetName(), t.pkg.GetPath()); err != nil {
		// HandleError only reacts to *status.Error, so wrap the plain install
		// error to surface ConditionCustomResourcesApplied=False on the CR.
		err = status.NewError("CustomResourcesApplyFailed", err)
		t.status.HandleError(t.pkg.GetName(), status.ConditionCustomResourcesApplied, err)
		return fmt.Errorf("ensureCRDs: %w", err)
	}

	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionCustomResourcesApplied)

	return nil
}
