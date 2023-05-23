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

resource "google_compute_address" "master" {
  count = local.disable_external_ip == true ? 0 : 1
  name  = join("-", [local.prefix, "master", var.nodeIndex])
}

resource "google_compute_disk" "kubernetes_data" {
  zone   = local.zone
  name   = join("-", [local.prefix, "kubernetes-data", var.nodeIndex])
  type   = "pd-ssd"
  size   = local.etcd_disk_size_gb
  labels = local.additional_labels
}

resource "google_compute_instance" "master" {
  zone         = local.zone
  name         = join("-", [local.prefix, "master", var.nodeIndex])
  machine_type = local.machine_type
  boot_disk {
    initialize_params {
      type  = "pd-ssd"
      image = local.image
      size  = local.disk_size_gb
    }
  }
  attached_disk {
    source      = google_compute_disk.kubernetes_data.self_link
    device_name = google_compute_disk.kubernetes_data.name
  }
  network_interface {
    subnetwork = data.google_compute_subnetwork.kube.self_link
    dynamic "access_config" {
      for_each = local.disable_external_ip == true ? [] : list(google_compute_address.master[0])
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
}
