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

package editionenabled

import (
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"
	"k8s.io/utils/ptr"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name extenders.ExtenderName = "EditionEnabled"
)

var _ extenders.Extender = &Extender{}

type Extender struct {
	edition string
	bundle  string
	modules map[string]*moduletypes.ModuleAccessibility
	logger  *log.Logger
}

func New(edition, bundle string, logger *log.Logger) *Extender {
	return &Extender{
		edition: edition,
		bundle:  bundle,
		modules: make(map[string]*moduletypes.ModuleAccessibility),
		logger:  logger,
	}
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return false
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	module, ok := e.modules[name]
	if !ok {
		e.logger.Debug("module skipped", slog.String("module", name))
		return nil, nil
	}

	if module.IsEnabled(e.edition, e.bundle) {
		e.logger.Debug("module enabled by bundle",
			slog.String("module", name),
			slog.String("bundle", e.bundle))
		return ptr.To(true), nil
	}

	e.logger.Debug("module disabled by bundle",
		slog.String("module", name),
		slog.String("bundle", e.bundle))

	return ptr.To(false), fmt.Errorf("disabled in '%s' bundle", e.bundle)
}

func (e *Extender) AddModule(name string, access *moduletypes.ModuleAccessibility) {
	e.modules[name] = access
}
