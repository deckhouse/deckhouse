/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package validation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	cpval "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation"
	cpvaladmission "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/admission"
	cpvalapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/api"
	cpvalprotocol "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/protocol"

	zicv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/instanceclass/v1"
	zpccv1 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/pcc/v1"
	zsettingsv2 "github.com/deckhouse/deckhouse/ee/se-plus/modules/030-cloud-provider-zvirt/pkg/api/settings/v2"
)

// State is the zVirt validation state: the generic validation state instantiated with the zVirt
// InstanceClass, ModuleConfig settings and providerClusterConfiguration types.
type State = cpvalapi.State[
	*zicv1.ZvirtInstanceClass,
	*zsettingsv2.ModuleConfigSettings,
	*zpccv1.ZvirtProviderClusterConfiguration,
]

// ProtocolStateBuilderFactory produces zVirt validation state builders for dhctl provider input.
type ProtocolStateBuilderFactory = cpvalprotocol.StateBuilderFactory[
	*zicv1.ZvirtInstanceClass,
	*zsettingsv2.ModuleConfigSettings,
	*zpccv1.ZvirtProviderClusterConfiguration,
]

// NewProtocolStateBuilderFactory creates a dhctl protocol state builder factory for the zVirt provider.
func NewProtocolStateBuilderFactory(config cpvalprotocol.StateBuilderConfig) *ProtocolStateBuilderFactory {
	return cpvalprotocol.NewStateBuilderFactory[
		*zicv1.ZvirtInstanceClass,
		*zsettingsv2.ModuleConfigSettings,
		*zpccv1.ZvirtProviderClusterConfiguration,
	](config)
}

// AdmissionStateBuilderFactory produces zVirt validation state builders for admission requests.
type AdmissionStateBuilderFactory = cpvaladmission.StateBuilderFactory[
	*zicv1.ZvirtInstanceClass,
	*zsettingsv2.ModuleConfigSettings,
	*zpccv1.ZvirtProviderClusterConfiguration,
]

// NewAdmissionStateBuilderFactory creates an in-cluster admission state builder factory for the zVirt provider.
func NewAdmissionStateBuilderFactory(client client.Client, config cpvaladmission.StateBuilderConfig) *AdmissionStateBuilderFactory {
	return cpvaladmission.NewStateBuilderFactory[
		*zicv1.ZvirtInstanceClass,
		*zsettingsv2.ModuleConfigSettings,
		*zpccv1.ZvirtProviderClusterConfiguration,
	](client, config)
}

var (
	// CredentialsValidator checks the managed credential Secret of the module. zVirt authenticates
	// with a login and a password, which the Secret carries as identity and secret.
	CredentialsValidator = &cpval.UserPasswordValidator{}
)
