/*
Copyright 2023 Flant JSC

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

package validation

import (
	addon_operator "github.com/flant/addon-operator/pkg/addon-operator"
	"github.com/flant/shell-operator/pkg/metric_storage"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/module"
)

type ModuleStorage interface {
	GetModuleByName(name string) (*module.DeckhouseModule, error)
}

// RegisterAdmissionHandlers register validation webhook handlers for admission server built-in in addon-operator
func RegisterAdmissionHandlers(operator *addon_operator.AddonOperator, moduleStorage ModuleStorage, metricStorage *metric_storage.MetricStorage) {
	operator.AdmissionServer.RegisterHandler("/validate/v1alpha1/module-configs", moduleConfigValidationHandler(moduleStorage, metricStorage))
	operator.AdmissionServer.RegisterHandler("/validate/v1alpha1/modules", moduleValidationHandler())
	operator.AdmissionServer.RegisterHandler("/validate/v1/configuration-secret", kubernetesVersionHandler(operator.ModuleManager))
}
