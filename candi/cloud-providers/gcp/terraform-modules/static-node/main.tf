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

data "google_compute_subnetwork" "kube" {
  name = local.prefix
}

data "google_compute_zones" "available" {}

locals {
  zone = element(local.zones, var.nodeIndex)
}

resource "google_compute_address" "static" {
  count = local.disable_external_ip == true ? 0 : 1
  name  = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
}

resource "google_compute_instance" "static" {
  zone         = local.zone
  name         = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
  machine_type = local.machine_type
  boot_disk {
    initialize_params {
      type  = local.disk_type
      image = local.image
      size  = local.disk_size_gb
    }
  }
  network_interface {
    subnetwork = data.google_compute_subnetwork.kube.self_link
    dynamic "access_config" {
      for_each = local.disable_external_ip == true ? [] : list(google_compute_address.static[0])
      content {
        nat_ip = access_config.value["address"]
      }
    }
  }
  allow_stopping_for_update = true
  can_ip_forward            = true
  tags                      = distinct(concat([local.prefix], local.additional_network_tags))
  labels                    = local.additional_labels
  metadata = {
    ssh-keys  = "${local.ssh_user}:${local.ssh_key}"
    user-data = base64decode(var.cloudConfig)
  }

  lifecycle {
    ignore_changes = [
      attached_disk,
      metadata["user-data"]
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}
