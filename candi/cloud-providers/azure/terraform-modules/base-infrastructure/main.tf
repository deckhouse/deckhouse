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

resource "azurerm_resource_group" "kube" {
  name     = local.prefix
  location = local.location
  tags     = local.additional_tags
}

resource "azurerm_virtual_network" "kube" {
  name                = local.prefix
  address_space       = [local.vnet_cidr]
  location            = azurerm_resource_group.kube.location
  resource_group_name = azurerm_resource_group.kube.name
  tags                = local.additional_tags
  dns_servers         = local.nameservers
}

resource "azurerm_subnet" "kube" {
  name                 = local.prefix
  resource_group_name  = azurerm_resource_group.kube.name
  virtual_network_name = azurerm_virtual_network.kube.name
  address_prefixes     = [local.subnet_cidr]
  service_endpoints    = local.service_endpoints
}

resource "azurerm_route_table" "kube" {
  name                          = local.prefix
  location                      = azurerm_resource_group.kube.location
  resource_group_name           = azurerm_resource_group.kube.name
  disable_bgp_route_propagation = true
  tags                          = local.additional_tags
}

resource "azurerm_subnet_route_table_association" "kube" {
  subnet_id      = azurerm_subnet.kube.id
  route_table_id = azurerm_route_table.kube.id
}

resource "azurerm_network_security_group" "kube" {
  name                = local.prefix
  location            = azurerm_resource_group.kube.location
  resource_group_name = azurerm_resource_group.kube.name
  tags                = local.additional_tags
}

resource "azurerm_network_security_rule" "icmp" {
  name                        = "AllowIcmp"
  priority                    = 100
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Icmp"
  source_port_range           = "*"
  destination_port_range      = "*"
  source_address_prefix       = "*"
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.kube.name
  network_security_group_name = azurerm_network_security_group.kube.name
}

resource "azurerm_network_security_rule" "ssh" {
  name                        = "AllowSsh"
  priority                    = 101
  direction                   = "Inbound"
  access                      = "Allow"
  protocol                    = "Tcp"
  source_port_range           = "*"
  destination_port_range      = "22"
  source_address_prefix       = local.ssh_allow_list == null ? "*" : null
  source_address_prefixes     = local.ssh_allow_list == null ? null : local.ssh_allow_list
  destination_address_prefix  = "*"
  resource_group_name         = azurerm_resource_group.kube.name
  network_security_group_name = azurerm_network_security_group.kube.name
}

resource "azurerm_subnet_network_security_group_association" "kube" {
  subnet_id                 = azurerm_subnet.kube.id
  network_security_group_id = azurerm_network_security_group.kube.id
}

# nat_gateway
resource "azurerm_nat_gateway" "kube" {
  count               = local.nat_gateway_public_ip_count > 0 ? 1 : 0
  name                = local.prefix
  location            = azurerm_resource_group.kube.location
  resource_group_name = azurerm_resource_group.kube.name
  sku_name            = "Standard"
  tags                = local.additional_tags
}

resource "azurerm_public_ip" "kube" {
  count               = local.nat_gateway_public_ip_count
  name                = join("-", [local.prefix, count.index])
  location            = azurerm_resource_group.kube.location
  resource_group_name = azurerm_resource_group.kube.name
  allocation_method   = "Static"
  sku                 = "Standard"
  tags                = local.additional_tags
}

resource "azurerm_nat_gateway_public_ip_association" "kube" {
  count                = local.nat_gateway_public_ip_count
  nat_gateway_id       = azurerm_nat_gateway.kube[0].id
  public_ip_address_id = azurerm_public_ip.kube[count.index].id
}

resource "azurerm_subnet_nat_gateway_association" "kube" {
  count          = local.nat_gateway_public_ip_count > 0 ? 1 : 0
  subnet_id      = azurerm_subnet.kube.id
  nat_gateway_id = azurerm_nat_gateway.kube[0].id
}

# peering
data "azurerm_virtual_network" "remote" {
  for_each            = local.peered_vnets
  name                = each.value.vnetName
  resource_group_name = each.value.resourceGroupName
}

locals {
  peered_vnets_data = [for v in data.azurerm_virtual_network.remote : v]
}

resource "azurerm_virtual_network_peering" "kube-with-remote" {
  count                        = length(local.peered_vnets)
  name                         = join("-with-", [azurerm_virtual_network.kube.name, local.peered_vnets_data[count.index].name])
  resource_group_name          = azurerm_resource_group.kube.name
  virtual_network_name         = azurerm_virtual_network.kube.name
  remote_virtual_network_id    = local.peered_vnets_data[count.index].id
  allow_virtual_network_access = true
  allow_forwarded_traffic      = true

  lifecycle {
    ignore_changes = [allow_gateway_transit]
  }
}

resource "azurerm_virtual_network_peering" "remote-with-kube" {
  count                        = length(local.peered_vnets)
  name                         = join("-with-", [local.peered_vnets_data[count.index].name, azurerm_virtual_network.kube.name])
  resource_group_name          = local.peered_vnets_data[count.index].resource_group_name
  virtual_network_name         = local.peered_vnets_data[count.index].name
  remote_virtual_network_id    = azurerm_virtual_network.kube.id
  allow_virtual_network_access = true
  allow_forwarded_traffic      = true

  lifecycle {
    ignore_changes = [allow_gateway_transit]
  }
}
