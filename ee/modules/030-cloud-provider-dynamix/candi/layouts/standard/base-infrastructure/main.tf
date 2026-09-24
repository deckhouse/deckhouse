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

data "decort_storage_policy_list" "account_storage_policies" {
  # Every ENABLED policy of the account, not just DynamixClusterConfiguration.storagePolicy:
  # the resource group is one per cluster, while a policy is picked per node (in the instance
  # class or cluster-wide) and the CSI driver offers a StorageClass per policy of the account.
  # Both filters are exact on the API side, so the list needs no further narrowing here.
  account_id = local.account_id
  status     = "ENABLED"

  lifecycle {
    postcondition {
      condition     = length([for p in self.items : p if p.name == local.storage_policy]) == 1
      error_message = <<-EOT
        ERROR: expected exactly one ENABLED Dynamix storage policy named '${local.storage_policy}' in account '${local.account}', found ${length([for p in self.items : p if p.name == local.storage_policy])} exact match(es) among ${length(self.items)} ENABLED policy(-ies) of the account.

        Set DynamixClusterConfiguration.storagePolicy to the exact name of an ENABLED storage policy available to the account.
      EOT
    }
  }
}

resource "decort_resgroup" "decort_resource_group" {
  name = local.resource_group_name
  account_id = local.account_id
  gid = local.gid
  def_net_type = "NONE"

  # Destroy settings, read by the provider only when the group is deleted.
  # "force" lets the group go even when something the cluster created outside this state
  # (a load balancer, a CSI volume, a machine the node group did not get to remove) is
  # still inside it, so aborting a half-built cluster does not leave the group behind.
  # "permanently" keeps the deleted group out of the recycle bin, where it would hold on
  # to its name and to the resources it contains.
  force       = true
  permanently = true

  # The set is declarative: a policy the account loses is detached on the next converge,
  # and "limit" is left at the provider default -1, so the module claims no storage quota.
  dynamic "storage_policy" {
    for_each = data.decort_storage_policy_list.account_storage_policies.items
    content {
      id = storage_policy.value.storage_policy_id
    }
  }
}
