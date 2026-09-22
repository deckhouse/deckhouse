# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_image_list" "images" {
  name = local.image_name
}

data "decort_rg_list" "resource_group" {
  name = local.resource_group_name
}

data "decort_vins_list" "vins" {
  name = local.vins_name
}

data "decort_extnet_list" "extnets" {
  name = local.extnet_name
}

data "decort_storage_policy_list" "storage_policies" {
  # A policy of another account, visible to the same credentials, would match the name
  # but is not attached to our resource group, so the list is narrowed by the account.
  account_id = local.account_id

  # The resource group carries the ENABLED policies of the account, so a DISABLED one
  # would resolve here and only fail later, when the platform refuses to create the
  # machine in a resource group the policy is not attached to.
  status = "ENABLED"

  # The API filters by substring, so the list may also contain policies whose
  # name merely includes local.storage_policy. Exact matching is done below.
  name = local.storage_policy

  lifecycle {
    postcondition {
      condition     = length([for p in self.items : p if p.name == local.storage_policy]) == 1
      error_message = <<-EOT
        ERROR: expected exactly one ENABLED Dynamix storage policy named '${local.storage_policy}' in the account, found ${length([for p in self.items : p if p.name == local.storage_policy])} exact match(es) among ${length(self.items)} ENABLED policy(-ies) returned by the name filter.

        Set DynamixClusterConfiguration.storagePolicy (or the instanceClass.storagePolicy of node group '${local.node_group_name}') to the exact name of an ENABLED storage policy available to the account.
      EOT
    }
  }
}

locals {
  account_id        = data.decort_rg_list.resource_group.items[0].account_id
  image_id          = data.decort_image_list.images.items[0].image_id
  rg_id             = data.decort_rg_list.resource_group.items[0].rg_id
  extnet_id         = data.decort_extnet_list.extnets.items[0].net_id
  storage_policy_id = one([for p in data.decort_storage_policy_list.storage_policies.items : p.storage_policy_id if p.name == local.storage_policy])
}

resource "decort_kvmvm" "node_vm" {
  name              = local.node_name
  rg_id             = local.rg_id
  cpu               = local.cpus
  ram               = local.ram_mb
  boot_disk_size    = local.root_disk_size
  image_id          = local.image_id
  storage_policy_id = local.storage_policy_id
  cloud_init        = local.cloud_init_script

  dynamic "network" {
    for_each = length(data.decort_vins_list.vins.items) > 0 ? [data.decort_vins_list.vins.items[0].vins_id] : []
    content {
      net_type = local.net_type_vins
      net_id   = network.value
    }
  }
  network {
    net_type = local.net_type_extnet
    net_id   = local.extnet_id
  }

  lifecycle {
    ignore_changes = [
      cloud_init,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}
