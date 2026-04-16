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
  default = "ru-3a"
}

variable "region" {
  default = "ru-3"
}

variable "volume_type" {
  default = "fast.ru-3a"
}

variable "flavor_name_large" {
  default = "m1.large"
}

variable "flavor_name_xlarge" {
  default = "m1.xlarge"
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
  vm_instance = {
    bastion = { image_id = data.openstack_images_image_v2.astra_image.id, name = "astra" }
    master1 = { image_id = data.openstack_images_image_v2.astra_image.id, name = "astra" }
    master2 = { image_id = data.openstack_images_image_v2.redos_image.id, name = "redos" }
    master3 = { image_id = data.openstack_images_image_v2.alt_image.id, name = "alt" }
    system = { image_id = data.openstack_images_image_v2.mosos_image.id, name = "mosos" }
    # system = { image_id = data.openstack_images_image_v2.astra_image.id, name = "mosos" }
    worker1 = { image_id = data.openstack_images_image_v2.alt_image.id, name = "alt" }
    worker2 = { image_id = data.openstack_images_image_v2.redos_image.id, name = "redos" }
    worker3 = { image_id = data.openstack_images_image_v2.astra_image.id, name = "astra" }
  }
}

data "openstack_networking_network_v2" "external" {
  name = "external-network"
}

resource "openstack_networking_network_v2" "internal" {
  name           = "cse-${var.PREFIX}"
  admin_state_up = "true"
}

resource "openstack_networking_subnet_v2" "internal" {
  name        = "cse-${var.PREFIX}"
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

resource "openstack_networking_port_v2" "port_without_security" {
  for_each = local.vm_instance
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
    ip_address = each.key == "bastion" ? "192.168.199.254" : null
  }
}

resource "openstack_networking_router_v2" "router" {
  name                = "cse-${var.PREFIX}"
  admin_state_up      = "true"
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "router" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.internal.id
}

resource "openstack_compute_keypair_v2" "ssh" {
  name       = "cse-${var.PREFIX}-key"
  public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQCxvEtHR2d9rO6F3ooHAWFxIJdMKAgNVGx5cbP3F576ltMsUauBHAC02ti5vCggORHJlq3BmAyrDXLbfDFS+evxL8oOGEVFlp+lHiUSTQZCxAnhJFVkjgJ8poCYno35ZYhlOTZGI6fqIWV2HuHIJSk3fL0rqRwjCVV2pqQniR6SYUNYISN/RmPnchGVFw4mRLo5HxkXHVPBE3OSX7ihODhS09c+8nyErd8iDf8YljFqB8Oepe3f7nwxWQM/mUjsU70hAL4DEuORrtPwSqeLcUrX4uzc3vQFzPR81AdbtAZ8Vh4CbF7v5dLIqKR1AkCGc8nENEGLu/AWbCjyb9epqmbjKpMT+ogyzJZjNlRjJ2PaImIUhGCMQ8wN1W68pB6Kx9rXKYXpK57nwWwbG33JrmMFWZK7Lj4oRNJZjHRRhOGccCT1gXATmTXzCikehBV4KVHfmOjzK1K0lfUb5DihfhXoAQ+YCIwZaUwtL5BBeq6oRuD1UxsNcczfjgZ22bmdDDs= root@04c20a0dffea"
}

data "openstack_images_image_v2" "alt_image" {
  most_recent = true
  visibility  = "shared"
  name        = "Alt-sp-server-c10f-fstek"
}

data "openstack_images_image_v2" "astra_image" {
  most_recent = true
  visibility  = "shared"
  name        = "alse-1.8.3-fstek-6.12.34-1-generic"
}

data "openstack_images_image_v2" "redos_image" {
  most_recent = true
  visibility  = "shared"
  name        = "redos-STD-MINIMAL-7.3.4"
}

data "openstack_images_image_v2" "mosos_image" {
  most_recent = true
  visibility  = "private"
  name        = "MosOS-Arbat-15.5"
}

resource "openstack_blockstorage_volume_v3" "vm" {
  for_each             = local.vm_instance
  name                 = "cse-${var.PREFIX}-${each.key}-${each.value.name}"
  size                 = can(regex("^master", each.key)) ? "50" : "30"
  image_id             = each.value.image_id
  volume_type          = var.volume_type
  availability_zone    = var.az_zone
  enable_online_resize = true
  timeouts {
    create = "20m"
  }
  lifecycle {
    ignore_changes = [image_id]
  }
}

resource "openstack_compute_instance_v2" "vm" {
  for_each          = local.vm_instance
  name              = "cse-${var.PREFIX}-${each.key}-${each.value.name}"
  flavor_name       = can(regex("^master", each.key)) ? var.flavor_name_xlarge : var.flavor_name_large
  key_pair          = "cse-${var.PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.port_without_security[each.key].id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v3.vm[each.key].id
    source_type           = "volume"
    destination_type      = "volume"
    boot_index            = 0
    delete_on_termination = true
  }
}

resource "openstack_networking_floatingip_v2" "bastion" {
  port_id = openstack_networking_port_v2.port_without_security["bastion"].id
  pool    = "external-network"
}

output "node_ip_address_for_ssh" {
  value = { for k, vm_instance in openstack_compute_instance_v2.vm : "${k}_ssh_addr" => vm_instance.network[0].fixed_ip_v4 }
}

output "bastion_ip_address_for_ssh" {
  value = openstack_networking_floatingip_v2.bastion.address
}
