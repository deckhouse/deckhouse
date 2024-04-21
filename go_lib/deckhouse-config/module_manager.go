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
	"github.com/flant/addon-operator/pkg/module_manager/models/modules"
)

// ModuleManager interface is a part of addon-operator's ModuleManager interface
// with methods needed for deckhouse-config package.
type ModuleManager interface {
	IsModuleEnabled(modName string) bool
	GetGlobal() *modules.GlobalModule
	GetModule(modName string) *modules.BasicModule
	GetModuleNames() []string
	GetEnabledModuleNames() []string
	ValidateModule(module *modules.BasicModule) error
}
