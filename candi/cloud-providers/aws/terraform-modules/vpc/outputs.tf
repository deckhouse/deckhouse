output "id" {
  value = var.existing_vpc_id == "" ? join("", aws_vpc.kube.*.id) : data.aws_vpc.kube.0.id
}

output "cidr_block" {
  value = var.existing_vpc_id == "" ? join("", aws_vpc.kube.*.cidr_block) : data.aws_vpc.kube.0.cidr_block
}
