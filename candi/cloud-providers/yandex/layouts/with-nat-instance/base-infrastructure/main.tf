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

resource "yandex_vpc_network" "kube" {
  count = local.existing_network_id != "" ? 0 : 1
  name = local.prefix

  labels = local.labels
}

locals {
  network_id = local.existing_network_id != "" ? local.existing_network_id : join("", yandex_vpc_network.kube.*.id) # https://github.com/hashicorp/terraform/issues/23222#issuecomment-547462883
}


module "vpc_components" {
  source = "../../../terraform-modules/vpc-components"
  layout = local.layout
  prefix = local.prefix
  network_id = local.network_id
  node_network_cidr = local.node_network_cidr
  existing_zone_to_subnet_id_map = local.existing_zone_to_subnet_id_map

  dhcp_domain_name = local.dhcp_domain_name
  dhcp_domain_name_servers = local.dhcp_domain_name_servers

  nat_instance_external_address = local.nat_instance_external_address
  nat_instance_internal_address = local.nat_instance_internal_address
  nat_instance_internal_subnet_id = local.nat_instance_internal_subnet_id
  nat_instance_external_subnet_id = local.nat_instance_external_subnet_id
  nat_instance_cores = local.nat_instance_cores
  nat_instance_memory = local.nat_instance_memory
  nat_instance_ssh_key = var.providerClusterConfiguration.sshPublicKey

  labels = local.labels
}

module "monitoring-service-account" {
  source = "../../../terraform-modules/monitoring-service-account"

  prefix   = local.prefix
  apiKey   = local.exporter_api_key
  folderID = var.providerClusterConfiguration.provider.folderID
}
