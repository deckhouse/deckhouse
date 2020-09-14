variable "prefix" {
  type = string
}

variable "dhcp_domain_name" {
  type = string
  default = null
}

variable "dhcp_domain_name_servers" {
  type = list(string)
  default = null
}

variable "network_id" {
  type = string
}

variable "node_network_cidr" {
  type = string
}

variable "should_create_nat_instance" {
  type = bool
  default = false
}

variable "nat_instance_external_address" {
  type = string
  default = null
}

variable "nat_instance_internal_subnet_id" {
  type = string
  default = null
}

variable "nat_instance_external_subnet_id" {
  type = string
  default = null
}

variable "nat_instance_ssh_key" {
  type = string
  default = ""
}
