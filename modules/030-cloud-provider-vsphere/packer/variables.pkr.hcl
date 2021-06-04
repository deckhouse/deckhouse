variable "vcenter_server" {
  description = "vCenter server to build the VM on"
}
variable "vcenter_username" {
  description = "Username to authenticate to vCenter"
}
variable "vcenter_password" {
  description = "Password to authenticate to vCenter"
  default     = ""
}
variable "vcenter_cluster" {}
variable "vcenter_datacenter" {}
variable "vcenter_resource_pool" {}
variable "vcenter_datastore" {}
variable "vcenter_folder" {
  description = "The vcenter folder to store the template"
}
variable "connection_username" {
  default = "ubuntu"
}
variable "connection_password" {
  default = "ubuntu"
}
variable "vm_hardware_version" {
  default = "15"
}
variable "iso_checksum" {}
variable "os_version" {}
variable "os_codename" {}
variable "guest_os_type" {}
variable "root_disk_size" {
  default = 48000
}
variable "nic_type" {
  default = "vmxnet3"
}
variable "vm_network" { }
variable "num_cpu" {
  default = 4
}
variable "num_cores" {
  default = 1
}
variable "vm_ram" {
  default = 4096
}
variable "os_family" {
  description = "OS Family builds the paths needed for packer"
  default = ""
}
variable "os_iso_url" {
  description = "The download url for the ISO"
  default = ""
}
variable "boot_command" {}
