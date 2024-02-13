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

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition = cidrsubnet(var.providerClusterConfiguration.nodeNetworkCIDR, 0, 0) == var.providerClusterConfiguration.nodeNetworkCIDR
    error_message = "Invalid nodeNetworkCIDR in YandexClusterConfiguration."
  }
}

variable "nodeIndex" {
  type = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  existing_network_id = lookup(var.providerClusterConfiguration, "existingNetworkID", "")
  node_network_cidr = var.providerClusterConfiguration.nodeNetworkCIDR
  existing_zone_to_subnet_id_map = lookup(var.providerClusterConfiguration, "existingZoneToSubnetIDMap", {})
  nat_instance_internal_subnet_id = lookup(var.providerClusterConfiguration.withNATInstance, "internalSubnetID", null)
  nat_instance_external_subnet_id = lookup(var.providerClusterConfiguration.withNATInstance, "externalSubnetID", null)
  nat_instance_external_address = lookup(var.providerClusterConfiguration.withNATInstance, "natInstanceExternalAddress", null)
  nat_instance_internal_address = lookup(var.providerClusterConfiguration.withNATInstance, "natInstanceInternalAddress", null)
  nat_instance_cores = lookup(var.providerClusterConfiguration.withNATInstance.natInstanceResources, "cores", 2)
  nat_instance_memory = floor(lookup(var.providerClusterConfiguration.withNATInstance.natInstanceResources, "memory", 2048) / 1024)
  exporter_api_key = lookup(var.providerClusterConfiguration.withNATInstance, "exporterAPIKey", "")

  dhcp_options = lookup(var.providerClusterConfiguration, "dhcpOptions", null)
  dhcp_domain_name = local.dhcp_options != null ? lookup(local.dhcp_options, "domainName", null) : null
  dhcp_domain_name_servers = local.dhcp_options != null ? lookup(local.dhcp_options, "domainNameServers", null) : null

  labels = lookup(var.providerClusterConfiguration, "labels", {})
  layout = var.providerClusterConfiguration.layout
}
