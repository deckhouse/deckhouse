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

# Absent in ModuleConfig-only clusters; the migration module resolves the source
# of truth and validates the result.
variable "providerClusterConfiguration" {
  type    = any
  default = null
}

variable "nodeIndex" {
  type    = number
  default = 0
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

variable "nodeGroups" {
  type    = any
  default = {}
}

variable "instanceClasses" {
  type    = any
  default = {}
}

variable "secrets" {
  type    = any
  default = {}
}

variable "settings" {
  type    = any
  default = null
}

module "migration" {
  source                       = "../../../terraform-modules/migration"
  providerClusterConfiguration = var.providerClusterConfiguration
  nodeGroups                   = var.nodeGroups
  instanceClasses              = var.instanceClasses
  secrets                      = var.secrets
  settings                     = var.settings
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix

  # The migration module resolves which source of truth wins; everything below
  # reads the resolved ModuleConfig. tolist/tomap also absorb explicit nulls,
  # which try() alone does not.
  _node_params = try(module.migration.settings.spec.settings.nodes.parameters, {})

  layout = try(local._node_params.layout, "")
  zones  = try(tolist(local._node_params.zones), [])

  # Raw explicit zones, before the migration module resolves its default: an
  # adopted subnet map without an explicit selection keeps the discovery zone
  # list on the static set (the legacy behavior node-manager relies on for
  # NodeGroup default zones), while node placement still uses every zone the
  # map covers. The migration module decides which configuration source wins,
  # so the explicit list comes from it: a stale PCC Secret left behind after
  # migration must not override the ModuleConfig selection.
  raw_zones = module.migration.explicit_zones

  # The pinned static subnet set; mirrors static_zone_slots in vpc-components.
  static_zones = ["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-e"]

  # Static discovery zones of an adopted map. Empty when the map has no static
  # zones at all — then discovery falls back to every adopted zone.
  adopted_static_discovery_zones = tolist(setintersection(keys(module.vpc_components.zone_to_subnet_id_map), local.static_zones))
  node_network_cidr              = try(local._node_params.nodeNetworkCIDR, "")
  existing_network_id            = try(local._node_params.existingNetworkID, "")
  existing_zone_to_subnet_id_map = try(tomap(local._node_params.existingZoneToSubnetIDMap), {})
  existing_route_table_id        = try(local._node_params.existingRouteTableID, "")
  labels                         = try(tomap(local._node_params.labels), {})
  ssh_public_key                 = try(local._node_params.sshPublicKey, "")

  folder_id = try(module.migration.settings.spec.settings.provider.parameters.folderID, "")

  # vpc-components branches on null, so an unset option must not arrive as "".
  _dhcp                    = try(local._node_params.dhcpOptions, {})
  dhcp_domain_name         = try(local._dhcp.domainName, "") != "" ? local._dhcp.domainName : null
  dhcp_domain_name_servers = length(try(tolist(local._dhcp.domainNameServers), [])) > 0 ? local._dhcp.domainNameServers : null

  _nat                              = try(local._node_params.withNATInstance, {})
  nat_instance_internal_subnet_id   = try(local._nat.internalSubnetID, "") != "" ? local._nat.internalSubnetID : null
  nat_instance_internal_subnet_cidr = try(local._nat.internalSubnetCIDR, "") != "" ? local._nat.internalSubnetCIDR : null
  nat_instance_external_subnet_id   = try(local._nat.externalSubnetID, "") != "" ? local._nat.externalSubnetID : null
  nat_instance_external_address     = try(local._nat.natInstanceExternalAddress, "") != "" ? local._nat.natInstanceExternalAddress : null
  nat_instance_internal_address     = try(local._nat.natInstanceInternalAddress, "") != "" ? local._nat.natInstanceInternalAddress : null
  nat_instance_cores                = try(local._nat.natInstanceResources.cores, 0) > 0 ? local._nat.natInstanceResources.cores : 2
  nat_instance_memory               = floor((try(local._nat.natInstanceResources.memory, 0) > 0 ? local._nat.natInstanceResources.memory : 2048) / 1024)
  nat_instance_platform             = try(local._nat.natInstanceResources.platform, "") != "" ? local._nat.natInstanceResources.platform : "standard-v2"

  # The exporter API key is a credential, so it lives in the
  # d8-credentials-exporter Secret rather than in the ModuleConfig. The Secret
  # selection itself is shared with the provider configuration in
  # terraform-modules/providers.tf, which is symlinked into this root module, so
  # both read the same resolved credentials.
  #
  # nonsensitive() is required here and only here: this key is published in the
  # cloud_discovery_data output, exactly as it was before the migration, and Terraform
  # refuses to place a sensitive value in a non-sensitive output.
  exporter_api_key = nonsensitive(lookup(local.credentials, "d8-credentials-exporter", ""))
}
