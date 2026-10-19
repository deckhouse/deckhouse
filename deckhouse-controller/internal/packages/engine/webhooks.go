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

package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	objectpatch "github.com/flant/shell-operator/pkg/kube/object_patch"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/lifecycle/model"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/nelm"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// conversionWebhooks is global.Deps.Webhooks: it renders a module's ConversionWebhook resources
// and applies them with the object patcher. It lives here because both talk Kubernetes.
type conversionWebhooks struct {
	nelm    *nelm.Service
	patcher *objectpatch.ObjectPatcher
	logger  *log.Logger

	mu sync.Mutex
	// ensured is the object whose webhooks were last applied, by module name, so a global run
	// does not render every module again. Every load creates a new object, so a new revision is
	// rendered again.
	ensured map[string]model.Package
}

func newConversionWebhooks(nelmService *nelm.Service, patcher *objectpatch.ObjectPatcher, logger *log.Logger) *conversionWebhooks {
	return &conversionWebhooks{
		nelm:    nelmService,
		patcher: patcher,
		logger:  logger.Named("conversion-webhooks"),
		ensured: make(map[string]model.Package),
	}
}

// EnsureWebhooks applies the conversion webhooks the view's chart declares, if any.
func (w *conversionWebhooks) EnsureWebhooks(ctx context.Context, view model.ModuleView) error {
	w.mu.Lock()
	done := w.ensured[view.Name] == view.Package
	w.mu.Unlock()

	if done {
		return nil
	}

	declared, err := w.nelm.HasConversionWebhook(view.Path)
	if err != nil {
		return fmt.Errorf("scan chart: %w", err)
	}

	if declared {
		webhooks, err := w.nelm.GetConversionWebhooks(ctx, app.NamespaceDeckhouse, view.Package)
		if err != nil {
			return fmt.Errorf("render conversion webhooks: %w", err)
		}

		if len(webhooks) > 0 {
			w.logger.Info("ensure conversion webhooks", slog.String("name", view.Name), slog.Int("count", len(webhooks)))

			ops := make([]sdkpkg.PatchCollectorOperation, 0, len(webhooks))
			for _, webhook := range webhooks {
				ops = append(ops, objectpatch.NewCreateOrUpdateOperation(map[string]any(webhook)))
			}

			if err := w.patcher.ExecuteOperations(ops); err != nil {
				return fmt.Errorf("apply conversion webhooks: %w", err)
			}
		}
	}

	w.mu.Lock()
	w.ensured[view.Name] = view.Package
	w.mu.Unlock()

	return nil
}
