# Copyright 2021 Flant CJSC
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
  kube_a_v4_cidr_block = cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 0)
  kube_b_v4_cidr_block = cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 1)
  kube_c_v4_cidr_block = cidrsubnet(var.node_network_cidr, ceil(log(3, 2)), 2)

  nat_instance_internal_address_calculated = var.should_create_nat_instance ? (var.nat_instance_internal_address == null ? cidrhost(local.kube_c_v4_cidr_block, 10) : var.nat_instance_internal_address) : null
}

resource "yandex_vpc_route_table" "kube" {
  name           = var.prefix
  network_id     = var.network_id

  lifecycle {
    ignore_changes = [
      static_route,
    ]
  }

  dynamic static_route {
    for_each = var.should_create_nat_instance ? [local.nat_instance_internal_address_calculated] : []
    content {
      destination_prefix = "0.0.0.0/0"
      next_hop_address = static_route.value
    }
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_a" {
  name           = "${var.prefix}-a"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_a_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-a"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
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
  name           = "${var.prefix}-b"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_b_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-b"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
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

resource "yandex_vpc_subnet" "kube_c" {
  name           = "${var.prefix}-c"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_c_v4_cidr_block]
  route_table_id = yandex_vpc_route_table.kube.id
  zone           = "ru-central1-c"

  dynamic "dhcp_options" {
    for_each = (var.dhcp_domain_name != null) || (var.dhcp_domain_name_servers != null) ? [1] : []
    content {
      domain_name = var.dhcp_domain_name
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
