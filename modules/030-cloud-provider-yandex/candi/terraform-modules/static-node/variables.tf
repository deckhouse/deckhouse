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

variable "nodeGroupName" {
  type = string
}

variable "nodeIndex" {
  type = number
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

# Maps a YandexInstanceClass `networkType` onto the provider's
# `network_acceleration_type`. Both API version spellings are listed on purpose:
# v1 (the storage version, and what YandexClusterConfiguration used) spells them
# Standard/SoftwareAccelerated, v1alpha1 spells them STANDARD/SOFTWARE_ACCELERATED.
# The conversion webhook normalises objects that go through the apiserver, but
# dhctl parses the bootstrap resources YAML verbatim
# (dhctl/pkg/config/cloud_provider_resources.go), so a v1alpha1 instance class
# reaches this module unconverted and must still map to the right value instead of
# silently degrading to the provider default.
variable "network_types" {
  type = map(any)
  default = {
    "Standard"             = "standard"
    "SoftwareAccelerated"  = "software_accelerated"
    "STANDARD"             = "standard"
    "SOFTWARE_ACCELERATED" = "software_accelerated"
  }
}

variable "resourceManagementTimeout" {
  type    = string
  default = "10m"
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
  source                       = "../migration"
  providerClusterConfiguration = var.providerClusterConfiguration
  nodeGroups                   = var.nodeGroups
  instanceClasses              = var.instanceClasses
  secrets                      = var.secrets
  settings                     = var.settings
}

locals {
  prefix          = var.clusterConfiguration.cloud.prefix
  node_group_name = var.nodeGroupName

  # The migration module resolves which source of truth wins; everything below
  # reads the resolved ModuleConfig. tolist/tomap also absorb explicit nulls,
  # which try() alone does not.
  _node_params = try(module.migration.settings.spec.settings.nodes.parameters, {})

  _node_group          = lookup(module.migration.nodeGroups, local.node_group_name, null)
  _instance_class_name = try(local._node_group.spec.cloudInstances.classReference.name, "")
  instance_class       = try(module.migration.instanceClasses[local._instance_class_name].spec, {})

  # Zones are already resolved in the NodeGroup (node group zones, then the
  # cluster-wide zones); null means "every zone the subnets cover".
  cluster_zones                  = try(tolist(local._node_params.zones), [])
  node_group_zones               = try(local._node_group.spec.cloudInstances.zones, null)
  existing_zone_to_subnet_id_map = try(tomap(local._node_params.existingZoneToSubnetIDMap), {})

  platform      = try(local.instance_class.platformID, "standard-v2")
  cores         = try(local.instance_class.cores, 0)
  core_fraction = try(local.instance_class.coreFraction, null)
  memory        = try(local.instance_class.memory, 0) / 1024
  disk_size_gb  = try(local.instance_class.diskSizeGB, 50)
  disk_type     = try(local.instance_class.diskType, "network-ssd")
  image_id      = try(local.instance_class.imageID, "")

  node_network_cidr = try(local._node_params.nodeNetworkCIDR, "")
  ssh_public_key    = try(local._node_params.sshPublicKey, "")

  # Per-node-group external addressing lives in nodes.parameters because
  # YandexInstanceClass has no externalIPAddresses counterpart. The
  # assignPublicIPAddress fallback covers ModuleConfig-only clusters that only
  # ask for an automatically allocated address.
  _external_ip_addresses = try(local._node_params.externalIPAddresses[local.node_group_name], [])
  external_ip_addresses = length(local._external_ip_addresses) > 0 ? local._external_ip_addresses : (
    try(local.instance_class.assignPublicIPAddress, false) ? ["Auto"] : []
  )

  _external_subnet_ids = try(local._node_params.externalSubnetIDs[local.node_group_name], [])
  external_subnet_ids  = length(local._external_subnet_ids) > 0 ? local._external_subnet_ids : try(local.instance_class.additionalSubnets, [])

  # Keep null for "unset": main.tf branches on it to decide whether to attach an
  # extra network interface.
  external_subnet_id_deprecated = try(local.instance_class.mainSubnet, "") != "" ? local.instance_class.mainSubnet : null

  # An unset networkType leaves network_acceleration_type at the provider default,
  # which is what YandexClusterConfiguration did before the migration. A *set* but
  # unrecognised value is an error rather than a silent fallback: the pre-migration
  # code indexed var.network_types directly and failed on an unknown key, and
  # silently ignoring it would hand the user a standard network while their
  # instance class asks for a software-accelerated one. See
  # the precondition on yandex_compute_instance.static.
  _network_type_raw = try(local.instance_class.networkType, "")
  network_type      = lookup(var.network_types, local._network_type_raw, null)

  additional_labels = merge(
    try(tomap(local._node_params.labels), {}),
    try(local.instance_class.additionalLabels, {}),
  )
}
