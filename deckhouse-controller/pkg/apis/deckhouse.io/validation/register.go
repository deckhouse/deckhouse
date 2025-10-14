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

	"sigs.k8s.io/controller-runtime/pkg/client"

	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type registerer interface {
	RegisterHandler(route string, handler http.Handler)
}

type moduleStorage interface {
	GetModuleByName(name string) (*moduletypes.Module, error)
	GetModulesByExclusiveGroup(exclusiveGroup string) []string
}

type moduleManager interface {
	IsModuleEnabled(name string) bool
	GetEnabledModuleNames() []string
}

// RegisterAdmissionHandlers registers validation webhook handlers for admission server built-in in addon-operator
func RegisterAdmissionHandlers(
	reg registerer,
	cli client.Client,
	mm moduleManager,
	validator *configtools.Validator,
	storage moduleStorage,
	metricStorage metricsstorage.Storage,
	schemaStore *config.SchemaStore,
	settings *helpers.DeckhouseSettingsContainer,
	exts *extenders.ExtendersStack,
) {
	reg.RegisterHandler("/validate/v1alpha1/module-configs", moduleConfigValidationHandler(cli, storage, metricStorage, mm, validator, settings, exts))
	reg.RegisterHandler("/validate/v1alpha1/modules", moduleValidationHandler())
	reg.RegisterHandler("/validate/v1/configuration-secret", clusterConfigurationHandler(mm, cli, schemaStore))
	reg.RegisterHandler("/validate/v1/provider-configuration-secret", providerConfigurationHandler(schemaStore))
	reg.RegisterHandler("/validate/v1/static-configuration-secret", staticConfigurationHandler(schemaStore))
	reg.RegisterHandler("/validate/v1alpha1/update-policies", updatePolicyHandler(cli))
	reg.RegisterHandler("/validate/v1alpha1/deckhouse-releases", DeckhouseReleaseValidationHandler(cli, metricStorage, mm, exts))
}
