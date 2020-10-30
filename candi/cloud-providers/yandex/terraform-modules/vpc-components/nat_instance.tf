data "yandex_compute_image" "nat_image" {
  count = var.should_create_nat_instance ? 1 : 0
  family = "nat-instance-ubuntu"
}

data "yandex_vpc_subnet" "external_subnet" {
  count = var.nat_instance_external_subnet_id == null ? 0 : 1
  subnet_id = var.nat_instance_external_subnet_id
}

locals {
  internal_subnet_id = var.nat_instance_internal_subnet_id == null ? yandex_vpc_subnet.kube_c.id : var.nat_instance_internal_subnet_id

  external_subnet_zone = var.nat_instance_external_subnet_id == null ? null : join("", data.yandex_vpc_subnet.external_subnet.*.zone) # https://github.com/hashicorp/terraform/issues/23222#issuecomment-547462883
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
    EOF

    netplan apply
  EOT
}

resource "yandex_compute_instance" "nat_instance" {
  count = var.should_create_nat_instance ? 1 : 0

  name         = join("-", [var.prefix, "nat"])
  hostname     = join("-", [var.prefix, "nat"])
  zone         = var.nat_instance_external_subnet_id == null ? yandex_vpc_subnet.kube_c.zone : local.external_subnet_zone

  platform_id  = "standard-v2"
  resources {
    cores  = 2
    memory = 2
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
    subnet_id      = var.nat_instance_internal_subnet_id == null ? yandex_vpc_subnet.kube_c.id : var.nat_instance_internal_subnet_id
    nat            = local.assign_external_ip_address
    nat_ip_address = local.assign_external_ip_address ? var.nat_instance_external_address : null
  }

  lifecycle {
    ignore_changes = [
      hostname,
      metadata,
      boot_disk[0].initialize_params[0].image_id,
      boot_disk[0].initialize_params[0].size
    ]
  }

  metadata = {
    user-data = local.user_data
  }

  depends_on = [yandex_vpc_subnet.kube_c]
}
