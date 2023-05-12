package module

import (
	"github.com/flant/addon-operator/pkg/module_manager"
)

var (
	builderProxy module_manager.ModuleBuilder
)

func RegisterBuilder(builder module_manager.ModuleBuilder) {
	builderProxy = builder
}

func Builder() module_manager.ModuleBuilder {
	return builderProxy
}
