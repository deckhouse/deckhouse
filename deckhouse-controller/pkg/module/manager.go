package module

import addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"

// Manager interface is a part of addon-operator's ModuleManager interface
type Manager interface {
	IsModuleEnabled(modName string) bool
	GetGlobal() *addonmodules.GlobalModule
	GetModule(modName string) *addonmodules.BasicModule
	GetModuleNames() []string
	GetEnabledModuleNames() []string
	GetUpdatedByExtender(string) (string, error)
	DisableModuleHooks(moduleName string)
	RunModuleWithNewOpenAPISchema(moduleName, moduleSource, modulePath string) error
}
