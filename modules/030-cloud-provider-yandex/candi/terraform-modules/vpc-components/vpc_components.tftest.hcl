# Copyright 2026 Flant JSC
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

# Run from this directory with the OpenTofu version pinned in
# candi/terraform_versions.yml:
#
#   tofu init -backend=false && tofu test
#
# The pinned provider version is no longer downloadable from the registry, so keep a copy
# in a plugin cache and point OpenTofu at it to run offline:
#
#   export TF_PLUGIN_CACHE_DIR="$HOME/.terraform.d/plugin-cache"
#
# The Yandex Cloud provider is mocked, so nothing is created and no credentials are
# needed; the provider still has to be installable for its schema.
#
# This module owns the network: it either carves subnets out of nodeNetworkCIDR or
# adopts the ones the operator already has, and it routes egress either through a cloud
# NAT gateway (Standard) or through a NAT instance (WithNATInstance). Every case below
# pins which resources exist, because getting this wrong recreates the whole VPC.
#
# Addressing scheme: nodeNetworkCIDR is split into four fixed slots —
# ru-central1-a=0, ru-central1-b=1, ru-central1-e=2, ru-central1-d=3. A zone outside
# the static set (ru-central1-f and any future one) is created only when explicitly
# listed in `zones` and reuses the slot of a static zone that is not listed, so
# enabling it never renumbers subnets an existing cluster already has.

mock_provider "yandex" {}

variables {
  prefix                         = "cluster-test"
  network_id                     = "enp-test"
  node_network_cidr              = "10.60.0.0/16"
  existing_zone_to_subnet_id_map = {}
  labels                         = {}
}

run "standard_creates_subnets_and_gateway" {
  command = plan

  variables {
    layout = "Standard"
  }

  # With no zones listed the module keeps the legacy behavior: all four static
  # subnets are created, one per fixed slot, and no dynamic subnet appears.
  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 1 && length(yandex_vpc_subnet.kube_b) == 1 && length(yandex_vpc_subnet.kube_e) == 1 && length(yandex_vpc_subnet.kube_d) == 1 && length(yandex_vpc_subnet.dynamic) == 0
    error_message = "expected the default subnet set to stay unchanged without explicit dynamic zones"
  }

  # The CIDR split is what nodes get their addresses from: /16 into four /18 blocks.
  assert {
    condition     = yandex_vpc_subnet.kube_a[0].v4_cidr_blocks == tolist(["10.60.0.0/18"]) && yandex_vpc_subnet.kube_a[0].zone == "ru-central1-a" && yandex_vpc_subnet.kube_a[0].name == "cluster-test-a"
    error_message = "expected ru-central1-a to take the first quarter of nodeNetworkCIDR"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_b[0].v4_cidr_blocks == tolist(["10.60.64.0/18"]) && yandex_vpc_subnet.kube_b[0].zone == "ru-central1-b" && yandex_vpc_subnet.kube_b[0].name == "cluster-test-b"
    error_message = "expected ru-central1-b to take the second quarter of nodeNetworkCIDR"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_e[0].v4_cidr_blocks == tolist(["10.60.128.0/18"]) && yandex_vpc_subnet.kube_e[0].zone == "ru-central1-e" && yandex_vpc_subnet.kube_e[0].name == "cluster-test-e"
    error_message = "expected ru-central1-e to take the third quarter of nodeNetworkCIDR"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_d[0].v4_cidr_blocks == tolist(["10.60.192.0/18"]) && yandex_vpc_subnet.kube_d[0].zone == "ru-central1-d" && yandex_vpc_subnet.kube_d[0].name == "cluster-test-d"
    error_message = "expected ru-central1-d to take the fourth quarter of nodeNetworkCIDR"
  }

  # The output map is the contract the layouts and node modules consume.
  assert {
    condition     = sort(keys(output.zone_to_subnet_id_map)) == tolist(["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-e"])
    error_message = "expected the output map to contain exactly the four static zones"
  }

  # Standard routes egress through a cloud NAT gateway and creates no NAT instance.
  assert {
    condition     = length(yandex_vpc_gateway.kube) == 1
    error_message = "expected a NAT gateway in the Standard layout"
  }

  # With no existing route table the module creates one and attaches every subnet to it.
  assert {
    condition     = length(yandex_vpc_route_table.kube) == 1
    error_message = "expected a route table to be created when no existing one is given"
  }

  assert {
    condition     = length(yandex_compute_instance.nat_instance) == 0
    error_message = "expected no NAT instance in the Standard layout"
  }

  assert {
    condition     = output.nat_instance_name == "" && output.nat_instance_zone == ""
    error_message = "expected empty NAT instance outputs in the Standard layout"
  }
}

