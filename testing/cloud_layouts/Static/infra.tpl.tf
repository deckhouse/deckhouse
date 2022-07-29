terraform {
  required_providers {
    openstack = {
      source = "terraform-provider-openstack/openstack"
    }
  }
  required_version = ">= 0.13"
}

provider "openstack" {
  auth_url = "https://cloud.flant.com/v3/"
  domain_name = "Default"
  tenant_name = "d8-tests"
  user_name = "e2e-tests"
  password = "${OS_PASSWORD}"
  region = "HetznerFinland"
}

data "openstack_networking_network_v2" "shared" {
  name = "shared"
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
    end = "192.168.199.254"
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

resource "openstack_networking_port_v2" "system_internal_without_security" {
  network_id = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}


resource "openstack_networking_router_v2" "router" {
  name = "candi-${PREFIX}"
  admin_state_up = "true"
  external_network_id = data.openstack_networking_network_v2.shared.id
}

resource "openstack_networking_router_interface_v2" "router" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.internal.id
}

resource "openstack_compute_keypair_v2" "ssh" {
  name = "candi-${PREFIX}-key"
  public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQDSNdUmV2ekit0rFrQE9IoRsVqKTJfR8h+skMYjXHBv/nJN6J2eBvQlebnhfZngxTvHYYxl0XeRu3KEz5v23gIidT21o9x0+tD4b2PcyZ24o64GwnF/oFnQ9mYBJDRisZNdXYPadTp/RafQ0qNUX/6h8vZYlSPM77dhW7Oyf6hcbaniAmOD30bO89UM//VHbllGgfhlIbU382/EnPOfGvAHReATADBBHmxxtTCLbu48rN35DlOtMgPob3ZwOsJI3keRrIZOf5qxeF3VB0Ox4inoR6PUzWMFLCJyIMp7hzY+JLakO4dqfvRJZjgTZHQUvjDs+aeUcH8tD4Wd5NDzmxnHLtJup0lkHkqgjo6vqWIcQeDXuXsk3+YGw0PwMpwO2HMVPs2SnfT6cZ+Mo6Dmq0t1EjtSBXLMe5C5aac5w6NrXuypRQDoce7p3uZP2TVsxmpyvkd6RyiWr+wuOOB3h/k8q+kRh4LKzivJMEkZoZeCxkJiIWDknxEAU1sl25W4hEU="
}

resource "openstack_compute_instance_v2" "master" {
  name = "candi-${PREFIX}-master-0"
  image_name = "ubuntu-18-04-cloud-amd64"
  flavor_name = "m1.large"
  key_pair = "candi-${PREFIX}-key"
  availability_zone = "nova"

  network {
    port = openstack_networking_port_v2.master_internal_without_security.id
  }

}

resource "openstack_compute_instance_v2" "system" {
  name = "candi-${PREFIX}-system-0"
  image_name = "ubuntu-18-04-cloud-amd64"
  flavor_name = "m1.large"
  key_pair = "candi-${PREFIX}-key"
  availability_zone = "nova"

  network {
    port = openstack_networking_port_v2.system_internal_without_security.id
  }

}

resource "openstack_compute_floatingip_v2" "master" {
  pool = data.openstack_networking_network_v2.shared.name
}

resource "openstack_compute_floatingip_v2" "system" {
  pool = data.openstack_networking_network_v2.shared.name
}

resource "openstack_compute_floatingip_associate_v2" "master" {
  floating_ip = openstack_compute_floatingip_v2.master.address
  instance_id = openstack_compute_instance_v2.master.id
}

resource "openstack_compute_floatingip_associate_v2" "system" {
  floating_ip = openstack_compute_floatingip_v2.system.address
  instance_id = openstack_compute_instance_v2.system.id
}

output "master_ip_address_for_ssh" {
  value = openstack_compute_floatingip_v2.master.address
}

output "system_ip_address_for_ssh" {
  value = openstack_compute_floatingip_v2.system.address
}

