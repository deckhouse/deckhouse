# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_account_list" "accounts" {
   name = local.account
}

data "decort_locations_list" "locations" {
   name = local.location
}

locals {
  account_id = data.decort_account_list.accounts.items[0].account_id
  gid = data.decort_locations_list.locations.items[0].gid
}

resource "decort_resgroup" "decort_resource_group" {
  name = local.resource_group_name
  account_id = local.account_id
  gid = local.gid
  def_net_type = "NONE"
}
