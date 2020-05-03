variable "prefix" {
  type = string
}

variable "root_disk_size" {
  type = string
}

variable "image_name" {
  type = string
}

variable "flavor_name" {
  type = string
}

variable "keypair_ssh_name" {
  type = string
}

variable "master_internal_port_id" {
  type = string
}

variable "master_external_port_id" {
  type = string
}

variable "config_drive" {
  type = bool
}
