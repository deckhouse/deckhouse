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
	"fmt"

	addonutils "github.com/flant/addon-operator/pkg/utils"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	taskTracer = "apply-settings"
)

type manager interface {
	ApplySettings(name string, settings addonutils.Values) error
}

type task struct {
	packageName string
	settings    addonutils.Values

	manager manager

	logger *log.Logger
}

func NewTask(name string, settings addonutils.Values, manager manager, logger *log.Logger) queue.Task {
	return &task{
		packageName: name,
		settings:    settings,
		manager:     manager,
		logger:      logger.Named(taskTracer),
	}
}

func (t *task) String() string {
	return "ApplySettings"
}

func (t *task) Execute(_ context.Context) error {
	if err := t.manager.ApplySettings(t.packageName, t.settings); err != nil {
		return fmt.Errorf("apply settings: %w", err)
	}

	return nil
}
