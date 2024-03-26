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
	"strings"
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
		},
		"global-hooks/openapi/values.yaml": {
			// from openapispec
			"properties.clusterConfiguration.properties.apiVersion",
			"properties.clusterConfiguration.properties.cloud.properties.provider",
			// http and https
			"properties.modulesImages.properties.registry.properties.scheme",
		},
		"modules/010-user-authn-crd/crds/dex-provider.yaml": {
			// v1alpha1 migrated to v1
			"spec.versions[0].schema.openAPIV3Schema.properties.spec.properties.github.properties.teamNameField",
		},
		"modules/010-prometheus-crd/crds/grafanaadditionaldatasources.yaml": {
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
		"modules/030-cloud-provider-vsphere/openapi/config-values.yaml": {
			// ignore temporary flag that is already used (will be deleted after all CSIs are migrated)
			"properties.storageClass.properties.compatibilityFlag",
		},
		"ee/modules/030-cloud-provider-vsphere/openapi/config-values.yaml": {
			// ignore temporary flag that is already used (will be deleted after all CSIs are migrated)
			"properties.storageClass.properties.compatibilityFlag",
		},
		"modules/030-cloud-provider-vsphere/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
		},
		"ee/modules/030-cloud-provider-vsphere/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
		},
		"modules/030-cloud-provider-vcd/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
		},
		"modules/030-cloud-provider-zvirt/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
		},
		"ee/modules/030-cloud-provider-vcd/openapi/values.yaml": {
			// ignore internal values
			"properties.internal.properties.providerDiscoveryData.properties.apiVersion",
			"properties.internal.properties.providerClusterConfiguration.properties.apiVersion",
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
		"modules/040-node-manager/openapi/config-values.yaml": {
			// ignore internal values
			"properties.allowedBundles.items",
		},
		"ee/modules/040-node-manager/openapi/config-values.yaml": {
			// ignore internal values
			"properties.allowedBundles.items",
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
		"modules/031-ceph-csi/crds/cephcsi.yaml": {
			// ignore file system names: ext4, xfs, etc.
			"properties.internal.properties.crs.items.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
		},
		"modules/031-ceph-csi/openapi/values.yaml": {
			// ignore file system names: ext4, xfs, etc.
			"properties.internal.properties.crs.items.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
			"spec.versions[*].schema.openAPIV3Schema.properties.spec.properties.rbd.properties.storageClasses.items.properties.defaultFSType",
		},
		"modules/380-metallb/openapi/config-values.yaml": {
			// ignore enum values
			"properties.addressPools.items.properties.protocol",
		},
		"ee/modules/380-metallb/openapi/config-values.yaml": {
			// ignore enum values
			"properties.addressPools.items.properties.protocol",
		},
		"candi/cloud-providers/azure/openapi/cluster_configuration.yaml": {
			// ignore enum values
			"apiVersions[*].openAPISpec.properties.serviceEndpoints.items",
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

	// check for slice path with wildcard
	index := arrayPathRegex.FindString(absoluteKey)
	if index != "" {
		wildcardKey := strings.ReplaceAll(absoluteKey, index, "[*]")
		if _, ok := en.excludes[wildcardKey]; ok {
			// excluding key with wildcard
			return nil
		}
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
