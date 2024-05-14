/*
Copyright 2022 Flant JSC

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

package mock

import (
	"fmt"
	"path/filepath"

	"github.com/flant/addon-operator/pkg/module_manager"
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
	"github.com/flant/addon-operator/pkg/utils"
	"k8s.io/utils/pointer"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

var (
	EnabledByBundle  = pointer.Bool(true)
	EnabledByScript  = pointer.Bool(true)
	DisabledByBundle = pointer.Bool(false)
	DisabledByScript = pointer.Bool(false)
)

// NewModuleManager returns mocked ModuleManager to test hooks
// without running values validations.
func NewModuleManager(mods ...*ModuleMock) *ModuleManagerMock {
	// Index input list of modules.
	modulesMap := map[string]*modules.BasicModule{}
	enabledModules := set.New()

	for _, mod := range mods {
		modulesMap[mod.module.GetName()] = mod.module
		if mod.enabled == nil || *mod.enabled {
			enabledModules.Add(mod.module.Name)
		}
	}

	return &ModuleManagerMock{
		modules:        modulesMap,
		enabledModules: enabledModules,
	}
}

type ModuleManagerMock struct {
	module_manager.ModuleManager
	modules        map[string]*modules.BasicModule
	enabledModules set.Set
}

func (m *ModuleManagerMock) IsModuleEnabled(name string) bool {
	return m.enabledModules.Has(name)
}

func (m *ModuleManagerMock) GetModule(name string) *modules.BasicModule {
	mod, has := m.modules[name]
	if has {
		return mod
	}
	return nil
}

func (m *ModuleManagerMock) GetModuleNames() []string {
	names := make([]string, 0)
	for modName := range m.modules {
		names = append(names, modName)
	}
	return names
}

type ModuleMock struct {
	module  *modules.BasicModule
	enabled *bool
}

func NewModule(name, path string, enabledByScript *bool) (*ModuleMock, error) {
	cb, vb, err := utils.ReadOpenAPIFiles(filepath.Join(path, "openapi"))
	if err != nil {
		return nil, fmt.Errorf("read open API files: %w", err)
	}

	bm, err := modules.NewBasicModule(name, "mockpath", 100, nil, cb, vb)
	if err != nil {
		return nil, fmt.Errorf("new basic module: %w", err)
	}

	return &ModuleMock{
		module:  bm,
		enabled: enabledByScript,
	}, nil
}
