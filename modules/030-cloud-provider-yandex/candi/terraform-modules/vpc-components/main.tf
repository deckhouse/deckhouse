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

locals {
  should_create_subnets = length(var.existing_zone_to_subnet_id_map) == 0 ? true : false

  kube_a_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 0) : null
  kube_b_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 1) : null
  kube_d_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 3) : null

  not_have_existing_subnet_a = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-a", null) == null)
  not_have_existing_subnet_b = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-b", null) == null)
  not_have_existing_subnet_d = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-d", null) == null)

  is_with_nat_instance = var.layout == "WithNATInstance"
  is_standard          = var.layout == "Standard"

  #For layout WithNATInstance we use next_hop_address and destination_prefix, for layout Standard we use created gateway
  next_hop_address = local.is_with_nat_instance ? [local.nat_instance_internal_address_calculated] : [null]
}

data "yandex_vpc_subnet" "kube_a" {
  count     = local.not_have_existing_subnet_a ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-a
}

data "yandex_vpc_subnet" "kube_b" {
  count     = local.not_have_existing_subnet_b ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-b
}

data "yandex_vpc_subnet" "kube_d" {
  count     = local.not_have_existing_subnet_d ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-d
}

resource "yandex_vpc_gateway" "kube" {
  count  = local.is_standard ? 1 : 0
  name   = var.prefix
  labels = var.labels
  shared_egress_gateway {}
}

resource "yandex_vpc_route_table" "kube" {
  name       = var.prefix
  network_id = var.network_id

  lifecycle {
    ignore_changes = [
      static_route,
    ]
  }

  dynamic "static_route" {
    for_each = local.is_standard || local.is_with_nat_instance ? local.next_hop_address : []
    content {
      destination_prefix = "0.0.0.0/0"
      next_hop_address   = static_route.value
      gateway_id         = local.is_standard ? yandex_vpc_gateway.kube[0].id : null
    }
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_a" {
  count          = local.should_create_subnets ? 1 : 0
  name           = "${var.prefix}-a"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_a_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-a"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name         = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_b" {
  count          = local.should_create_subnets ? 1 : 0
  name           = "${var.prefix}-b"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_b_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-b"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name         = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_d" {
  count          = local.should_create_subnets ? 1 : 0
  name           = "${var.prefix}-d"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_d_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-d"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name         = var.dhcp_domain_name
      domain_name_servers = var.dhcp_domain_name_servers
    }
  }

  lifecycle {
    ignore_changes = [
      v4_cidr_blocks,
    ]
  }

  labels = var.labels
}
