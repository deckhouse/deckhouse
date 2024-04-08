# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "url" {
  description = "zVirt API URL"
}
variable "username" {
  description = "zVirt Admin user"
}
variable "password" {
  description = "zVirt Admin password"
}
variable "insecure_mode" {
  description = "TLS validation"
}

variable "cluster_id" {
  description = "Cluster id"
}

variable "storage_domain_id" {
  description = "UUID of the storage domain used to store VM disks"
}

variable "vnic_profile_id" {
  description = "vNIC profile ID for VMs"
}

variable "nic_name" {
  description = "Network interface name for VM"
  default = "eth0"
}

variable "template_name" {
  description = "VM template name"
}

variable "instance_cpu_cores" {
  description = "VM instance CPU cores count"
  default     = 4
}

variable "instance_memory" {
  description = "VM instance RAM amount in GB"
  default     = 8 # Gb
}

variable "instance_vm_type" {
  description = "VM type"
  default = "high_performance"
}

