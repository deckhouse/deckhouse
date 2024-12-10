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
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
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

type moduleDescriptor struct {
	constraints *versionmatcher.Matcher
}

type Extender struct {
	modulesStateHelper func() []string
	modules            map[string]moduleDescriptor
	logger             *log.Logger
}

func Instance() *Extender {
	once.Do(func() {
		instance = &Extender{
			logger:  log.Default().With("extender", Name),
			modules: make(map[string]moduleDescriptor),
		}
	})
	return instance
}

func (e *Extender) AddConstraint(name string, value map[string]string) error {
	module := e.modules[name]
	if module.constraints == nil {
		module.constraints = versionmatcher.New(false)
	}

	for dependency, version := range value {
		if err := module.constraints.AddConstraint(dependency, version); err != nil {
			return err
		}
	}
	e.modules[name] = module
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
	for module, descriptor := range e.modules {
		for _, constraintName := range descriptor.constraints.GetConstraintNames() {
			if constraintName == moduleName {
				hints = append(hints, module)
				break
			}
		}
	}

	return hints
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(_ string, _ map[string]string) (*bool, error) {
	// TODO
	return nil, nil
}

// SetModulesStateHelper implements StatefulExtender interface of the addon-operator
func (e *Extender) SetModulesStateHelper(f func() []string) {
	e.modulesStateHelper = f
}
