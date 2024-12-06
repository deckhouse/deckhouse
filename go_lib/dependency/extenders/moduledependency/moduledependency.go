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
	"strconv"
	"sync"

	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/scheduler/extenders"

	"github.com/deckhouse/deckhouse/go_lib/dependency/versionmatcher"
	"github.com/deckhouse/deckhouse/pkg/log"
)

const (
	Name              extenders.ExtenderName = "ModuleDependency"
	RequirementsField string                 = "modules"
)

var (
	instance *Extender
	once     sync.Once
)

var _ extenders.Extender = &Extender{}

type moduleDescriptor struct {
	version     semver.Version
	constraints *versionmatcher.Matcher
}

type Extender struct {
	modules map[string]moduleDescriptor
	logger  *log.Logger
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

func (e *Extender) AddConstraint(name string, value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		e.logger.Debugf("adding installed constraint for the '%s' module failed", name)
		return err
	}
	e.modules[name] = parsed
	e.logger.Debugf("installed constraint for the '%s' module is added", name)
	return nil
}

func (e *Extender) DeleteConstraint(name string) {
	// TODO
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
		for constraintName := range descriptor.constraints.GetConstraintNames() {
			if constraintName == moduleName {
				hints = append(hints, constraintName)
				break
			}
		}
	}

	return hints
}

// Filter implements Extender interface, it is used by scheduler in addon-operator
func (e *Extender) Filter(name string, _ map[string]string) (*bool, error) {
	// TODO
	return nil
}
