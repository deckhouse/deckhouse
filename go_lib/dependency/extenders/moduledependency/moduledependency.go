/*
Copyright 2024 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package moduledependency

import (
	"slices"
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"

	"k8s.io/utils/ptr"
)

const (
	Name extenders.ExtenderName = "ModuleDependency"
)

var (
	instance *Extender
	once     sync.Once
)

var (
	_ extenders.Extender            = &Extender{}
	_ extenders.TopologicalExtender = &Extender{}
	_ extenders.StatefulExtender    = &Extender{}
)

type Extender struct {
	modulesVersionHelper func(moduleName string) (string, error)
	modulesStateHelper   func() []string
	modules              map[string]*versionmatcher.Matcher
	logger               *log.Logger
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:  log.Default().With("extender", Name),
			modules: make(map[string]*versionmatcher.Matcher),
		}
	})
	return instance
}

func (e *Extender) SetModulesVersionHelper(f func(moduleName string) (string, error)) {
	e.modulesVersionHelper = f
}

func (e *Extender) AddConstraint(name string, value map[string]string) error {
	constraints := e.modules[name]
	if constraints == nil {
		constraints = versionmatcher.New(false)
	}

	for dependency, version := range value {
		if err := constraints.AddConstraint(dependency, version); err != nil {
			return err
		}
	}
	e.modules[name] = constraints
	e.logger.Debugf("installed constraint for the '%s' module is added", name)

	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	delete(e.modules, name)
}

// Name implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Name() extenders.ExtenderName {
	return Name
}

// IsTerminator implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) IsTerminator() bool {
	return true
}

// GetTopologicalHints implements TopologicalExtender interface of the addon-operator
func (e *Extender) GetTopologicalHints(moduleName string) []string {
	hints := make([]string, 0)
	if constraints, found := e.modules[moduleName]; found {
		hints = append(hints, constraints.GetConstraintNames()...)
	}

	return hints
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(moduleName string, _ map[string]string) (*bool, error) {
	constraints, found := e.modules[moduleName]
	if !found {
		return nil, nil
	}

	enabledModules := e.modulesStateHelper()

	for _, parentModule := range constraints.GetConstraintNames() {
		if !slices.Contains(enabledModules, parentModule) {
			return ptr.To(false), nil
		}
	}

	// TODO: check modules' versions
	return ptr.To(true), nil
}

// SetModulesStateHelper implements StatefulExtender interface of the addon-operator
func (e *Extender) SetModulesStateHelper(f func() []string) {
	e.modulesStateHelper = f
}
