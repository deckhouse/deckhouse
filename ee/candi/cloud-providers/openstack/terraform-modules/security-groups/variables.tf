variable "layout_security_group_ids" {
  default = []
  type = list(string)
}

variable "layout_security_group_names" {
  default = []
  type = list(string)
}

variable "security_group_names" {
  type = list(string)
}
