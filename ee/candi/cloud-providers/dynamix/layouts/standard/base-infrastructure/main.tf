# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_account" "account" {
   account_id = local.account_id
}

resource "decort_resgroup" "resource_group" {
  name = local.resource_group_name
  account_id = data.decort_account.account.account_id
  gid = local.grid
  def_net_type = "NONE"
}

resource "decort_vins" "vins" {
  name = local.vins_name
  rg_id = decort_resgroup.resource_group.id
  ipcidr = local.node_network_cidr
  ip {
    type = "DHCP"
  }
  ext_net_id = local.extnet_id
}
