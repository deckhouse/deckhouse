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

  # Fixed four-way split of nodeNetworkCIDR. The slot indices are pinned, not
  # alphabetical: a=0, b=1, e=2, d=3 is the historical layout existing clusters
  # already run on, and renumbering it would renumber live subnets.
  static_zone_slots = {
    "ru-central1-a" = 0
    "ru-central1-b" = 1
    "ru-central1-e" = 2
    "ru-central1-d" = 3
  }
  static_zones = keys(local.static_zone_slots)

  # Zones outside the static set (ru-central1-f and any future ones) are created
  # only when explicitly selected in `zones`, and reuse the CIDR slots of the
  # static zones that are not selected. Sorted so the assignment never depends
  # on the input order. Adoption mode never creates subnets, so it stays out of
  # the slot math entirely.
  dynamic_zones = local.should_create_subnets ? sort(setsubtract(var.zones, local.static_zones)) : []
  has_dynamic   = length(local.dynamic_zones) > 0

  # Legacy mode (no dynamic zone selected) keeps the historical behavior
  # exactly: all four static subnets exist no matter which subset `zones`
  # lists. With at least one dynamic zone selected, only the selected static
  # subnets are created.
  static_zone_enabled = {
    for zone in local.static_zones : zone => (
      local.should_create_subnets && (!local.has_dynamic || contains(var.zones, zone))
    )
  }

  # Slots freed by the omitted static zones, iterated in the pinned slot order
  # so dynamic zones get deterministic addresses. The four-zone cap on `zones`
  # guarantees there are always enough vacated slots.
  vacated_slots = [
    for zone in ["ru-central1-a", "ru-central1-b", "ru-central1-e", "ru-central1-d"] :
    local.static_zone_slots[zone] if local.has_dynamic && !contains(var.zones, zone)
  ]

  dynamic_zone_slots = { for index, zone in local.dynamic_zones : zone => local.vacated_slots[index] }

  kube_a_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, 2, 0) : null
  kube_b_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, 2, 1) : null
  kube_e_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, 2, 2) : null
  kube_d_v4_cidr_block = local.should_create_subnets ? cidrsubnet(var.node_network_cidr, 2, 3) : null

  not_have_existing_subnet_a = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-a", null) == null)
  not_have_existing_subnet_b = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-b", null) == null)
  not_have_existing_subnet_e = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-e", null) == null)
  not_have_existing_subnet_d = local.should_create_subnets || (lookup(var.existing_zone_to_subnet_id_map, "ru-central1-d", null) == null)

  is_with_nat_instance = var.layout == "WithNATInstance"
  is_standard          = var.layout == "Standard"

  #For layout WithNATInstance we use next_hop_address and destination_prefix, for layout Standard we use created gateway
  next_hop_address = local.is_with_nat_instance ? [local.nat_instance_internal_address_calculated] : [null]

  should_create_route_table = var.existing_route_table_id == ""

  # Subnets created here attach to whichever route table is in play, and the module's output has to
  # name the same one. Mirrors how existing_network_id and existing_zone_to_subnet_id_map already
  # switch between "use what the operator gave us" and "create it".
  route_table_id = local.should_create_route_table ? join("", yandex_vpc_route_table.kube.*.id) : var.existing_route_table_id
}

data "yandex_vpc_subnet" "kube_a" {
  count     = local.not_have_existing_subnet_a ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-a
}

data "yandex_vpc_subnet" "kube_b" {
  count     = local.not_have_existing_subnet_b ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-b
}

data "yandex_vpc_subnet" "kube_e" {
  count     = local.not_have_existing_subnet_e ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-e
}

data "yandex_vpc_subnet" "kube_d" {
  count     = local.not_have_existing_subnet_d ? 0 : 1
  subnet_id = var.existing_zone_to_subnet_id_map.ru-central1-d
}

# Adopted subnets in zones outside the static set are read generically, so a
# dynamic zone never needs its own data source.
data "yandex_vpc_subnet" "dynamic" {
  for_each  = local.should_create_subnets ? {} : { for zone, id in var.existing_zone_to_subnet_id_map : zone => id if !contains(local.static_zones, zone) }
  subnet_id = each.value
}

resource "yandex_vpc_gateway" "kube" {
  count  = local.is_standard ? 1 : 0
  name   = var.prefix
  labels = var.labels
  shared_egress_gateway {}
}

resource "yandex_vpc_route_table" "kube" {
  count      = local.should_create_route_table ? 1 : 0
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
  count          = local.static_zone_enabled["ru-central1-a"] ? 1 : 0
  name           = "${var.prefix}-a"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_a_v4_cidr_block]
  route_table_id = local.route_table_id
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
  count          = local.static_zone_enabled["ru-central1-b"] ? 1 : 0
  name           = "${var.prefix}-b"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_b_v4_cidr_block]
  route_table_id = local.route_table_id
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

resource "yandex_vpc_subnet" "kube_e" {
  count          = local.static_zone_enabled["ru-central1-e"] ? 1 : 0
  name           = "${var.prefix}-e"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_e_v4_cidr_block]
  route_table_id = local.route_table_id
  zone           = "ru-central1-e"

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

# Subnets in zones outside the static set are created generically: each
# selected dynamic zone takes the CIDR slot of an omitted static zone, so
# enabling ru-central1-f never renumbers the subnets an existing cluster
# already has.
resource "yandex_vpc_subnet" "dynamic" {
  for_each       = local.should_create_subnets ? local.dynamic_zone_slots : {}
  name           = "${var.prefix}-${trimprefix(each.key, "ru-central1-")}"
  network_id     = var.network_id
  v4_cidr_blocks = [cidrsubnet(var.node_network_cidr, 2, each.value)]
  route_table_id = local.route_table_id
  zone           = each.key

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

    # A dynamic zone's CIDR slot is derived from the current zone list. If the
    # selected static set changes while this zone stays selected (for example,
    # [a,b,d,f] becomes [a,b,e,f]), the computed slot moves, but the real
    # subnet keeps its original CIDR because of ignore_changes. That is only a
    # problem when a *static* subnet that is now enabled claims that same
    # CIDR — the cloud then answers "Subnet CIDRs can not overlap". Refuse
    # early with a clear message in that case, and let benign slot drift
    # through (the subnet simply keeps its slot).
    postcondition {
      condition     = !contains([for zone, enabled in local.static_zone_enabled : cidrsubnet(var.node_network_cidr, 2, local.static_zone_slots[zone]) if enabled], self.v4_cidr_blocks[0])
      error_message = "ERROR: subnet '${self.name}' occupies ${self.v4_cidr_blocks[0]}, which the current zone list assigns to a static zone. Migrate in two steps: remove the dynamic zone from `zones` and converge (its subnet is deleted), then add it back and converge again."
    }
  }

  labels = var.labels
}

resource "yandex_vpc_subnet" "kube_d" {
  count          = local.static_zone_enabled["ru-central1-d"] ? 1 : 0
  name           = "${var.prefix}-d"
  network_id     = var.network_id
  v4_cidr_blocks = [local.kube_d_v4_cidr_block]
  route_table_id = local.route_table_id
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
