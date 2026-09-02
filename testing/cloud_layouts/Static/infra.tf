terraform {
  backend "s3" {
    bucket                      = "deckhouse-e2e-terraform-state"
    region                      = "ru-7"
    endpoint                    = "https://s3.ru-7.storage.selcloud.ru"
    skip_region_validation      = true
    skip_credentials_validation = true
  }
  required_version = ">= 0.14.0"
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "= 1.54.1"
    }
  }
}

variable "az_zone" {
  default = "ru-1c"
}

variable "region" {
  default = "ru-1"
}

variable "volume_type" {
  default = "fast.ru-1c"
}

variable "flavor_name_large" {
  default = "m1.large"
}

variable "flavor_name_medium" {
  default = "m1.medium"
}

variable "OS_PASSWORD" {}
variable "PREFIX" {}

provider "openstack" {
  auth_url    = "https://api.selvpc.ru/identity/v3"
  domain_name = "48348"
  tenant_id   = "ceda80a1b33844adb1cbddd20ee93585"
  user_name   = "deckhouse-e2e"
  password    = var.OS_PASSWORD
  region      = var.region
}

locals {
  worker_images = {
    "redos"    = data.openstack_images_image_v2.redos_image.id
    "opensuse" = data.openstack_images_image_v2.opensuse_image.id
    "rosa"     = data.openstack_images_image_v2.rosa_image.id
  }
}


data "openstack_networking_network_v2" "external" {
  name = "external-network"
}

resource "openstack_networking_network_v2" "internal" {
  name                  = "candi-${var.PREFIX}"
  admin_state_up        = "true"
  #port_security_enabled = "false"
}

resource "openstack_networking_subnet_v2" "internal" {
  name        = "candi-${var.PREFIX}"
  network_id  = openstack_networking_network_v2.internal.id
  cidr        = "192.168.199.0/24"
  ip_version  = 4
  enable_dhcp = "true"
  allocation_pool {
    start = "192.168.199.2"
    end   = "192.168.199.253"
  }
  dns_nameservers = ["8.8.8.8", "8.8.4.4"]
}

resource "openstack_networking_port_v2" "master_internal_without_security" {
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_port_v2" "bastion_internal_without_security" {
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id  = openstack_networking_subnet_v2.internal.id
    ip_address = "192.168.199.254"
  }
}

resource "openstack_networking_port_v2" "system_internal_without_security" {
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_port_v2" "worker_internal_without_security" {
  for_each       = {for image_name, image_id in local.worker_images : image_name => image_id}
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_router_v2" "router" {
  name                = "candi-${var.PREFIX}"
  admin_state_up      = "true"
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "router" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.internal.id
}

resource "openstack_compute_keypair_v2" "ssh" {
  name       = "candi-${var.PREFIX}-key"
  public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQCfAkEUbTk8ly/lZNS67hkSBsiG3hJAho4mag6McwT9iQ/1qcE7XrtaqxnVMt8zsscCpMuwPxOKJn41MvLiHm9YlZMYGCyNovrlmfL5da62nEkJR6iVjAqjRe6hXbw5PClZp2YkQjRVRhCPGf1zXF8WpdLFFYaXxXeHg+CeSecnxllT+WCyw6zvIdaK7ijuDxdGwgAr/qrn5xUzcwW6x7mYlkmLKbEBB7hm0+WQM73FYbaWRBHiCN/EZ/oy3GEXXlqlxd9KLBcA/K13yB+zaMSVdtRBlUvqZWocYCD8KSbu8mzFKyZ7cb00r66N9PUpgnCndvVFX9NzfzXjjZkNuYop5dGM+siv1qUG+L02DGBHhiFo7rMClS6nry4ujSAKwxqoSOVKjxiRE47uD2b06mk+sSLhickQAWv/WVdI529KjnNCCTl4KYZ0Z5d3yrOMT/Kr+Xwn1BRXqt381nVWRzZEiieyV5S3fx0F8HjJhU1xVfmnhxBBiBpVzUP4qxDCa8hyvOv5cK31GrQKQR9REGYL4A7N51+9CI3/qH4YQfWZrZsumhAizj71bTD94Hi80wkHCupX3LfvnrP0gQ0XGnEkbCTMPrLF0HwZ/omddYBGlCVEFhgcVcKdE8mF1nGqwIqtj3n1lqNRiMuqFqYLBOKNG+sfzhi+W5B9i7tn9kFO4Q== root@e2e-tests"
}

data "openstack_images_image_v2" "astra_image" {
  most_recent = true
  visibility  = "shared"
  name        = "alse-1.8.1-base"
}

data "openstack_images_image_v2" "alt_image" {
  most_recent = true
  visibility  = "shared"
  name        = "alt-p11-cloud-x86_64"
}

