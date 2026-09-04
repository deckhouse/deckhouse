# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_account_list" "accounts" {
  # The API filters by substring, so the list may also contain accounts whose
  # name merely includes local.account. Exact matching is done below.
  name = local.account

  lifecycle {
    postcondition {
      condition     = length([for a in self.items : a if a.account_name == local.account]) == 1
      error_message = <<-EOT
        ERROR: expected exactly one Dynamix account named '${local.account}', found ${length([for a in self.items : a if a.account_name == local.account])} exact match(es) among ${length(self.items)} account(s) returned by the name filter.

        Set DynamixClusterConfiguration.account to the exact name of an account the credentials have access to.
      EOT
    }
  }
}

data "decort_locations_list" "locations" {
   name = local.location
}

locals {
  account_id = one([for a in data.decort_account_list.accounts.items : a.account_id if a.account_name == local.account])
  gid = data.decort_locations_list.locations.items[0].gid
}

resource "decort_resgroup" "decort_resource_group" {
  name = local.resource_group_name
  account_id = local.account_id
  gid = local.gid
  def_net_type = "NONE"
}
