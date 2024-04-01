# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "ovirt_templates" "node_template" {
  name          = local.template_name
  fail_on_empty = true
}

resource "ovirt_vm" "node_vm" {
  name        = local.node_name
  cluster_id  = local.cluster_id
  template_id = tolist(data.ovirt_templates.node_template.templates)[0].id
  clone = true

  cpu_sockets = 1
  cpu_cores   = local.cpus
  cpu_threads = 1

  memory         = local.ram_mb * 1024 * 1024
  maximum_memory = local.ram_mb * 1024 * 1024
  memory_ballooning = false

  vm_type = local.vm_type

  initialization_custom_script = local.cloud_init_script

  lifecycle {
    ignore_changes = [
      os_type,
      initialization_custom_script,
      placement_policy_affinity,
      placement_policy_host_ids
    ]
  }
}

data "ovirt_disk_attachments" "node-vm-boot-disk-attachment" {
  vm_id = ovirt_vm.node_vm.id
}

resource "ovirt_disk_resize" "node_boot_disk_resize" {
  disk_id = tolist(data.ovirt_disk_attachments.node-vm-boot-disk-attachment.attachments)[0].disk_id
  size    = local.root_disk_size

  lifecycle {
    ignore_changes = [disk_id]
  }
}

resource "ovirt_nic" "node_vm_nic" {
  name            = local.nic_name
  vm_id           = ovirt_vm.node_vm.id
  vnic_profile_id = local.vnic_profile_id
}

resource "ovirt_vm_start" "node_vm" {
  vm_id      = ovirt_vm.node_vm.id
  #stop_behavior = "stop"
  force_stop = true

  depends_on = [ovirt_nic.node_vm_nic, ovirt_disk_resize.node_boot_disk_resize]
}