# A static-only subset must not delete the subnets the cluster already has: with no
# dynamic zone listed, all four static subnets exist regardless of the subset.
run "static_zone_subset_keeps_all_four_subnets" {
  command = plan

  variables {
    layout = "Standard"
    zones  = ["ru-central1-a", "ru-central1-b"]
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 1 && length(yandex_vpc_subnet.kube_b) == 1 && length(yandex_vpc_subnet.kube_e) == 1 && length(yandex_vpc_subnet.kube_d) == 1 && length(yandex_vpc_subnet.dynamic) == 0
    error_message = "expected all four static subnets to stay for a static-only zone subset"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_e[0].v4_cidr_blocks == tolist(["10.60.128.0/18"]) && yandex_vpc_subnet.kube_d[0].v4_cidr_blocks == tolist(["10.60.192.0/18"])
    error_message = "expected unselected static subnets to keep their fixed CIDRs"
  }
}

# The explicit opt-in: with ru-central1-f listed and ru-central1-e dropped, f takes the
# slot e vacated and the other subnets keep their historical CIDRs. A /24 split into
# four slots gives /26 subnets.
run "dynamic_zone_takes_vacated_slot" {
  command = plan

  variables {
    layout            = "Standard"
    node_network_cidr = "192.168.12.0/24"
    zones             = ["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-f"]
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 1 && length(yandex_vpc_subnet.kube_b) == 1 && length(yandex_vpc_subnet.kube_d) == 1 && length(yandex_vpc_subnet.kube_e) == 0 && length(yandex_vpc_subnet.dynamic) == 1
    error_message = "expected the selected static subnets plus exactly one dynamic subnet, without ru-central1-e"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_a[0].v4_cidr_blocks == tolist(["192.168.12.0/26"])
    error_message = "expected ru-central1-a to keep the first slot"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_b[0].v4_cidr_blocks == tolist(["192.168.12.64/26"])
    error_message = "expected ru-central1-b to keep the second slot"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_d[0].v4_cidr_blocks == tolist(["192.168.12.192/26"])
    error_message = "expected ru-central1-d to keep the fourth slot"
  }

  # ru-central1-f reuses the third slot, the one ru-central1-e vacated.
  assert {
    condition     = yandex_vpc_subnet.dynamic["ru-central1-f"].v4_cidr_blocks == tolist(["192.168.12.128/26"]) && yandex_vpc_subnet.dynamic["ru-central1-f"].zone == "ru-central1-f" && yandex_vpc_subnet.dynamic["ru-central1-f"].name == "cluster-test-f"
    error_message = "expected ru-central1-f to take the vacated third slot"
  }

  assert {
    condition     = sort(keys(output.zone_to_subnet_id_map)) == tolist(["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-f"])
    error_message = "expected the output map to contain exactly the four selected zones"
  }
}

# Two runs with the same zones in a different order, including a hypothetical future
# zone: the sorted dynamic zones must land in the same vacated slots either way.
run "dynamic_slots_ignore_input_order_reversed" {
  command = plan

  variables {
    layout = "Standard"
    zones  = ["ru-central1-d", "ru-central1-f", "ru-central1-g"]
  }

  assert {
    condition     = yandex_vpc_subnet.dynamic["ru-central1-f"].v4_cidr_blocks == tolist(["10.60.0.0/18"]) && yandex_vpc_subnet.dynamic["ru-central1-g"].v4_cidr_blocks == tolist(["10.60.64.0/18"]) && yandex_vpc_subnet.kube_d[0].v4_cidr_blocks == tolist(["10.60.192.0/18"])
    error_message = "expected sorted dynamic zones in the first two vacated slots and ru-central1-d in its own slot"
  }
}

