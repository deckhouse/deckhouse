resource "aws_vpc" "kube" {
  count = var.existing_vpc_id == "" ? 1 : 0

  cidr_block = var.cidr_block
  enable_dns_hostnames = true

  tags = {
    Name = var.prefix
  }
}

data "aws_vpc" "kube" {
  count = var.existing_vpc_id == "" ? 0 : 1

  id = var.existing_vpc_id
}