data "openstack_images_image_v2" "redos_image" {
  most_recent = true
  visibility  = "shared"
  name        = "redos-STD-MINIMAL-8.0.0"
}

data "openstack_images_image_v2" "opensuse_image" {
  most_recent = true
  visibility  = "shared"
  name        = "openSUSE-Leap-15.6-nocontainerd"
}

data "openstack_images_image_v2" "rosa_image" {
  most_recent = true
  visibility  = "shared"
  name        = "rosa-server-cobalt-20240613"
}

resource "openstack_blockstorage_volume_v3" "master" {
  name                 = "candi-${var.PREFIX}-master-0"
  size                 = "30"
  image_id             = data.openstack_images_image_v2.astra_image.id
  volume_type          = var.volume_type
  availability_zone    = var.az_zone
  enable_online_resize = true
  lifecycle {
    ignore_changes = [image_id]
  }
}

resource "openstack_compute_instance_v2" "master" {
  name              = "candi-${var.PREFIX}-master-0"
  flavor_name       = var.flavor_name_large
  key_pair          = "candi-${var.PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("astra-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.master_internal_without_security.id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v3.master.id
    source_type           = "volume"
    destination_type      = "volume"
    boot_index            = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "bastion" {
  name                 = "candi-${var.PREFIX}-bastion-0"
  size                 = "60"
  image_id             = data.openstack_images_image_v2.astra_image.id
  volume_type          = var.volume_type
  availability_zone    = var.az_zone
  enable_online_resize = true
  lifecycle {
    ignore_changes = [image_id]
  }
}

resource "openstack_compute_instance_v2" "bastion" {
  name              = "candi-${var.PREFIX}-bastion"
  flavor_name       = var.flavor_name_medium
  key_pair          = "candi-${var.PREFIX}-key"
  availability_zone = var.az_zone

  network {
    port = openstack_networking_port_v2.bastion_internal_without_security.id
  }
  block_device {
    uuid                  = openstack_blockstorage_volume_v3.bastion.id
    source_type           = "volume"
    destination_type      = "volume"
    boot_index            = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "system" {
  name                 = "candi-${var.PREFIX}-system-0"
  size                 = "30"
  image_id             = data.openstack_images_image_v2.alt_image.id
  volume_type          = var.volume_type
  availability_zone    = var.az_zone
  enable_online_resize = true
  lifecycle {
    ignore_changes = [image_id]
  }
}

resource "openstack_compute_instance_v2" "system" {
  name              = "candi-${var.PREFIX}-system-0"
  flavor_name       = var.flavor_name_large
  key_pair          = "candi-${var.PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("alt-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.system_internal_without_security.id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v3.system.id
    source_type           = "volume"
    destination_type      = "volume"
    boot_index            = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "worker" {
  for_each             = {for image_name, image_id in local.worker_images : image_name => image_id}
  name                 = "candi-${var.PREFIX}-worker-${each.key}"
  size                 = "30"
  image_id             = each.value
  volume_type          = var.volume_type
  availability_zone    = var.az_zone
  enable_online_resize = true
  lifecycle {
    ignore_changes = [image_id]
  }
}

resource "openstack_compute_instance_v2" "worker" {
  for_each          = {for image_name, image_id in local.worker_images : image_name => image_id}
  name              = "candi-${var.PREFIX}-worker-${each.key}"
  flavor_name       = var.flavor_name_large
  key_pair          = "candi-${var.PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("${each.key}-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.worker_internal_without_security[each.key].id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v3.worker[each.key].id
    source_type           = "volume"
    destination_type      = "volume"
    boot_index            = 0
    delete_on_termination = true
  }
}

resource "openstack_networking_floatingip_v2" "bastion" {
  port_id = openstack_networking_port_v2.bastion_internal_without_security.id
  pool    = "external-network"
}

output "master_ip_address_for_ssh" {
  value = lookup(openstack_compute_instance_v2.master.network[0], "fixed_ip_v4")
}

output "system_ip_address_for_ssh" {
  value = lookup(openstack_compute_instance_v2.system.network[0], "fixed_ip_v4")
}

output "worker_redos_ip_address_for_ssh" {
  value = lookup(openstack_compute_instance_v2.worker["redos"].network[0], "fixed_ip_v4")
}

output "worker_opensuse_ip_address_for_ssh" {
  value = lookup(openstack_compute_instance_v2.worker["opensuse"].network[0], "fixed_ip_v4")
}

output "worker_rosa_ip_address_for_ssh" {
  value = lookup(openstack_compute_instance_v2.worker["rosa"].network[0], "fixed_ip_v4")
}

output "bastion_ip_address_for_ssh" {
  value = openstack_networking_floatingip_v2.bastion.address
}
