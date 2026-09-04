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

package module

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	"github.com/deckhouse/deckhouse/pkg/log"
)

// controllerName is the name the controller is registered under in the manager.
const controllerName = "d8-modulev2-settings-controller"

// settingsManager applies a module's settings-and-enabled change to the package runtime.
type settingsManager interface {
	UpdateModulesSettings(name string, settingsVersion int, settings addonutils.Values, maintenance string, enabled *bool)
}

// RegisterController registers the Module settings controller with the manager.
func RegisterController(
	sync *sync.WaitGroup,
	runtime ctrlmanager.Manager,
	manager settingsManager,
	logger *log.Logger,
) error {
	r := &reconciler{
		init:    sync,
		client:  runtime.GetClient(),
		manager: manager,
		logger:  logger.Named(controllerName),
	}

	if err := ctrl.NewControllerManagedBy(runtime).
		Named(controllerName).
		For(&v1alpha2.Module{}).
		WithEventFilter(predicate.Or(predicate.GenerationChangedPredicate{}, predicate.AnnotationChangedPredicate{})).
		Complete(r); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	return nil
}

// reconciler reconciles Module objects.
type reconciler struct {
	init    *sync.WaitGroup
	client  client.Client
	manager settingsManager
	logger  *log.Logger
}

// Reconcile applies a Module's settings-and-enabled change to the package runtime.
func (r *reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// wait until init
	r.init.Wait()

	r.logger.Debug("reconciling module", slog.String("name", req.Name))
	module := new(v1alpha2.Module)
	if err := r.client.Get(ctx, client.ObjectKey{Name: req.Name}, module); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Warn("module not found", slog.String("name", req.Name))
			return ctrl.Result{}, nil
		}

		r.logger.Error("failed to get module", slog.String("name", req.Name), log.Err(err))
		return ctrl.Result{}, fmt.Errorf("get: %w", err)
	}

	// handle delete event
	if !module.DeletionTimestamp.IsZero() {
		r.logger.Debug("module is being deleted", slog.String("name", req.Name))
		// no-op cleanup: this controller owns only the module settings, there is
		// nothing to tear down in the package runtime here. Just release the object.
		if err := r.removeFinalizer(ctx, module); err != nil {
			r.logger.Error("failed to remove finalizer", slog.String("name", req.Name), log.Err(err))
			return ctrl.Result{}, fmt.Errorf("remove finalizer: %w", err)
		}

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// removeFinalizer drops the controller's finalizer so the Module can be garbage-collected.
func (r *reconciler) removeFinalizer(ctx context.Context, module *v1alpha2.Module) error {
	return utils.Update[*v1alpha2.Module](ctx, r.client, module, func(module *v1alpha2.Module) bool {
		if controllerutil.ContainsFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered) {
			controllerutil.RemoveFinalizer(module, v1alpha2.ModuleFinalizerModuleRegistered)
			return true
		}

		return false
	})
}
