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

variable "flavor_name_medium" {
  default = "m1.medium"
}

terraform {
  required_providers {
    openstack = {
      source = "terraform-provider-openstack/openstack"
    }
  }
  required_version = ">= 0.13"
}

provider "openstack" {
  auth_url = "https://api.selvpc.ru/identity/v3"
  domain_name = "48348"
  tenant_id = "ceda80a1b33844adb1cbddd20ee93585"
  user_name = "deckhouse-e2e"
  password = "${OS_PASSWORD}"
  region = var.region
}

locals {
  worker_images = {
    "redos" = data.openstack_images_image_v2.redos_image.id
    "opensuse" = data.openstack_images_image_v2.opensuse_image.id
    "rosa" = data.openstack_images_image_v2.rosa_image.id
  }
}


data "openstack_networking_network_v2" "external" {
  name = "external-network"
}

resource "openstack_networking_network_v2" "internal" {
  name           = "candi-${PREFIX}"
  admin_state_up = "true"
  port_security_enabled = "false"
}

resource "openstack_networking_subnet_v2" "internal" {
  name = "candi-${PREFIX}"
  network_id = openstack_networking_network_v2.internal.id
  cidr = "192.168.199.0/24"
  ip_version = 4
  enable_dhcp = "true"
  allocation_pool {
    start = "192.168.199.2"
    end = "192.168.199.253"
  }
  dns_nameservers = ["8.8.8.8", "8.8.4.4"]
}

resource "openstack_networking_port_v2" "master_internal_without_security" {
  network_id = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_port_v2" "bastion_internal_without_security" {
  network_id = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
    ip_address = "192.168.199.254"
  }
}

resource "openstack_networking_port_v2" "system_internal_without_security" {
  network_id = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_port_v2" "worker_internal_without_security" {
  for_each = { for image_name, image_id in local.worker_images : image_name => image_id }
  network_id = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

resource "openstack_networking_router_v2" "router" {
  name = "candi-${PREFIX}"
  admin_state_up = "true"
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "router" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.internal.id
}

resource "openstack_compute_keypair_v2" "ssh" {
  name = "candi-${PREFIX}-key"
  public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQClxHb/dtki3ucSBf9Jqx596QJjZd3TPaVncZV/JEyHQVECOFs2vvZ96mYE6MGYvxieT+K/dWSkOtpugHNqzSY0cjKcXr0WpBLNa+Sl4jC9eCEMdf23mHLkZKbdzcR9LB5eX1m6GsOb4sPH3sDcI5BEFfYDIOH423HUfLTQFLDHtuk9bSceGnslZmyTEymw4FCzlcYLb3oJ9SVvVkQ9jgWaJ1XopKIpJhPMw8c6mfipj20gf4webolqFH/43AusC/q2x96CAwLWYIsIPl6YJnVov+8yvSfcOPYVn5VzUaNrWjWEPegHf8wcwLE/QoEX7Xk6z+XoQz72999hV2LmVkecngm31XT20KYm2e6bZpTAayjxo/HznjwSiDvqxWi+lQgIv6uNbbKKBm2kafBPIRvfrNC3m3pS3eCW/nIRoa2D4UYthYPW4dh2BDrFbYhyZro3wzPuHFYuDer9ndqR0eIUaOl391L5aoTSI8D5N1fTLwHJOoqyzN7Y1u2JAki+vIEe7ypLTvmn+lKBS2TZYRnlxmuQZLTa+R0M/JM01nJqdk1rghUQlmNXe+nl53D3WIIjGW0JHH94Wbr57vwYkqXNGREyxU3djGBIu/0biJ6QxuKxD1YigtU7isiQEtWLYU9EDCbq8qWQYiAM2Q3+z6ygSzDFrkuT40UrW6IgwDGzLw== root@3500d8e6274f"
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
  name        = "openSUSE-Leap-15.6"
}

data "openstack_images_image_v2" "rosa_image" {
  most_recent = true
  visibility  = "shared"
  name        = "rosa-server-cobalt-20240613"
}

resource "openstack_blockstorage_volume_v3" "master" {
  name                 = "candi-${PREFIX}-master-0"
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
  name = "candi-${PREFIX}-master-0"
  flavor_name = var.flavor_name_large
  key_pair = "candi-${PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("astra-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.master_internal_without_security.id
  }

  block_device {
    uuid             = openstack_blockstorage_volume_v3.master.id
    source_type      = "volume"
    destination_type = "volume"
    boot_index       = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "bastion" {
  name                 = "candi-${PREFIX}-bastion-0"
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
  name = "candi-${PREFIX}-bastion"
  flavor_name = var.flavor_name_medium
  key_pair = "candi-${PREFIX}-key"
  availability_zone = var.az_zone

  network {
    port = openstack_networking_port_v2.bastion_internal_without_security.id
  }
  block_device {
    uuid             = openstack_blockstorage_volume_v3.bastion.id
    source_type      = "volume"
    destination_type = "volume"
    boot_index       = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "system" {
  name                 = "candi-${PREFIX}-system-0"
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
  name = "candi-${PREFIX}-system-0"
  flavor_name = var.flavor_name_large
  key_pair = "candi-${PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("alt-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.system_internal_without_security.id
  }

  block_device {
    uuid             = openstack_blockstorage_volume_v3.system.id
    source_type      = "volume"
    destination_type = "volume"
    boot_index       = 0
    delete_on_termination = true
  }
}

resource "openstack_blockstorage_volume_v3" "worker" {
  for_each = { for image_name, image_id in local.worker_images : image_name => image_id }
  name                 = "candi-${PREFIX}-worker-${each.key}"
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
  for_each = { for image_name, image_id in local.worker_images : image_name => image_id }
  name = "candi-${PREFIX}-worker-${each.key}"
  flavor_name = var.flavor_name_large
  key_pair = "candi-${PREFIX}-key"
  availability_zone = var.az_zone
  user_data = file("${each.key}-instance-bootstrap.sh")

  network {
    port = openstack_networking_port_v2.worker_internal_without_security[each.key].id
  }

  block_device {
    uuid             = openstack_blockstorage_volume_v3.worker[each.key].id
    source_type      = "volume"
    destination_type = "volume"
    boot_index       = 0
    delete_on_termination = true
  }
}

resource "openstack_networking_floatingip_v2" "bastion" {
  port_id             = openstack_networking_port_v2.bastion_internal_without_security.id
  pool                = "external-network"
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
