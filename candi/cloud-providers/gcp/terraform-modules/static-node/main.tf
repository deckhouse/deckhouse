data "google_compute_subnetwork" "kube" {
  name = local.prefix
}

data "google_compute_zones" "available" {}

locals {
  zones       = length(local.configured_zones) > 0 ? local.configured_zones : data.google_compute_zones.available.names
  zones_count = length(local.zones)
  zone        = local.zones[var.nodeIndex % local.zones_count]
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
      metadata["user-data"]
    ]
  }
}
