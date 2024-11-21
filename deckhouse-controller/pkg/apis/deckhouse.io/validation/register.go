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

	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
)

type registerer interface {
	RegisterHandler(route string, handler http.Handler)
}

type moduleStorage interface {
	GetModuleByName(name string) (*moduleloader.Module, error)
}

type moduleManager interface {
	IsModuleEnabled(name string) bool
}

// RegisterAdmissionHandlers registers validation webhook handlers for admission server built-in in addon-operator
func RegisterAdmissionHandlers(
	registerer registerer,
	client client.Client,
	mm moduleManager,
	configValidator *configtools.Validator,
	storage moduleStorage,
	metricStorage *metricstorage.MetricStorage) {
	registerer.RegisterHandler("/validate/v1alpha1/module-configs", moduleConfigValidationHandler(client, storage, metricStorage, configValidator))
	registerer.RegisterHandler("/validate/v1alpha1/modules", moduleValidationHandler())
	registerer.RegisterHandler("/validate/v1/configuration-secret", kubernetesVersionHandler(mm))
}
