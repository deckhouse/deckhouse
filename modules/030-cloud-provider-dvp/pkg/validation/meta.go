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

// Package validation contains DVP validation types and constants.
package validation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"

	dvpicv1alpha1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/instanceclass/v1alpha1"
	dvppccv1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/pcc/v1"
	dvpsettings "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/pkg/api/settings"
)

// State is the DVP validation state: the generic validation state instantiated
// with the DVP InstanceClass, ModuleConfig settings and providerClusterConfiguration types.
type State = cpvalapi.State[
	*dvpicv1alpha1.DVPInstanceClass,
	*dvpsettings.ModuleConfigSettings,
	*dvppccv1.DVPProviderClusterConfiguration,
]

// ProtocolStateBuilderFactory produces DVP validation state builders for dhctl provider input.
type ProtocolStateBuilderFactory = cpvalprotocol.StateBuilderFactory[
	*dvpicv1alpha1.DVPInstanceClass,
	*dvpsettings.ModuleConfigSettings,
	*dvppccv1.DVPProviderClusterConfiguration,
]

// NewProtocolStateBuilderFactory creates a dhctl protocol state builder factory for the DVP provider.
func NewProtocolStateBuilderFactory(config cpvalprotocol.StateBuilderConfig) *ProtocolStateBuilderFactory {
	return cpvalprotocol.NewStateBuilderFactory[
		*dvpicv1alpha1.DVPInstanceClass,
		*dvpsettings.ModuleConfigSettings,
		*dvppccv1.DVPProviderClusterConfiguration,
	](config)
}

// AdmissionStateBuilderFactory produces DVP validation state builders for admission requests.
type AdmissionStateBuilderFactory = cpvaladmission.StateBuilderFactory[
	*dvpicv1alpha1.DVPInstanceClass,
	*dvpsettings.ModuleConfigSettings,
	*dvppccv1.DVPProviderClusterConfiguration,
]

// NewAdmissionStateBuilderFactory creates an in-cluster admission state builder factory for the DVP provider.
func NewAdmissionStateBuilderFactory(client client.Client, config cpvaladmission.StateBuilderConfig) *AdmissionStateBuilderFactory {
	return cpvaladmission.NewStateBuilderFactory[
		*dvpicv1alpha1.DVPInstanceClass,
		*dvpsettings.ModuleConfigSettings,
		*dvppccv1.DVPProviderClusterConfiguration,
	](client, config)
}

var (
	CredentialsValidator = &cpval.KubeconfigValidator{}
)
