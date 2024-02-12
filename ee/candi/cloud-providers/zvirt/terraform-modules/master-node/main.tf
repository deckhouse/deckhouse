# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  vnic_profile_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vnicProfileId", [])
  storage_domain_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "storageDomainId", [])
  cluster_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "clusterId", [])
  template_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "template", [])
  master_cpus = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "numCPUs", [])
  master_ram_mb = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "memory", [])
  master_os_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "os", [])
  master_vm_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vmType", [])
  master_nic_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "nicName", [])
}

data "ovirt_templates" "master_template" {
  name          = local.template_name
  fail_on_empty = true
}

resource "ovirt_vm" "master_vm" {
  name        = join("-", [local.resource_name_prefix, "master", var.nodeIndex])
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
  os_type = local.master_os_type
}

resource "ovirt_nic" "master_vm_nic" {
  name            = local.master_nic_name
  vm_id           = ovirt_vm.master_vm.id
  vnic_profile_id = local.vnic_profile_id
}

resource "ovirt_disk" "master-kubernetes-data" {
  format            = "raw"
  size              = 15*1024*1024*1024 # 15 GB
  storage_domain_id = local.storage_domain_id
  alias             = join("-", [local.resource_name_prefix, "master", var.nodeIndex, "kubernetes-data"])
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

  depends_on = [ovirt_nic.master_vm_nic, ovirt_disk.master-kubernetes-data, ovirt_disk_attachment.master-kubernetes-data-attachment]
}

