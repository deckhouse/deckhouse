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
		"/deckhouse/ee/candi/cloud-providers/openstack":                             {Strategy: linker.ThrowError, NewName: "/deckhouse/candi/cloud-providers/openstack"},
		"/deckhouse/ee/candi/cloud-providers/vsphere":                               {Strategy: linker.ThrowError, NewName: "/deckhouse/candi/cloud-providers/vsphere"},
		"/deckhouse/ee/modules/030-cloud-provider-openstack/cloud-instance-manager": {Strategy: linker.ThrowError, NewName: "/deckhouse/modules/040-node-manager/cloud-providers/openstack"},
		"/deckhouse/ee/modules/030-cloud-provider-vsphere/cloud-instance-manager":   {Strategy: linker.ThrowError, NewName: "/deckhouse/modules/040-node-manager/cloud-providers/vsphere"},
	},
	TempDir: ".d8-module-bak",
}
