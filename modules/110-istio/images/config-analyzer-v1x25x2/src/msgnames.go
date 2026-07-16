/*
Copyright 2026 Flant JSC

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

//go:build deckhouse_external

package main

import (
	"istio.io/istio/pkg/config/analysis/diag"
	"istio.io/istio/pkg/config/analysis/msg"
)

var codeToMessageTypeName map[string]string

func init() {
	codeToMessageTypeName = make(map[string]string, len(messageTypes))
	for _, item := range messageTypes {
		codeToMessageTypeName[item.mt.Code()] = item.name
	}
}

func messageTypeName(mt *diag.MessageType) string {
	if name, ok := codeToMessageTypeName[mt.Code()]; ok {
		return name
	}
	return mt.Code()
}

type messageTypeEntry struct {
	name string
	mt   *diag.MessageType
}

// messageTypes maps Istio analysis message codes to stable type names from Istio 1.25.2.
var messageTypes = []messageTypeEntry{
	{"InternalError", msg.InternalError},
	{"Deprecated", msg.Deprecated},
	{"ReferencedResourceNotFound", msg.ReferencedResourceNotFound},
	{"NamespaceNotInjected", msg.NamespaceNotInjected},
	{"PodMissingProxy", msg.PodMissingProxy},
	{"SchemaValidationError", msg.SchemaValidationError},
	{"MisplacedAnnotation", msg.MisplacedAnnotation},
	{"UnknownAnnotation", msg.UnknownAnnotation},
	{"ConflictingMeshGatewayVirtualServiceHosts", msg.ConflictingMeshGatewayVirtualServiceHosts},
	{"ConflictingSidecarWorkloadSelectors", msg.ConflictingSidecarWorkloadSelectors},
	{"MultipleSidecarsWithoutWorkloadSelectors", msg.MultipleSidecarsWithoutWorkloadSelectors},
	{"VirtualServiceDestinationPortSelectorRequired", msg.VirtualServiceDestinationPortSelectorRequired},
	{"DeploymentAssociatedToMultipleServices", msg.DeploymentAssociatedToMultipleServices},
	{"PortNameIsNotUnderNamingConvention", msg.PortNameIsNotUnderNamingConvention},
	{"NamespaceMultipleInjectionLabels", msg.NamespaceMultipleInjectionLabels},
	{"InvalidAnnotation", msg.InvalidAnnotation},
	{"UnknownMeshNetworksServiceRegistry", msg.UnknownMeshNetworksServiceRegistry},
	{"NoMatchingWorkloadsFound", msg.NoMatchingWorkloadsFound},
	{"NoServerCertificateVerificationDestinationLevel", msg.NoServerCertificateVerificationDestinationLevel},
	{"NoServerCertificateVerificationPortLevel", msg.NoServerCertificateVerificationPortLevel},
	{"VirtualServiceUnreachableRule", msg.VirtualServiceUnreachableRule},
	{"VirtualServiceIneffectiveMatch", msg.VirtualServiceIneffectiveMatch},
	{"VirtualServiceHostNotFoundInGateway", msg.VirtualServiceHostNotFoundInGateway},
	{"SchemaWarning", msg.SchemaWarning},
	{"ServiceEntryAddressesRequired", msg.ServiceEntryAddressesRequired},
	{"DeprecatedAnnotation", msg.DeprecatedAnnotation},
	{"AlphaAnnotation", msg.AlphaAnnotation},
	{"DeploymentConflictingPorts", msg.DeploymentConflictingPorts},
	{"GatewayDuplicateCertificate", msg.GatewayDuplicateCertificate},
	{"InvalidWebhook", msg.InvalidWebhook},
	{"IngressRouteRulesNotAffected", msg.IngressRouteRulesNotAffected},
	{"InsufficientPermissions", msg.InsufficientPermissions},
	{"UnsupportedKubernetesVersion", msg.UnsupportedKubernetesVersion},
	{"LocalhostListener", msg.LocalhostListener},
	{"InvalidApplicationUID", msg.InvalidApplicationUID},
	{"ConflictingGateways", msg.ConflictingGateways},
	{"ImageAutoWithoutInjectionWarning", msg.ImageAutoWithoutInjectionWarning},
	{"ImageAutoWithoutInjectionError", msg.ImageAutoWithoutInjectionError},
	{"NamespaceInjectionEnabledByDefault", msg.NamespaceInjectionEnabledByDefault},
	{"JwtClaimBasedRoutingWithoutRequestAuthN", msg.JwtClaimBasedRoutingWithoutRequestAuthN},
	{"ExternalNameServiceTypeInvalidPortName", msg.ExternalNameServiceTypeInvalidPortName},
	{"EnvoyFilterUsesRelativeOperation", msg.EnvoyFilterUsesRelativeOperation},
	{"EnvoyFilterUsesReplaceOperationIncorrectly", msg.EnvoyFilterUsesReplaceOperationIncorrectly},
	{"EnvoyFilterUsesAddOperationIncorrectly", msg.EnvoyFilterUsesAddOperationIncorrectly},
	{"EnvoyFilterUsesRemoveOperationIncorrectly", msg.EnvoyFilterUsesRemoveOperationIncorrectly},
	{"EnvoyFilterUsesRelativeOperationWithProxyVersion", msg.EnvoyFilterUsesRelativeOperationWithProxyVersion},
	{"UnsupportedGatewayAPIVersion", msg.UnsupportedGatewayAPIVersion},
	{"InvalidTelemetryProvider", msg.InvalidTelemetryProvider},
	{"PodsIstioProxyImageMismatchInNamespace", msg.PodsIstioProxyImageMismatchInNamespace},
	{"ConflictingTelemetryWorkloadSelectors", msg.ConflictingTelemetryWorkloadSelectors},
	{"MultipleTelemetriesWithoutWorkloadSelectors", msg.MultipleTelemetriesWithoutWorkloadSelectors},
	{"InvalidGatewayCredential", msg.InvalidGatewayCredential},
	{"GatewayPortNotDefinedOnService", msg.GatewayPortNotDefinedOnService},
	{"InvalidExternalControlPlaneConfig", msg.InvalidExternalControlPlaneConfig},
	{"ExternalControlPlaneAddressIsNotAHostname", msg.ExternalControlPlaneAddressIsNotAHostname},
	{"ReferencedInternalGateway", msg.ReferencedInternalGateway},
	{"IneffectiveSelector", msg.IneffectiveSelector},
	{"IneffectivePolicy", msg.IneffectivePolicy},
	{"UnknownUpgradeCompatibility", msg.UnknownUpgradeCompatibility},
	{"UpdateIncompatibility", msg.UpdateIncompatibility},
	{"MultiClusterInconsistentService", msg.MultiClusterInconsistentService},
	{"NegativeConditionStatus", msg.NegativeConditionStatus},
}
