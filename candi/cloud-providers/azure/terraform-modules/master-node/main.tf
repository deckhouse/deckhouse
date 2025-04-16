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

data "azurerm_resource_group" "kube" {
  name = local.prefix
}

data "azurerm_subnet" "kube" {
  name                 = local.prefix
  resource_group_name  = data.azurerm_resource_group.kube.name
  virtual_network_name = local.prefix
}

locals {
  zones_count = length(local.zones)
  zone        = local.zones[var.nodeIndex % local.zones_count]
  default_cloud_config = <<-EOF
  #cloud-config
  mounts:
  - [ ephemeral0, /mnt/resource ]
  EOF
}

resource "azurerm_public_ip" "master" {
  count               = local.enable_external_ip == true ? 1 : 0
  name                = join("-", [local.prefix, "master", var.nodeIndex])
  resource_group_name = data.azurerm_resource_group.kube.name
  location            = data.azurerm_resource_group.kube.location
  zones               = [local.zone] # Please Note: Availability Zones are only supported with a Standard SKU and in select regions at this time. Standard SKU Public IP Addresses that do not specify a zone are zone redundant by default.
  sku                 = "Standard"   # require for allocation_method=Static
  allocation_method   = "Static"
  tags                = local.additional_tags
}

resource "azurerm_network_interface" "master" {
  name                = join("-", [local.prefix, "master", var.nodeIndex])
  location            = data.azurerm_resource_group.kube.location
  resource_group_name = data.azurerm_resource_group.kube.name

  ip_forwarding_enabled          = true
  accelerated_networking_enabled = local.accelerated_networking

  ip_configuration {
    name                          = local.prefix
    subnet_id                     = data.azurerm_subnet.kube.id
    private_ip_address_allocation = "Dynamic"
    public_ip_address_id          = local.enable_external_ip == true ? azurerm_public_ip.master[0].id : null
  }

  tags = local.additional_tags
}

resource "azurerm_linux_virtual_machine" "master" {
  name                = join("-", [local.prefix, "master", var.nodeIndex])
  resource_group_name = data.azurerm_resource_group.kube.name
  location            = data.azurerm_resource_group.kube.location
  zone                = local.zone
  size                = local.machine_size
  admin_username      = local.admin_username
  network_interface_ids = [
    azurerm_network_interface.master.id,
  ]

  admin_ssh_key {
    username   = local.admin_username
    public_key = local.ssh_public_key
  }

  os_disk {
    caching              = "ReadWrite"
    storage_account_type = local.disk_type
    disk_size_gb         = local.disk_size_gb
  }

  source_image_reference {
    publisher = local.image_publisher
    offer     = local.image_offer
    sku       = local.image_sku
    version   = local.image_version
  }

  custom_data = var.cloudConfig != "" ? var.cloudConfig : base64encode(local.default_cloud_config)

  tags = local.additional_tags

  lifecycle {
    ignore_changes = [
      custom_data
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}

resource "azurerm_managed_disk" "kubernetes_data" {
  name                 = join("-", [local.prefix, "kubernetes-data", var.nodeIndex])
  resource_group_name  = data.azurerm_resource_group.kube.name
  location             = data.azurerm_resource_group.kube.location
  zone                =  local.zone
  storage_account_type = local.disk_type
  create_option        = "Empty"
  disk_size_gb         = local.etcd_disk_size_gb
  tags                 = local.additional_tags

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}

resource "azurerm_virtual_machine_data_disk_attachment" "kubernetes_data" {
  managed_disk_id    = azurerm_managed_disk.kubernetes_data.id
  virtual_machine_id = azurerm_linux_virtual_machine.master.id
  lun                = "10" # this value used to determine the disk name in bashible (000_discover_kubernetes_data_device_path.sh.tpl)
  caching            = "ReadWrite"
}
