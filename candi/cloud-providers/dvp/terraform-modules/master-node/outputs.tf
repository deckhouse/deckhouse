data "kubernetes_resource" "vm_data" {
  api_version = local.apiVersion
  kind        = "VirtualMachine"

  metadata {
    name      = local.vm_name
    namespace = local.namespace
  }
  depends_on = [
    kubernetes_manifest.vm
  ]

}

output "master_ip_address_for_ssh" {
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}

output "node_internal_ip_address" {
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}

output "kubernetes_data_device_path" {
  value = "/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_vmd-${local.etc_disk_name}"
}
