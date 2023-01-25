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

package conf

import "edition_linker/linker"

var MergeConf = linker.MergeConf{
	Targets: linker.MergeTargets{
		// ee/candy/cloud-providers:
		"/deckhouse/ee/candi/cloud-providers/openstack": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/candi/cloud-providers/openstack",
		},
		"/deckhouse/ee/candi/cloud-providers/vsphere": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/candi/cloud-providers/vsphere",
		},

		// ee/fe/modules:
		"/deckhouse/ee/fe/modules/340-monitoring-applications": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/340-monitoring-applications",
		},
		"/deckhouse/ee/fe/modules/500-basic-auth": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/500-basic-auth",
		},

		// ee/modules:
		// ee/modules/007-registrypackages doesn't contain tests
		// ee/modules/030-cloud-provider-*:
		"/deckhouse/ee/modules/030-cloud-provider-openstack": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/030-cloud-provider-openstack",
		},
		"/deckhouse/ee/modules/030-cloud-provider-vsphere": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/030-cloud-provider-vsphere",
		},
		// ee/modules/040-node-manager:
		"/deckhouse/modules/030-cloud-provider-openstack/cloud-instance-manager": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/040-node-manager/cloud-providers/openstack",
		},
		"/deckhouse/modules/030-cloud-provider-vsphere/cloud-instance-manager": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/040-node-manager/cloud-providers/vsphere",
		},
		"/deckhouse/ee/modules/040-node-manager/openapi/config-values.yaml": {
			Strategy: linker.StashInTemp,
			NewName:  "/deckhouse/modules/040-node-manager/openapi/config-values.yaml",
		},
		"/deckhouse/ee/modules/040-node-manager/openapi/doc-ru-config-values.yaml": {
			Strategy: linker.StashInTemp,
			NewName:  "/deckhouse/modules/040-node-manager/openapi/doc-ru-config-values.yaml",
		},
		"/deckhouse/ee/modules/040-node-manager/openapi/openapi-case-tests.yaml": {
			Strategy: linker.StashInTemp,
			NewName:  "/deckhouse/modules/040-node-manager/openapi/openapi-case-tests.yaml",
		},
		// ee/modules/040-terraform-manager doesn't contain tests
		// ee/modules/110-istio
		"/deckhouse/ee/modules/110-istio": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/110-istio",
		},
		// ee/modules/140-user-authz
		"/deckhouse/ee/modules/140-user-authz/images": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/140-user-authz/images",
		},
		"/deckhouse/ee/modules/140-user-authz/templates/webhook": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/140-user-authz/templates/webhook",
		},
		// ee/modules/350-node-local-dns
		"/deckhouse/ee/modules/350-node-local-dns": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/350-node-local-dns",
		},
		// ee/modules/380-metallb
		"/deckhouse/ee/modules/380-metallb": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/380-metallb",
		},
		// ee/modules/450-keepalived
		"/deckhouse/ee/modules/450-keepalived": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/450-keepalived",
		},
		// ee/modules/450-network-gateway
		"/deckhouse/ee/modules/450-network-gateway": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/450-network-gateway",
		},
		// ee/modules/450-network-gateway
		"/deckhouse/ee/modules/502-delivery": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/502-delivery",
		},
		// ee/modules/450-network-gateway
		"/deckhouse/ee/modules/600-flant-integration": {
			Strategy: linker.ThrowError,
			NewName:  "/deckhouse/modules/600-flant-integration",
		},
	},
	TempDir: ".d8-module-bak",
}
