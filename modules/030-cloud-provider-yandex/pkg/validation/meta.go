// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package validation contains Yandex validation types and constants.
package validation

import (
	"github.com/yandex-cloud/go-sdk/iamkey"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"

	ycicv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/instanceclass/v1"
	ycpccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/pcc/v1"
	ycsettingsv2 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-yandex/pkg/api/settings/v2"
)

// State is the Yandex validation state: the generic validation state instantiated
// with the Yandex InstanceClass, ModuleConfig settings and providerClusterConfiguration types.
type State = cpvalapi.State[
	*ycicv1.YandexInstanceClass,
	*ycsettingsv2.ModuleConfigSettings,
	*ycpccv1.YandexProviderClusterConfiguration,
]

// ProtocolStateBuilderFactory produces Yandex validation state builders for dhctl provider input.
type ProtocolStateBuilderFactory = cpvalprotocol.StateBuilderFactory[
	*ycicv1.YandexInstanceClass,
	*ycsettingsv2.ModuleConfigSettings,
	*ycpccv1.YandexProviderClusterConfiguration,
]

// NewProtocolStateBuilderFactory creates a dhctl protocol state builder factory for the Yandex provider.
func NewProtocolStateBuilderFactory(config cpvalprotocol.StateBuilderConfig) *ProtocolStateBuilderFactory {
	return cpvalprotocol.NewStateBuilderFactory[
		*ycicv1.YandexInstanceClass,
		*ycsettingsv2.ModuleConfigSettings,
		*ycpccv1.YandexProviderClusterConfiguration,
	](config)
}

// AdmissionStateBuilderFactory produces Yandex validation state builders for admission requests.
type AdmissionStateBuilderFactory = cpvaladmission.StateBuilderFactory[
	*ycicv1.YandexInstanceClass,
	*ycsettingsv2.ModuleConfigSettings,
	*ycpccv1.YandexProviderClusterConfiguration,
]

// NewAdmissionStateBuilderFactory creates an in-cluster admission state builder factory for the Yandex provider.
func NewAdmissionStateBuilderFactory(client client.Client, config cpvaladmission.StateBuilderConfig) *AdmissionStateBuilderFactory {
	return cpvaladmission.NewStateBuilderFactory[
		*ycicv1.YandexInstanceClass,
		*ycsettingsv2.ModuleConfigSettings,
		*ycpccv1.YandexProviderClusterConfiguration,
	](client, config)
}

var (
	ValidateServiceAccountFunc = func(data string) error {
		_, err := iamkey.ReadFromJSONBytes([]byte(data))
		return err
	}
	CredentialsValidator = &cpval.ServiceAccountValidator{
		ValidateContentFunc: ValidateServiceAccountFunc,
	}
	ExporterCredentialsValidator = &cpval.APITokenValidator{}
)
