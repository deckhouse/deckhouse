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

package ensurehooks

import (
	"context"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "ensurehooks"
)

// install applies the package's ConversionWebhook resources into the cluster.
type install func(ctx context.Context, namespace string, pkg nelm.Package) error

// task applies the package's ConversionWebhook resources before its templates,
// so the webhook-handler patches the target CRDs before the Run task creates the
// custom resources that need converting.
// On success, sets ConditionConversionWebhooksApplied to True.
type task struct {
	pkg       nelm.Package
	namespace string

	install install
	status  *status.Service

	logger *log.Logger
}

// NewTask creates a task that installs the package's ConversionWebhook resources.
func NewTask(pkg nelm.Package, namespace string, install install, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		pkg:       pkg,
		namespace: namespace,
		install:   install,
		status:    status,
		logger:    logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "EnsureHooks"
}

// Execute applies the package's ConversionWebhook resources.
// On success, sets ConditionConversionWebhooksApplied to True.
// On failure, wraps errors with the matching status condition.
func (t *task) Execute(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "EnsureHooks")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	if t.status.IsConditionStatusTrue(t.pkg.GetName(), status.ConditionConversionWebhooksApplied) {
		t.logger.Debug("conversion webhooks already ensured")
		return nil
	}

	t.logger.Debug("ensure conversion webhooks")

	if err := t.install(ctx, t.namespace, t.pkg); err != nil {
		// HandleError only reacts to *status.Error, so wrap the plain install
		// error to surface ConditionConversionWebhooksApplied=False on the CR.
		err = status.NewError("ConversionWebhooksApplyFailed", err)
		t.status.HandleError(t.pkg.GetName(), status.ConditionConversionWebhooksApplied, err)

		return fmt.Errorf("ensureHooks: %w", err)
	}

	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionConversionWebhooksApplied)

	return nil
}
