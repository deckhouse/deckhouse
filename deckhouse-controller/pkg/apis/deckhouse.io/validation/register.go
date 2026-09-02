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
	"context"
	"net/http"

	addonutils "github.com/flant/addon-operator/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/module-sdk/pkg/settingscheck"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/go_lib/configtools"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

type registerer interface {
	Register(path string, handler http.Handler)
}

type moduleStorage interface {
	GetModuleByName(name string) (*moduletypes.Module, error)
	GetModulesByExclusiveGroup(exclusiveGroup string) []string
}

type packageManager interface {
	ValidatePackageSettings(ctx context.Context, name string, settingsVersion int, settings addonutils.Values) (settingscheck.Result, error)
	CheckConstraints(name string, constraints schedule.Constraints) error
}

type moduleManager interface {
	IsModuleEnabled(name string) bool
	GetEnabledModuleNames() []string
}

// moduleDependencyExtender validates that a module can be enabled with respect to
// the constraints of already enabled dependent modules. It is satisfied by
// *moduledependency.Extender (extenders.ExtendersStack.GetModuleDependency()).
type moduleDependencyExtender interface {
	CheckEnabling(name string) error
}

// RegisterAdmissionHandlers registers validation webhook handlers on the webhook server built-in in the controller-runtime manager
func RegisterAdmissionHandlers(
	reg registerer,
	cli client.Client,
	mm moduleManager,
	pm packageManager,
	validator *configtools.Validator,
	storage moduleStorage,
	metricStorage metricsstorage.Storage,
	schemaStore *config.SchemaStore,
	settings *helpers.DeckhouseSettingsContainer,
	exts extenders.IExtendersStack,
	edition *d8edition.Edition,
) {
	reg.Register("/validate/v1/deckhouse-registry-secret", withInvalidReason(RegistrySecretHandler()))
	reg.Register("/validate/v1alpha1/module-configs", withInvalidReason(moduleConfigValidationHandler(cli, storage, metricStorage, mm, validator, settings, exts.GetModuleDependency(), edition)))
	reg.Register("/validate/v1alpha1/modules", withInvalidReason(moduleValidationHandler()))
	reg.Register("/validate/v1/configuration-secret", withInvalidReason(clusterConfigurationHandler(mm, cli, schemaStore)))
	reg.Register("/validate/v1/provider-configuration-secret", withInvalidReason(providerConfigurationHandler(schemaStore)))
	reg.Register("/validate/v1/static-configuration-secret", withInvalidReason(staticConfigurationHandler(schemaStore)))
	reg.Register("/validate/v1alpha1/update-policies", withInvalidReason(updatePolicyHandler(cli)))
	reg.Register("/validate/v1alpha1/deckhouse-releases", withInvalidReason(DeckhouseReleaseValidationHandler(cli, metricStorage, mm, exts)))
	reg.Register("/validate/v1alpha1/applications", withInvalidReason(applicationValidationHandler(cli, pm)))
}
