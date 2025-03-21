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
	"net/http"

	"github.com/flant/shell-operator/pkg/metric"
	"sigs.k8s.io/controller-runtime/pkg/client"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
)

type registerer interface {
	RegisterHandler(route string, handler http.Handler)
}

type moduleStorage interface {
	GetModuleByName(name string) (*moduletypes.Module, error)
}

type moduleManager interface {
	IsModuleEnabled(name string) bool
}

// RegisterAdmissionHandlers registers validation webhook handlers for admission server built-in in addon-operator
func RegisterAdmissionHandlers(
	reg registerer,
	cli client.Client,
	mm moduleManager,
	validator *configtools.Validator,
	storage moduleStorage,
	metricStorage metric.Storage,
) {
	reg.RegisterHandler("/validate/v1alpha1/module-configs", moduleConfigValidationHandler(cli, storage, metricStorage, validator))
	reg.RegisterHandler("/validate/v1alpha1/modules", moduleValidationHandler())
	reg.RegisterHandler("/validate/v1/configuration-secret", kubernetesVersionHandler(mm))
	reg.RegisterHandler("/validate/v1alpha1/update-policies", updatePolicyHandler(cli))
}
