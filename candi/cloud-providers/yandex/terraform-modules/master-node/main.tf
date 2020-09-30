data "yandex_vpc_subnet" "kube_a" {
  name = "${local.prefix}-a"
}

data "yandex_vpc_subnet" "kube_b" {
  name = "${local.prefix}-b"
}

data "yandex_vpc_subnet" "kube_c" {
  name = "${local.prefix}-c"
}

locals {
  zone_to_subnet = {
    "ru-central1-a" = data.yandex_vpc_subnet.kube_a
    "ru-central1-b" = data.yandex_vpc_subnet.kube_b
    "ru-central1-c" = data.yandex_vpc_subnet.kube_c
  }
  configured_zones = lookup(local.master_instance_class, "zones", [])
  subnets = length(local.configured_zones) > 0 ? [for z in local.configured_zones : local.zone_to_subnet[z]] : values(local.zone_to_subnet)
  internal_subnet = element(local.subnets, var.nodeIndex)

  external_ip_address = length(local.external_ip_addresses) > 0 ? local.external_ip_addresses[var.nodeIndex] : null
  assign_external_ip_address = (local.external_subnet_id == null) && (local.external_ip_address != null) ? true : false
}

resource "yandex_compute_disk" "kubernetes_data" {
  name = join("-", [local.prefix, "kubernetes-data", var.nodeIndex])
  description = "volume for etcd and kubernetes certs"
  size = 10
  zone = local.internal_subnet.zone
  type = "network-ssd"
}

resource "yandex_compute_instance" "master" {
  name         = join("-", [local.prefix, "master", var.nodeIndex])
  hostname     = join("-", [local.prefix, "master", var.nodeIndex])
  zone         = local.internal_subnet.zone

  platform_id  = "standard-v2"
  resources {
    cores  = local.cores
    memory = local.memory
  }

  boot_disk {
    initialize_params {
      type = "network-ssd"
      image_id = local.image_id
      size = local.disk_size_gb

    }
  }

  secondary_disk {
    disk_id = yandex_compute_disk.kubernetes_data.id
    auto_delete = "false"
    device_name = "kubernetes-data"
  }

  dynamic "network_interface" {
    for_each = local.external_subnet_id != null ? [local.external_subnet_id] : []
    content {
      subnet_id = network_interface.value
      nat       = false
    }
  }

  network_interface {
    subnet_id = local.internal_subnet.id
    nat       = local.assign_external_ip_address
    nat_ip_address = local.assign_external_ip_address && (local.external_ip_address != "Auto") ? local.external_ip_address : null
  }

  lifecycle {
    ignore_changes = [
      metadata,
      secondary_disk,
    ]
  }

  metadata = {
    ssh-keys = "user:${local.ssh_public_key}"
    user-data = base64decode(var.cloudConfig)
  }
}
