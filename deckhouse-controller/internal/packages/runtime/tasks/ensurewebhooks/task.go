// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package ensurewebhooks applies a package's ConversionWebhook resources ahead of
// its Helm release, so a conversion is registered before the release applies the
// custom resources it converts.
package ensurewebhooks

import (
	"context"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/kube-client/manifest"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "ensurewebhooks"

	// conditionReasonRenderFailed reports a chart that could not be rendered.
	conditionReasonRenderFailed status.ConditionReason = "WebhooksRenderFailed"
	// conditionReasonApplyFailed reports webhooks that rendered but were not applied.
	conditionReasonApplyFailed status.ConditionReason = "WebhooksApplyFailed"
)

// packageI is the view of a package needed to render its chart: a name for
// logging and the status key, plus the path and values the render runs against.
type packageI interface {
	// GetName returns the package name, used for logging and the status key.
	GetName() string
	// GetPath returns the package root path that contains the Helm chart.
	GetPath() string
	GetRuntimeValues() string
	GetValues() addonutils.Values
	GetMaintenance() nelm.MaintenanceState
}

// nelmI renders a package and returns only its ConversionWebhook manifests.
// A package without a Helm chart yields no webhooks and no error.
type nelmI interface {
	GetConversionWebhooks(ctx context.Context, namespace string, pkg nelm.Package) ([]manifest.Manifest, error)
}

// patcher applies the rendered webhooks to the cluster.
type patcher interface {
	ExecuteOperations(ops []sdkpkg.PatchCollectorOperation) error
}

// task applies the package's conversion webhooks.
// On success, sets ConditionWebhooksEnsured to True.
// On failure, wraps errors with appropriate status conditions.
type task struct {
	pkg packageI

	nelm          nelmI
	objectpatcher patcher
	status        *status.Service

	logger *log.Logger
}

// NewTask creates a task that applies the conversion webhooks of the given package.
func NewTask(pkg packageI, nelm nelmI, objectpatcher patcher, status *status.Service, logger *log.Logger) queue.Task {
	return &task{
		pkg:           pkg,
		nelm:          nelm,
		objectpatcher: objectpatcher,
		status:        status,
		logger:        logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "EnsureWebhooks"
}

// Execute applies the package's conversion webhooks.
//
// The condition short-circuit keeps the render off the hot path: NewStatus resets
// the conditions whenever the package changes (version, settings, maintenance), so
// a package is rendered again exactly when its webhooks may have changed.
//
// A package with no chart or no ConversionWebhook resources counts as ensured.
func (t *task) Execute(ctx context.Context) error {
	ctx, span := otel.Tracer(taskTracer).Start(ctx, "EnsureWebhooks")
	defer span.End()

	span.SetAttributes(attribute.String("name", t.pkg.GetName()))

	if t.status.IsConditionStatusTrue(t.pkg.GetName(), status.ConditionWebhooksEnsured) {
		t.logger.Debug("conversion webhooks already ensured")
		return nil
	}

	webhooks, err := t.nelm.GetConversionWebhooks(ctx, app.NamespaceDeckhouse, t.pkg)
	if err != nil {
		// HandleError only reacts to *status.Error, so wrap the plain render error
		// to record ConditionWebhooksEnsured=False.
		err = status.NewError(conditionReasonRenderFailed, err)
		t.status.HandleError(t.pkg.GetName(), status.ConditionWebhooksEnsured, err)

		return fmt.Errorf("get conversion webhooks: %w", err)
	}

	if len(webhooks) > 0 {
		t.logger.Info("ensure conversion webhooks", slog.Int("count", len(webhooks)))

		ops := make([]sdkpkg.PatchCollectorOperation, 0, len(webhooks))
		for _, webhook := range webhooks {
			ops = append(ops, objectpatch.NewCreateOrUpdateOperation(map[string]any(webhook)))
		}

		if err := t.objectpatcher.ExecuteOperations(ops); err != nil {
			err = status.NewError(conditionReasonApplyFailed, err)
			t.status.HandleError(t.pkg.GetName(), status.ConditionWebhooksEnsured, err)

			return fmt.Errorf("apply conversion webhooks: %w", err)
		}
	}

	t.status.SetConditionTrue(t.pkg.GetName(), status.ConditionWebhooksEnsured)

	return nil
}
