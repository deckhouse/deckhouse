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

package load

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/status"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/registry"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "package-load"
)

type manager interface {
	LoadPackage(ctx context.Context, registry registry.Registry, namespace, name string) (string, error)
	ApplySettings(name string, settings addonutils.Values) error
}

type statusService interface {
	SetConditionTrue(name string, conditionName status.ConditionName)
	HandleError(name string, err error)
	SetVersion(name string, version string)
}

type task struct {
	packageName string
	namespace   string
	registry    registry.Registry
	settings    addonutils.Values

	manager manager
	status  statusService

	logger *log.Logger
}

func NewTask(reg registry.Registry, namespace, name string, settings addonutils.Values, status statusService, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		namespace:   namespace,
		registry:    reg,
		settings:    settings,
		manager:     manager,
		status:      status,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "Load"
}

func (t *task) Execute(ctx context.Context) error {
	// Load package into package manager (parse hooks, values, chart)
	t.logger.Debug("load package", slog.String("name", t.packageName))
	version, err := t.manager.LoadPackage(ctx, t.registry, t.namespace, t.packageName)
	if err != nil {
		t.status.HandleError(t.packageName, err)
		return fmt.Errorf("load package: %w", err)
	}

	t.logger.Debug("apply initial settings", slog.String("name", t.packageName))
	if err = t.manager.ApplySettings(t.packageName, t.settings); err != nil {
		t.status.HandleError(t.packageName, err)
		return fmt.Errorf("apply initial settings: %w", err)
	}

	t.status.SetConditionTrue(t.packageName, status.ConditionSettingsValid)
	t.status.SetVersion(t.packageName, version)

	t.status.HandleError(t.packageName, &status.Error{
		Err: errors.New("wait for converge done"),
		Conditions: []status.Condition{
			{
				Name:    status.ConditionReadyInRuntime,
				Status:  metav1.ConditionFalse,
				Reason:  "WaitConverge",
				Message: fmt.Sprintf("wait for converge done"),
			},
		},
	})

	return nil
}