run "dynamic_slots_ignore_input_order_shuffled" {
  command = plan

  variables {
    layout = "Standard"
    zones  = ["ru-central1-g", "ru-central1-f", "ru-central1-d"]
  }

  assert {
    condition     = yandex_vpc_subnet.dynamic["ru-central1-f"].v4_cidr_blocks == tolist(["10.60.0.0/18"]) && yandex_vpc_subnet.dynamic["ru-central1-g"].v4_cidr_blocks == tolist(["10.60.64.0/18"]) && yandex_vpc_subnet.kube_d[0].v4_cidr_blocks == tolist(["10.60.192.0/18"])
    error_message = "expected the same slot assignment regardless of the input order"
  }
}

run "without_nat_creates_neither_gateway_nor_instance" {
  command = plan

  variables {
    layout = "WithoutNAT"
  }

  assert {
    condition     = length(yandex_vpc_gateway.kube) == 0
    error_message = "expected no NAT gateway in the WithoutNAT layout"
  }

  assert {
    condition     = length(yandex_compute_instance.nat_instance) == 0
    error_message = "expected no NAT instance in the WithoutNAT layout"
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 1
    error_message = "expected the subnets to be created regardless of the egress layout"
  }
}

run "with_nat_instance_creates_instance" {
  command = plan

  variables {
    layout                            = "WithNATInstance"
    nat_instance_internal_subnet_cidr = "10.60.192.0/24"
    # The layout passes the NAT instance sizing through from the configuration; the
    # resource requires it, so the suite has to supply it as well.
    nat_instance_cores  = 2
    nat_instance_memory = 2048
  }

  assert {
    condition     = length(yandex_compute_instance.nat_instance) == 1
    error_message = "expected a NAT instance in the WithNATInstance layout"
  }

  # WithNATInstance routes through the instance, not through a cloud gateway.
  assert {
    condition     = length(yandex_vpc_gateway.kube) == 0
    error_message = "expected no NAT gateway when egress goes through the NAT instance"
  }

  assert {
    condition     = length(yandex_vpc_subnet.nat_instance) == 1
    error_message = "expected a dedicated subnet for the NAT instance when its CIDR is given"
  }
}

# With a dynamic zone selected and ru-central1-a omitted, the NAT instance must not fall
# back to the hardcoded ru-central1-a: it goes to the alphabetically first selected zone
# and takes its internal address from that zone's subnet.
run "nat_instance_uses_first_selected_zone_in_dynamic_mode" {
  command = plan

  variables {
    layout              = "WithNATInstance"
    zones               = ["ru-central1-b", "ru-central1-d", "ru-central1-e", "ru-central1-f"]
    nat_instance_cores  = 2
    nat_instance_memory = 2048
  }

  assert {
    condition     = length(yandex_compute_instance.nat_instance) == 1 && yandex_compute_instance.nat_instance[0].zone == "ru-central1-b"
    error_message = "expected the NAT instance in the alphabetically first selected zone"
  }

  # ru-central1-b keeps its fixed second slot, so the NAT internal address comes from it.
  assert {
    condition     = yandex_compute_instance.nat_instance[0].network_interface[0].ip_address == "10.60.64.10"
    error_message = "expected the NAT instance internal address from the ru-central1-b subnet"
  }

  # No explicit NAT subnet CIDR: no dedicated subnet, the instance uses the created one.
  assert {
    condition     = length(yandex_vpc_subnet.nat_instance) == 0
    error_message = "expected no dedicated NAT subnet without an explicit NAT subnet CIDR"
  }
}

# Adopting existing subnets is the path that must never create anything: the operator
# already owns the network, and a created subnet here would mean a second, parallel one.
run "existing_subnets_are_adopted" {
  command = plan

  variables {
    layout = "Standard"
    existing_zone_to_subnet_id_map = {
      "ru-central1-a" = "e9b-subnet-a"
      "ru-central1-b" = "e9b-subnet-b"
      "ru-central1-e" = "e9b-subnet-e"
      "ru-central1-d" = "e9b-subnet-d"
      "ru-central1-f" = "e9b-subnet-f"
    }
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 0 && length(yandex_vpc_subnet.kube_b) == 0 && length(yandex_vpc_subnet.kube_e) == 0 && length(yandex_vpc_subnet.kube_d) == 0 && length(yandex_vpc_subnet.dynamic) == 0
    error_message = "expected no subnets to be created when existing ones are supplied"
  }

  assert {
    condition     = length(data.yandex_vpc_subnet.kube_a) == 1
    error_message = "expected the existing subnet to be read instead of created"
  }

  # The dynamic zone is read through the generic data source and published under its
  # own zone name in the output map. (The static adopted entries are keyed by the
  # zone reported by the cloud, which the mock does not pin, so only the dynamic
  # entry — keyed by the operator-supplied map — is asserted here.)
  assert {
    condition     = length(data.yandex_vpc_subnet.dynamic) == 1 && output.zone_to_subnet_id_map["ru-central1-f"] == data.yandex_vpc_subnet.dynamic["ru-central1-f"].id
    error_message = "expected the adopted dynamic zone to be read and published"
  }
}

