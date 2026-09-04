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
	"sync"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
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

// Reconcile is a stub and does nothing yet.
func (r *reconciler) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
