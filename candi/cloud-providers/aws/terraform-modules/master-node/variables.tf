variable "prefix" {
  type = string
}

variable "cluster_uuid" {
  type = string
}

variable "node_group" {
  type = any
}

variable "node_index" {
  type = number
}

variable "root_volume_size" {
  type = number
}

variable "root_volume_type" {
  type = string
}

variable "associate_public_ip_address" {
  type = bool
}

variable "cloud_config" {
  type = any
}

variable "additional_security_groups" {
  type = list(string)
  default = []
}

variable "zones" {
  type = list(string)
}

locals {
  zones = sort(distinct(var.zones))
}
