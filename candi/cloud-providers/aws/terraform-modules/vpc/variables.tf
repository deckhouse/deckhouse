variable "prefix" {
  type = string
}

variable "existing_vpc_id" {
  type = string
  default = ""
}

variable "cidr_block" {
  type = string
}

variable "tags" {
  type = map(string)
}
