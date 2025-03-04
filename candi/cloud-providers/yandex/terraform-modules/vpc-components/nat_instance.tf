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
  family = var.nat_instance_family_id
}

data "yandex_vpc_subnet" "user_internal_subnet" {
  count = var.nat_instance_internal_subnet_id == null ? 0 : 1
  subnet_id = var.nat_instance_internal_subnet_id
}

data "yandex_vpc_subnet" "external_subnet" {
  count = var.nat_instance_external_subnet_id == null ? 0 : 1
  subnet_id = var.nat_instance_external_subnet_id
}

data "yandex_compute_instance" "nat_instance" {
  count = local.is_with_nat_instance ? (var.nat_instance_internal_subnet_cidr == null ? 1 : 0) : 0
  name = join("-", [var.prefix, "nat"])
}

locals {
  user_internal_subnet_zone = var.nat_instance_internal_subnet_id == null ? null : data.yandex_vpc_subnet.user_internal_subnet[0].zone
  external_subnet_zone = var.nat_instance_external_subnet_id == null ? null : join("", data.yandex_vpc_subnet.external_subnet.*.zone) # https://github.com/hashicorp/terraform/issues/23222#issuecomment-547462883
  internal_subnet_zone = local.user_internal_subnet_zone == null ? (local.external_subnet_zone == null ? "ru-central1-a" : local.external_subnet_zone) : local.user_internal_subnet_zone

  # if user set internal subnet id for nat instance get cidr from its subnet
  user_internal_subnet_cidr = var.nat_instance_internal_subnet_id == null ? null : data.yandex_vpc_subnet.user_internal_subnet[0].v4_cidr_blocks[0]

  nat_instance_internal_cidr = var.nat_instance_internal_subnet_cidr != null ? var.nat_instance_internal_subnet_cidr : local.user_internal_subnet_cidr

  # if user pass nat instance internal address directly (it for backward compatibility) use passed address,
  # else get 10 host address from cidr which got in previous step
  nat_instance_internal_address_calculated = local.is_with_nat_instance ? (var.nat_instance_internal_address != null ? var.nat_instance_internal_address : (local.nat_instance_internal_cidr != null ? cidrhost(local.nat_instance_internal_cidr, 10): null)) : null

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
    subnet_id      = var.nat_instance_internal_subnet_cidr != null ? yandex_vpc_subnet.nat_instance[0].id: (var.nat_instance_internal_subnet_id != null ? var.nat_instance_internal_subnet_id: data.yandex_compute_instance.nat_instance[0].network_interface.0.subnet_id)
    ip_address     = local.nat_instance_internal_address_calculated != null ? local.nat_instance_internal_address_calculated : data.yandex_compute_instance.nat_instance[0].network_interface.0.ip_address
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
