output "id" {
  value = lookup(azurerm_linux_virtual_machine.master, "id")
}

output "node_internal_ip_address" {
  value = lookup(azurerm_linux_virtual_machine.master, "private_ip_address")
}

output "master_ip_address_for_ssh" {
  value = local.enable_external_ip == false ? lookup(azurerm_linux_virtual_machine.master, "private_ip_address") : lookup(azurerm_linux_virtual_machine.master, "public_ip_address")
}

output "kubernetes_data_device_path" {
  value = azurerm_virtual_machine_data_disk_attachment.kubernetes_data.lun
}