# A partially supplied map is the mixed case: the zones the operator listed are adopted,
# the rest stay absent rather than being created behind their back.
run "partially_existing_subnets_are_not_mixed_with_created_ones" {
  command = plan

  variables {
    layout = "Standard"
    existing_zone_to_subnet_id_map = {
      "ru-central1-a" = "e9b-subnet-a"
    }
  }

  assert {
    condition     = length(data.yandex_vpc_subnet.kube_a) == 1
    error_message = "expected the supplied zone to be read from the cloud"
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 0
    error_message = "expected no subnet to be created for the supplied zone"
  }

  assert {
    condition     = length(data.yandex_vpc_subnet.kube_b) == 0 && length(yandex_vpc_subnet.kube_b) == 0 && length(data.yandex_vpc_subnet.dynamic) == 0 && length(yandex_vpc_subnet.dynamic) == 0
    error_message = "expected the zones absent from the map to be neither read nor created"
  }
}

# Adopting an existing route table is the same pattern as the subnets: the operator already
# owns it, and a created route table here would mean CCM writes pod routes into a table the
# infrastructure run never produced.
run "existing_route_table_is_adopted" {
  command = plan

  variables {
    layout                  = "Standard"
    existing_route_table_id = "enp-existing-route-table"
  }

  assert {
    condition     = length(yandex_vpc_route_table.kube) == 0
    error_message = "expected no route table to be created when an existing one is supplied"
  }

  assert {
    condition     = output.route_table_id == "enp-existing-route-table"
    error_message = "expected the existing route table id to be published"
  }

  # The created subnets must attach to the adopted table, not to the (absent) created one.
  assert {
    condition     = yandex_vpc_subnet.kube_a[0].route_table_id == "enp-existing-route-table"
    error_message = "expected the created subnet to attach to the existing route table"
  }
}

# Apply runs share state within this file: the first run creates ru-central1-f
# in the slot the then-omitted ru-central1-e vacated (slot 2), the second
# changes the static set so the computed slot moves to 3. The real subnet keeps
# its CIDR (ignore_changes), and the postcondition must stop the plan with a
# clear two-step-migration error instead of the cloud's overlap failure.
run "dynamic_zone_slot_is_assigned_on_apply" {
  command = apply

  variables {
    layout = "Standard"
    zones  = ["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-f"]
  }

  assert {
    condition     = yandex_vpc_subnet.dynamic["ru-central1-f"].v4_cidr_blocks == tolist(["10.60.128.0/18"])
    error_message = "expected ru-central1-f to take the slot ru-central1-e vacated"
  }
}

run "dynamic_zone_slot_drift_fails_with_clear_error" {
  command = plan

  variables {
    layout = "Standard"
    zones  = ["ru-central1-a", "ru-central1-b", "ru-central1-e", "ru-central1-f"]
  }

  expect_failures = [
    yandex_vpc_subnet.dynamic,
  ]
}

# Same state as the run above (ru-central1-f really occupies slot 2). A plan
# whose computed slot for f moves elsewhere must still pass when no enabled
# static subnet claims the real CIDR: benign drift is allowed, the subnet
# simply keeps its slot.
run "dynamic_zone_benign_slot_drift_is_allowed" {
  command = plan

  variables {
    layout = "Standard"
    zones  = ["ru-central1-b", "ru-central1-d", "ru-central1-f"]
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 0 && length(yandex_vpc_subnet.kube_e) == 0 && length(yandex_vpc_subnet.kube_b) == 1 && length(yandex_vpc_subnet.kube_d) == 1 && length(yandex_vpc_subnet.dynamic) == 1
    error_message = "expected benign slot drift to plan cleanly: f keeps its slot, only selected statics exist"
  }
}
