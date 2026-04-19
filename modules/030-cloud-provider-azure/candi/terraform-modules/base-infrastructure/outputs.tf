# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

output "cloud_discovery_data" {
  value = {
    "apiVersion"        = "deckhouse.io/v1"
    "kind"              = "AzureCloudDiscoveryData"
    "resourceGroupName" = azurerm_resource_group.kube.name
    "vnetName"          = azurerm_virtual_network.kube.name
    "subnetName"        = azurerm_subnet.kube.name
    "zones"             = local.zones
    "instances" = {
      "urn"            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn
      "diskType"       = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "StandardSSD_LRS")
      "additionalTags" = lookup(var.providerClusterConfiguration, "tags", {})
    }
  }
}
