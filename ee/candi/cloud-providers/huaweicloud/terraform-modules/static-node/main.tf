# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  metadata_tags       = merge(var.tags, var.additional_tags)
  server_group_policy = lookup(var.server_group, "policy", "")
}

data "huaweicloud_vpc_subnet" "subnet" {
  name = var.subnet
}

data "huaweicloud_compute_servergroups" "node" {
  count = local.server_group_policy == "AntiAffinity" ? 1 : 0
  name  = var.prefix
}

resource "huaweicloud_compute_instance" "node" {
  name               = join("-", [var.prefix, var.node_group_name, var.node_index])
  image_name         = var.image_name
  flavor_id          = var.flavor_name
  key_pair           = var.keypair_ssh_name
  user_data          = var.cloud_config == "" ? null : base64decode(var.cloud_config)
  availability_zone  = var.zone
  security_group_ids = var.security_group_ids
  enterprise_project_id = var.enterprise_project_id

  network {
    uuid = data.huaweicloud_vpc_subnet.subnet.id
  }

  system_disk_type = var.volume_type
  system_disk_size = var.root_disk_size

  lifecycle {
    ignore_changes = [
      user_data,
    ]
  }

  metadata = local.metadata_tags

  dynamic "scheduler_hints" {
    for_each = (
      local.server_group_policy == "AntiAffinity" ?
      data.huaweicloud_compute_servergroups.node[0].servergroups :
      []
    )

    content {
      group = scheduler_hints.value["id"]
    }
  }
}

resource "huaweicloud_vpc_eip" "node" {
  count = var.enable_eip == true ? 1 : 0

  publicip {
    type = "5_bgp"
  }

  bandwidth {
    name       = join("-", [var.prefix, "node", var.node_index])
    size       = 100
    share_type = "PER"
  }

  enterprise_project_id = var.enterprise_project_id
}

resource "huaweicloud_compute_eip_associate" "node" {
  count       = var.enable_eip == true ? 1 : 0
  public_ip   = huaweicloud_vpc_eip.node[0].address
  instance_id = huaweicloud_compute_instance.node.id
}
