package manager

import (
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type configurator interface {
	configurateOptions(*controllerruntime.Options)
	configurateRuntimeManager(manager.Manager) error
}
