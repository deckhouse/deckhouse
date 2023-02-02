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

	"github.com/deckhouse/deckhouse/go_lib/set"
)

// deckhouse-config Service is a middleware between ModuleManager instance and hooks to
// safely (in terms of addon-operator internals) retrieve information about modules.

var (
	serviceInstance     *ConfigService
	serviceInstanceLock sync.Mutex
)

func InitService(mm ModuleManager) {
	serviceInstanceLock.Lock()
	defer serviceInstanceLock.Unlock()

	possibleNames := set.New(mm.GetModuleNames()...)
	possibleNames.Add("global")

	serviceInstance = &ConfigService{
		moduleManager:   mm,
		possibleNames:   possibleNames,
		transformer:     NewTransformer(possibleNames),
		configValidator: NewConfigValidator(mm.GetValuesValidator()),
		statusReporter:  NewModuleInfo(mm, possibleNames),
		externalNames:   make(map[string]string),
	}
}

func Service() *ConfigService {
	if serviceInstance == nil {
		panic("deckhouse-config Service is not initialized")
	}
	return serviceInstance
}

type ConfigService struct {
	moduleManager   ModuleManager
	possibleNames   set.Set
	transformer     *Transformer
	configValidator *ConfigValidator
	statusReporter  *StatusReporter

	externalNamesLock sync.RWMutex
	externalNames     map[string]string
}

func (srv *ConfigService) PossibleNames() set.Set {
	return srv.possibleNames
}

func (srv *ConfigService) Transformer() *Transformer {
	return srv.transformer
}

func (srv *ConfigService) ConfigValidator() *ConfigValidator {
	return srv.configValidator
}

func (srv *ConfigService) StatusReporter() *StatusReporter {
	return srv.statusReporter
}

func (srv *ConfigService) SetExternalNames(allExternalNamesToRepos map[string]string) {
	srv.externalNamesLock.Lock()
	defer srv.externalNamesLock.Unlock()

	srv.externalNames = allExternalNamesToRepos
}

func (srv *ConfigService) AddExternalModuleName(moduleName, moduleSource string) {
	srv.externalNamesLock.Lock()
	defer srv.externalNamesLock.Unlock()

	srv.externalNames[moduleName] = moduleSource
}

func (srv *ConfigService) ExternalNames() map[string]string {
	srv.externalNamesLock.RLock()
	defer srv.externalNamesLock.RUnlock()

	res := make(map[string]string)
	for module, repo := range srv.externalNames {
		res[module] = repo
	}

	return res
}
