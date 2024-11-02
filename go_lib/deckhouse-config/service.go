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

package deckhouse_config

import (
	"sync"

	"github.com/flant/addon-operator/pkg/module_manager/models/modules"

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// deckhouse-config Service is a middleware between ModuleManager instance and hooks to
// safely (in terms of addon-operator internals) retrieve information about modules.

var (
	serviceInstance *ConfigService
)

type ConfigService struct {
	moduleManager   ModuleManager
	lock            sync.Mutex
	possibleNames   set.Set
	configValidator *ConfigValidator
	statusReporter  *StatusReporter
}

// ModuleManager interface is a part of addon-operator's ModuleManager interface
// with methods needed for deckhouse-config package.
type ModuleManager interface {
	IsModuleEnabled(modName string) bool
	GetGlobal() *modules.GlobalModule
	GetModule(modName string) *modules.BasicModule
	GetModuleNames() []string
	GetEnabledModuleNames() []string
	GetUpdatedByExtender(string) (string, error)
}

func InitService(mm ModuleManager) {
	possibleNames := set.New()
	possibleNames.Add("global")

	serviceInstance = &ConfigService{
		moduleManager:   mm,
		possibleNames:   possibleNames,
		configValidator: NewConfigValidator(mm),
		statusReporter:  NewStatusReporter(mm),
	}
}

func IsServiceInited() bool {
	return serviceInstance != nil
}

func Service() *ConfigService {
	if serviceInstance == nil {
		panic("deckhouse-config Service is not initialized")
	}
	return serviceInstance
}

func (s *ConfigService) AddPossibleName(name string) {
	s.lock.Lock()
	s.possibleNames.Add(name)
	s.lock.Unlock()
}

func (s *ConfigService) PossibleNames() set.Set {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.possibleNames
}

func (s *ConfigService) ConfigValidator() *ConfigValidator {
	return s.configValidator
}

func (s *ConfigService) StatusReporter() *StatusReporter {
	return s.statusReporter
}
