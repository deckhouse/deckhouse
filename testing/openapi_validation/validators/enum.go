/*
Copyright 2021 Flant JSC

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

package validators

import (
	"fmt"
	"regexp"
	"unicode"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

var (
	// openapi key excludes by file
	fileExcludes = map[string][]string{
		// all files
		"*": {"apiVersions[*].openAPISpec.properties.apiVersion"},
		// exclude zone - ru-center-1, ru-center-2, ru-center-3
		"candi/cloud-providers/yandex/openapi/cluster_configuration.yaml": {
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.zones.items",
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.zones.items",
			"apiVersions[0].openAPISpec.properties.zones.items",
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.instanceClass.properties.diskType",
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.instanceClass.properties.diskType",
		},
		// disk types - gp2.,..
		"candi/cloud-providers/aws/openapi/cluster_configuration.yaml": {
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.instanceClass.properties.diskType",
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.instanceClass.properties.diskType",
			"apiVersions[0].openAPISpec.properties.withNAT.properties.bastionInstance.properties.instanceClass.properties.diskType",
		},
		// disk types: pd-standard, pd-ssd, ...
		"candi/cloud-providers/gcp/openapi/instance_class.yaml": {
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.diskType",
		},
		// disk types: network-ssd, network-hdd
		"candi/cloud-providers/yandex/openapi/instance_class.yaml": {
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.diskType",
			// v1alpha1 : SOFTWARE_ACCELERATED - migrated in v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.networkType",
		},
		"candi/openapi/cluster_configuration.yaml": {
			// vSphere
			"apiVersions[0].openAPISpec.properties.cloud.properties.provider",
			// encryptionAlgorithm
			"apiVersions[0].openAPISpec.properties.encryptionAlgorithm",
		},
		"global-hooks/openapi/values.yaml": {
			// from openapispec
			"properties.clusterConfiguration.properties.apiVersion",
			"properties.clusterConfiguration.properties.cloud.properties.provider",
			// http and https
			"properties.modulesImages.properties.registry.properties.scheme",
			// allow SE-plus edition
			"properties.deckhouseEdition",
		},
		"modules/150-user-authn/crds/dex-provider.yaml": {
			// v1alpha1 migrated to v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.github.properties.teamNameField",
		},
		"modules/300-prometheus/crds/grafanaadditionaldatasources.yaml": {
			// v1alpha1 migrated to v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.access",
		},
		"modules/015-admission-policy-engine/crds/operation-policy.yaml": {
			// probes are inherited from Kubernetes
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredProbes.items",
			// requests and limits are cpu and memory, they are taken from kubernetes
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredResources.properties.requests.items",
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredResources.properties.limits.items",
		},
		"modules/015-admission-policy-engine/crds/security-policy.yaml": {
			// volumes are inherited from kubernetes
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.allowedVolumes.items",
			// capabilities names are hardcoded, it's not ours
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.allowedCapabilities.items",
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.policies.properties.requiredDropCapabilities.items",
		},
		"modules/015-admission-policy-engine/openapi/values.yaml": {
			// enforcement actions are discovered from label values and should be propagated further into the helm chart as is
			"properties.internal.properties.podSecurityStandards.properties.enforcementActions.items",
		},
		"modules/030-cloud-provider-azure/openapi/config-values.yaml": {
			// ignore Azure disk types
			"properties.storageClass.properties.provision.items.properties.type",
			"properties.storageClass.properties.provision.items.oneOf[*].properties.type",
		},
		"modules/030-cloud-provider-aws/openapi/config-values.yaml": {
			// ignore AWS disk types
			"properties.storageClass.properties.provision.items.properties.type",
			"properties.storageClass.properties.provision.items.oneOf[*].properties.type",
		},
		"modules/030-cloud-provider-openstack/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.discoveryData.properties.apiVersion",
		},
		// for local tests run
		"ee/modules/030-cloud-provider-openstack/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.discoveryData.properties.apiVersion",
		},
		"modules/030-cloud-provider-aws/openapi/values.yaml": {
			// ignore AWS disk types
			"properties.internal.properties.storageClasses.items.oneOf[*].properties.type",
		},
		"modules/030-cloud-provider-vcd/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.discoveryData.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.edgeGateway.properties.type",
			"properties.internal.properties.providerClusterConfiguration.properties.edgeGateway.properties.NSX-V.properties.externalNetworkType",
		},
		"modules/030-cloud-provider-zvirt/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
		},
		"modules/030-cloud-provider-dynamix/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
		},
		"modules/030-cloud-provider-dvp/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.masterNodeGroup.properties.instanceClass.properties.virtualMachine.properties.cpu.properties.coreFraction",
			"properties.internal.properties.providerClusterConfiguration.properties.nodeGroups.items.properties.instanceClass.properties.virtualMachine.properties.cpu.properties.coreFraction",
		},
		"candi/cloud-providers/dvp/openapi/instance_class.yaml": {
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.virtualMachine.properties.cpu.properties.coreFraction",
		},
		"candi/cloud-providers/dvp/openapi/cluster_configuration.yaml": {
			"apiVersions[0].openAPISpec.properties.nodeGroups.items.properties.instanceClass.properties.virtualMachine.properties.cpu.properties.coreFraction",
			"apiVersions[0].openAPISpec.properties.masterNodeGroup.properties.instanceClass.properties.virtualMachine.properties.cpu.properties.coreFraction",
		},
		"modules/030-cloud-provider-huaweicloud/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
		},
		"ee/modules/030-cloud-provider-vcd/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.discoveryData.properties.apiVersion",
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.edgeGateway.properties.type",
			"properties.internal.properties.providerClusterConfiguration.properties.edgeGateway.properties.NSX-V.properties.externalNetworkType",
		},
		"modules/030-cloud-provider-yandex/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.zones.items",
			"properties.internal.properties.providerClusterConfiguration.properties.nodeGroups.items.properties.zones.items",
			"properties.internal.properties.providerClusterConfiguration.properties.masterNodeGroup.properties.zones.items",
		},
		"modules/035-cni-flannel/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.podNetworkMode",
		},
		"modules/040-node-manager/crds/nodegroupconfiguration.yaml": {
			// ignore bundles name values
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.bundles.items",
		},
		"modules/042-kube-dns/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.specificNodeType",
		},
		"modules/300-prometheus/openapi/values.yaml": {
			// grafana constant in internal values
			"properties.internal.properties.grafana.properties.alertsChannelsConfig.properties.notifiers.items.properties.type",
		},
		"modules/402-ingress-nginx/crds/ingress-nginx.yaml": {
			// GeoIP base constants: GeoIP2-ISP, GeoIP2-ASN, ...
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.geoIP2.properties.maxmindEditionIDs.items",
		},
		"modules/380-metallb/openapi/config-values.yaml": {
			// ignore enum values
			"properties.addressPools.items.properties.protocol",
		},
		"modules/400-descheduler/openapi/values.yaml": {
			// ignore enum values
			"properties.internal.properties.deschedulers.items.properties.strategies.properties.removePodsViolatingNodeAffinity.properties.nodeAffinityType.items",
		},
		"ee/modules/380-metallb/openapi/config-values.yaml": {
			// ignore enum values
			"properties.addressPools.items.properties.protocol",
		},
		"candi/cloud-providers/azure/openapi/cluster_configuration.yaml": {
			// ignore enum values
			"apiVersions[*].openAPISpec.properties.serviceEndpoints.items",
		},
		"candi/cloud-providers/vcd/openapi/cluster_configuration.yaml": {
			// ignore enum values "NSX-T" and "NSX-V"
			"apiVersions[*].openAPISpec.properties.edgeGateway.properties.type",
			"apiVersions[*].openAPISpec.allOf[*].oneOf[*].properties.edgeGateway.oneOf[*].properties.type",
			// ignore enum values "org" and "ext"
			"apiVersions[*].openAPISpec.properties.edgeGateway.properties.NSX-V.properties.externalNetworkType",
		},
		"ee/candi/cloud-providers/vcd/openapi/cluster_configuration.yaml": {
			// ignore enum values "NSX-T" and "NSX-V"
			"apiVersions[*].openAPISpec.properties.edgeGateway.properties.type",
			"apiVersions[*].openAPISpec.allOf[*].oneOf[*].properties.edgeGateway.oneOf[*].properties.type",
			// ignore enum values "org" and "ext"
			"apiVersions[*].openAPISpec.properties.edgeGateway.properties.NSX-V.properties.externalNetworkType",
		},
	}

	arrayPathRegex = regexp.MustCompile(`\[\d+\]`)
)

type EnumValidator struct {
	key      string
	excludes map[string]struct{}
}

func NewEnumValidator() EnumValidator {
	keyExcludes := make(map[string]struct{})

	for _, exc := range fileExcludes["*"] {
		keyExcludes[exc+".enum"] = struct{}{}
	}

	return EnumValidator{
		key:      "enum",
		excludes: keyExcludes,
	}
}

func (en EnumValidator) Run(fileName, absoluteKey string, value interface{}) error {
	for _, exc := range fileExcludes[fileName] {
		en.excludes[exc+".enum"] = struct{}{}
	}
	if _, ok := en.excludes[absoluteKey]; ok {
		// excluding key, dont check it
		return nil
	}

	// check for slice path with wildcard (imporoved: replace all indexes with [*])
	wildcardKey := arrayPathRegex.ReplaceAllString(absoluteKey, "[*]")
	if _, ok := en.excludes[wildcardKey]; ok {
		// excluding key with wildcard
		return nil
	}

	values := value.([]interface{})
	enum := make([]string, 0, len(values))
	for _, val := range values {
		valStr, ok := val.(string)
		if !ok {
			continue // skip boolean flags
		}
		enum = append(enum, valStr)
	}

	err := en.validateEnumValues(absoluteKey, enum)

	return err
}

func (en EnumValidator) validateEnumValues(enumKey string, values []string) *multierror.Error {
	var res *multierror.Error
	for _, value := range values {
		err := en.validateEnumValue(value)
		if err != nil {
			res = multierror.Append(res, errors.Wrap(err, fmt.Sprintf("Enum '%s' is invalid", enumKey)))
		}
	}

	return res
}

func (en EnumValidator) validateEnumValue(value string) error {
	if len(value) == 0 {
		return nil
	}

	vv := []rune(value)
	if (vv[0] < 'A' || vv[0] > 'Z') && (vv[0] < '0' || vv[0] > '9') {
		return fmt.Errorf("value '%s' must start with Capital letter", value)
	}

	for i, char := range vv {
		if unicode.IsLetter(char) {
			continue
		}
		if unicode.IsNumber(char) {
			continue
		}

		if char == '.' && i != 0 && unicode.IsNumber(vv[i-1]) {
			// permit dot into float numbers
			continue
		}

		// if rune is symbol/space/etc - it's invalid

		return fmt.Errorf("value: '%s' must be in CamelCase", value)
	}

	return nil
}
