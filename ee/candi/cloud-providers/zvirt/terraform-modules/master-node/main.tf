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

  cpu_sockets = local.master_cpus
  cpu_cores   = 1
  cpu_threads = 1

  memory         = local.master_ram_mb * 1024 * 1024
  maximum_memory = local.master_ram_mb * 1024 * 1024
  memory_ballooning = false

  vm_type = local.master_vm_type

  initialization_custom_script = local.master_cloud_init_script
}

data "ovirt_disk_attachments" "master-vm-boot-disk-attachment" {
  vm_id = ovirt_vm.master_vm.id
}

resource "ovirt_disk_resize" "master_boot_disk_resize" {
  disk_id = tolist(data.ovirt_disk_attachments.master-vm-boot-disk-attachment.attachments)[0].disk_id
  size    = local.master_root_disk_size
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
  disk_interface = "virtio_scsi"
  vm_id          = ovirt_vm.master_vm.id
  bootable       = false
  active         = true
}

resource "ovirt_vm_start" "master_vm" {
  vm_id      = ovirt_vm.master_vm.id
  #stop_behavior = "stop"
  force_stop = true

  depends_on = [ovirt_nic.master_vm_nic, ovirt_disk.master-kubernetes-data, ovirt_disk_attachment.master-kubernetes-data-attachment, ovirt_disk_resize.master_boot_disk_resize]
}

