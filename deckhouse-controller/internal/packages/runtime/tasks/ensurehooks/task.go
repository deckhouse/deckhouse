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
package ensurehooks

import (
	"context"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"github.com/flant/kube-client/manifest"
	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "ensurehooks"
)

// packageI is the minimal view of a package the installer needs: a name for
// logging and a filesystem path under which CRD manifests are located.
type packageI interface {
	GetName() string
	GetPath() string
	GetRuntimeValues() string
	GetValues() addonutils.Values
	// GetMaintenance reports the package's maintenance mode; the package itself
	// decides whether its resources must be reconciled.
	GetMaintenance() nelm.MaintenanceState
}

// nelmI abstracts Helm operations and release monitoring.
type nelmI interface {
	GetConversionWebhooks(ctx context.Context, namespace string, pkg nelm.Package) ([]manifest.Manifest, error)
}

// patcher abstracts object patching operations.
type patcher interface {
	ExecuteOperations(ops []sdkpkg.PatchCollectorOperation) error
}

type task struct {
	pkg packageI

	nelm          nelmI
	objectpatcher patcher

	logger *log.Logger
}

// NewTask creates a task that installs CRDs for the given package.
func NewTask(pkg packageI, nelm nelmI, objectpatcher patcher, logger *log.Logger) queue.Task {
	return &task{
		pkg:           pkg,
		nelm:          nelm,
		objectpatcher: objectpatcher,
		logger:        logger.Named(taskTracer).With(slog.String("name", pkg.GetName())),
	}
}

func (t *task) String() string {
	return "EnsureHooks"
}

// Execute ensure conversion webhooks for the given package.
func (t *task) Execute(ctx context.Context) error {
	webhooks, err := t.nelm.GetConversionWebhooks(ctx, app.NamespaceDeckhouse, t.pkg)
	if err != nil {
		return fmt.Errorf("get conversion webhooks: %w", err)
	}

	if len(webhooks) == 0 {
		return nil
	}

	t.logger.Info("ensure conversion webhooks", slog.Int("count", len(webhooks)))

	ops := make([]sdkpkg.PatchCollectorOperation, 0, len(webhooks))
	for _, hook := range webhooks {
		ops = append(ops, objectpatch.NewCreateOrUpdateOperation(map[string]any(hook)))
	}

	if err := t.objectpatcher.ExecuteOperations(ops); err != nil {
		return fmt.Errorf("apply conversion webhooks: %w", err)
	}

	return nil
}
