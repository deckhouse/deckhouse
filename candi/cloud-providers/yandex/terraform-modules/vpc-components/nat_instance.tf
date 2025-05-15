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

data "yandex_compute_image" "nat_image" {
  count  = local.is_with_nat_instance ? 1 : 0
  family = "nat-instance-ubuntu-2204"
}

data "yandex_vpc_subnet" "user_internal_subnet" {
  count = var.nat_instance_internal_subnet_id == null ? 0 : 1
  subnet_id = var.nat_instance_internal_subnet_id
}

data "yandex_vpc_subnet" "external_subnet" {
  count = var.nat_instance_external_subnet_id == null ? 0 : 1
  subnet_id = var.nat_instance_external_subnet_id
}

locals {
  user_internal_subnet_zone = var.nat_instance_internal_subnet_id == null ? null : data.yandex_vpc_subnet.user_internal_subnet[0].zone
  external_subnet_zone = var.nat_instance_external_subnet_id == null ? null : join("", data.yandex_vpc_subnet.external_subnet.*.zone) # https://github.com/hashicorp/terraform/issues/23222#issuecomment-547462883
  internal_subnet_zone = local.user_internal_subnet_zone == null ? (local.external_subnet_zone == null ? "ru-central1-a" : local.external_subnet_zone) : local.user_internal_subnet_zone

  # used for backward compatability
  zone_to_subnet_id = tomap({
    "ru-central1-a" = local.should_create_subnets ? yandex_vpc_subnet.kube_a[0].id : (local.not_have_existing_subnet_a ? null : data.yandex_vpc_subnet.kube_a[0].id)
    "ru-central1-b" = local.should_create_subnets ? yandex_vpc_subnet.kube_b[0].id : (local.not_have_existing_subnet_b ? null : data.yandex_vpc_subnet.kube_b[0].id)
    "ru-central1-d" = local.should_create_subnets ? yandex_vpc_subnet.kube_d[0].id : (local.not_have_existing_subnet_d ? null : data.yandex_vpc_subnet.kube_d[0].id)
  })

  # used for backward compatability
  # we can not use one map because we will get cycle
  # local.nat_instance_internal_address_calculated uses in route table and yandex_vpc_subnet.kube_* depend on route table
  zone_to_cidr = tomap({
    "ru-central1-a" = local.should_create_subnets ? local.kube_a_v4_cidr_block : (local.not_have_existing_subnet_a ? null : data.yandex_vpc_subnet.kube_a[0].v4_cidr_blocks[0])
    "ru-central1-b" = local.should_create_subnets ? local.kube_b_v4_cidr_block : (local.not_have_existing_subnet_b ? null : data.yandex_vpc_subnet.kube_b[0].v4_cidr_blocks[0])
    "ru-central1-d" = local.should_create_subnets ? local.kube_d_v4_cidr_block : (local.not_have_existing_subnet_d ? null : data.yandex_vpc_subnet.kube_d[0].v4_cidr_blocks[0])
  })

  # if user set internal subnet id for nat instance get cidr from its subnet
  user_internal_subnet_cidr = var.nat_instance_internal_subnet_id == null ? null : data.yandex_vpc_subnet.user_internal_subnet[0].v4_cidr_blocks[0]

  nat_instance_internal_cidr = var.nat_instance_internal_subnet_cidr != null ? var.nat_instance_internal_subnet_cidr : (local.user_internal_subnet_cidr != null ? local.user_internal_subnet_cidr : local.zone_to_cidr[local.internal_subnet_zone])

  # if user passes nat instance internal address directly (deprecated, keep for backward compatibility) use passed address,
  # else get 10 host address from cidr which got in previous step
  nat_instance_internal_address_calculated = var.nat_instance_internal_address != null ? var.nat_instance_internal_address : cidrhost(local.nat_instance_internal_cidr, 10)

  assign_external_ip_address = var.nat_instance_external_subnet_id == null ? true : false

  user_data = <<-EOT
    #!/bin/bash

    echo "${var.nat_instance_ssh_key}" >> /home/ubuntu/.ssh/authorized_keys

    cat > /etc/netplan/49-second-interface.yaml <<"EOF"
    network:
      version: 2
      ethernets:
        eth1:
          dhcp4: yes
          dhcp4-overrides:
            use-dns: false
            use-ntp: false
            use-hostname: false
            use-routes: false
            use-domains: false
          routes:
          - to: "${var.node_network_cidr}"
            scope: link
    EOF

    netplan apply

    # Load conntrack module at boot time
    cat > /etc/modules-load.d/ip_conntrack.conf <<EOF
    ip_conntrack
    EOF

    cat > /etc/sysctl.d/999-netfilter-nf-conntrack.conf <<EOF
    net.netfilter.nf_conntrack_max=1048576
    net.netfilter.nf_conntrack_tcp_timeout_time_wait=30
    EOF

    sysctl -p /etc/sysctl.d/999-netfilter-nf-conntrack.conf
  EOT
}

resource "yandex_vpc_subnet" "nat_instance" {
  count = local.is_with_nat_instance ? (var.nat_instance_internal_subnet_cidr != null ? 1 : 0) : 0

  name           = join("-", [var.prefix, "nat"])
  zone           = local.internal_subnet_zone
  network_id     = var.network_id
  v4_cidr_blocks = [local.nat_instance_internal_cidr]
}

resource "yandex_compute_instance" "nat_instance" {
  count = local.is_with_nat_instance ? 1 : 0

  allow_stopping_for_update = true

  name                      = join("-", [var.prefix, "nat"])
  hostname                  = join("-", [var.prefix, "nat"])
  zone                      = local.internal_subnet_zone
  network_acceleration_type = "software_accelerated"

  platform_id = var.nat_instance_platform
  resources {
    cores  = var.nat_instance_cores
    memory = var.nat_instance_memory
  }

  boot_disk {
    initialize_params {
      type = "network-hdd"
      image_id = data.yandex_compute_image.nat_image.0.image_id
      size = 13
    }
  }

  dynamic "network_interface" {
    for_each = var.nat_instance_external_subnet_id != null ? [var.nat_instance_external_subnet_id] : []
    content {
      subnet_id = network_interface.value
      ip_address = var.nat_instance_external_address
      nat       = false
    }
  }

  network_interface {
    subnet_id      = var.nat_instance_internal_subnet_cidr != null ? yandex_vpc_subnet.nat_instance[0].id: (var.nat_instance_internal_subnet_id != null ? var.nat_instance_internal_subnet_id: local.zone_to_subnet_id[local.internal_subnet_zone])
    ip_address     = local.nat_instance_internal_address_calculated
    nat            = local.assign_external_ip_address
    nat_ip_address = local.assign_external_ip_address ? var.nat_instance_external_address : null
  }

  lifecycle {
    ignore_changes = [
      hostname,
      metadata,
      boot_disk[0].initialize_params[0].image_id,
      boot_disk[0].initialize_params[0].size,
      network_acceleration_type,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }

  metadata = {
    user-data = local.user_data
  }
}
