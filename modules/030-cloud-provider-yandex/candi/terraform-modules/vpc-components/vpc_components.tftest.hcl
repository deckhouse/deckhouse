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
# This module owns the network: it either carves four subnets out of nodeNetworkCIDR or
# adopts the ones the operator already has, and it routes egress either through a cloud
# NAT gateway (Standard) or through a NAT instance (WithNATInstance). Every case below
# pins which resources exist, because getting this wrong recreates the whole VPC.

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

  # With no existing subnets the module creates one per zone, derived from nodeNetworkCIDR.
  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 1 && length(yandex_vpc_subnet.kube_b) == 1 && length(yandex_vpc_subnet.kube_e) == 1 && length(yandex_vpc_subnet.kube_d) == 1
    error_message = "expected a subnet per zone to be created when no existing subnets are given"
  }

  # The CIDR split is what nodes get their addresses from: /16 into four /18 blocks.
  assert {
    condition     = yandex_vpc_subnet.kube_a[0].v4_cidr_blocks == tolist(["10.60.0.0/18"])
    error_message = "expected the first subnet to take the first quarter of nodeNetworkCIDR"
  }

  assert {
    condition     = yandex_vpc_subnet.kube_b[0].v4_cidr_blocks == tolist(["10.60.64.0/18"])
    error_message = "expected the second subnet to take the second quarter of nodeNetworkCIDR"
  }

  # Standard routes egress through a cloud NAT gateway and creates no NAT instance.
  assert {
    condition     = length(yandex_vpc_gateway.kube) == 1
    error_message = "expected a NAT gateway in the Standard layout"
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
    }
  }

  assert {
    condition     = length(yandex_vpc_subnet.kube_a) == 0 && length(yandex_vpc_subnet.kube_b) == 0 && length(yandex_vpc_subnet.kube_e) == 0 && length(yandex_vpc_subnet.kube_d) == 0
    error_message = "expected no subnets to be created when existing ones are supplied"
  }

  assert {
    condition     = length(data.yandex_vpc_subnet.kube_a) == 1
    error_message = "expected the existing subnet to be read instead of created"
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
    condition     = length(data.yandex_vpc_subnet.kube_b) == 0 && length(yandex_vpc_subnet.kube_b) == 0
    error_message = "expected the zones absent from the map to be neither read nor created"
  }
}
