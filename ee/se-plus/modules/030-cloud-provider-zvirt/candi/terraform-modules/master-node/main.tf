# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "ovirt_templates" "master_template" {
  name          = local.template_name
  fail_on_empty = true
}

resource "ovirt_vm" "master_vm" {
  name        = local.master_node_name
  cluster_id  = local.cluster_id
  template_id = tolist(data.ovirt_templates.master_template.templates)[0].id
  clone = true

  cpu_sockets = 1
  cpu_cores   = local.master_cpus
  cpu_threads = 1

  memory         = local.master_ram_mb * 1024 * 1024
  maximum_memory = local.master_ram_mb * 1024 * 1024
  memory_ballooning = false

  vm_type = local.master_vm_type

  initialization_custom_script = local.master_cloud_init_script
  initialization_dns = local.custom_network_dns

  dynamic "initialization_nic" {
    for_each = local.custom_network_config
    content {
      name = local.custom_network_name
      ipv4 {
        address = local.custom_network_address
        netmask = local.custom_network_netmask
        gateway = local.custom_network_gateway
      }
    }
  }

  lifecycle {
    ignore_changes = [
      os_type,
      initialization_custom_script,
      placement_policy_affinity,
      placement_policy_host_ids
    ]
  }
}

data "ovirt_disk_attachments" "master-vm-boot-disk-attachment" {
  vm_id = ovirt_vm.master_vm.id
}

resource "ovirt_disk_resize" "master_boot_disk_resize" {
  disk_id = tolist(data.ovirt_disk_attachments.master-vm-boot-disk-attachment.attachments)[0].disk_id
  size    = local.master_root_disk_size

  lifecycle {
    # The disk is chosen when the VM is cloned, at the one moment nothing else is
    # attached to it, and frozen here so that a volume attached later cannot take its
    # place. It is chosen again only when there is a new VM to choose it from.
    ignore_changes       = [disk_id]
    replace_triggered_by = [ovirt_vm.master_vm]
  }
}

resource "ovirt_nic" "master_vm_nic" {
  name            = local.master_nic_name
  vm_id           = ovirt_vm.master_vm.id
  vnic_profile_id = local.vnic_profile_id
}

resource "ovirt_disk" "master-kubernetes-data" {
  format            = "raw"
  size              = local.master_etcd_disk_size
  storage_domain_id = local.storage_domain_id
  alias             = join("-", [local.master_node_name, "kubernetes-data"])
  sparse            = false
}

resource "ovirt_disk_attachment" "master-kubernetes-data-attachment" {
  disk_id        = ovirt_disk.master-kubernetes-data.id
  disk_interface = "virtio"
  vm_id          = ovirt_vm.master_vm.id
  bootable       = false
  active         = true

  # Attach the etcd disk only once the boot disk has been resized: the attachments data
  # source is read on the way there, and it must see the disk the VM was cloned with as
  # the only one.
  depends_on = [ovirt_disk_resize.master_boot_disk_resize]

  lifecycle {
    ignore_changes = [disk_interface]
  }
}

resource "ovirt_vm_start" "master_vm" {
  vm_id      = ovirt_vm.master_vm.id
  # Power the VM off rather than asking the guest to shut itself down. Deckhouse holds
  # a block inhibitor on handle-power-key and routes the key through a flow meant for a
  # node still in the cluster, so an ACPI request from the engine is never acted on and
  # the destroy waits for a machine that will not go down. Every other cloud provider
  # ends a VM through its API the same way, and by this point the node has already been
  # drained and taken out of etcd.
  stop_behavior = "stop"
  force_stop    = true

  depends_on = [ovirt_nic.master_vm_nic, ovirt_disk.master-kubernetes-data, ovirt_disk_attachment.master-kubernetes-data-attachment, ovirt_disk_resize.master_boot_disk_resize]
}
