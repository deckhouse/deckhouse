# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_account_list" "accounts" {
   name = local.account
}

data "decort_locations_list" "locations" {
   name = local.location
}

data "decort_extnet_list" "extnets" {
  name = local.extnet_name
}

locals {
  account_id = data.decort_account_list.accounts.items[0].account_id
  gid = data.decort_locations_list.locations.items[0].gid
  extnet_id = data.decort_extnet_list.extnets.items[0].net_id
}

resource "decort_resgroup" "decort_resource_group" {
  name = local.resource_group_name
  account_id = local.account_id
  gid = local.gid
  def_net_type = "NONE"
}

resource "decort_vins" "vins" {
  name = local.vins_name
  rg_id = decort_resgroup.decort_resource_group.rg_id
  ipcidr = local.node_network_cidr
  ip {
    type = "DHCP"
  }
  ext_net_id = local.extnet_id
  dns = length(local.nameservers) > 0 ? local.nameservers : []
}
