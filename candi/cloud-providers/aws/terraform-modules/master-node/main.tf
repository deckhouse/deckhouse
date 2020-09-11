data "aws_availability_zones" "available" {}
locals {
  az_count = length(data.aws_availability_zones.available.names)
}

resource "aws_ebs_volume" "kubernetes_data" {
  size            = 150 # To achieve io rate burst limit 450iops, average io rate for etcd is 300iops
  type            = "gp2"
  tags = {
    Name = "${var.prefix}-kubernetes-data-${var.node_index}"
  }
  availability_zone = data.aws_availability_zones.available.names[var.node_index % local.az_count]
}

resource "aws_volume_attachment" "kubernetes_data" {
  device_name = "/dev/xvdf"
  skip_destroy = true
  volume_id   = aws_ebs_volume.kubernetes_data.id
  instance_id = aws_instance.master.id
}

data "aws_subnet" "kube" {
  tags = {
    Name = "${var.prefix}-${var.associate_public_ip_address ? "public" : "internal" }-${var.node_index % local.az_count}"
  }
}

data "aws_security_group" "ssh-accessible" {
  name = "${var.prefix}-ssh-accessible"
}

data "aws_security_group" "node" {
  name = "${var.prefix}-node"
}

resource "aws_instance" "master" {
  ami             = var.node_group.instanceClass.ami
  instance_type   = var.node_group.instanceClass.instanceType
  key_name        = var.prefix
  subnet_id       = data.aws_subnet.kube.id
  vpc_security_group_ids = concat([data.aws_security_group.node.id, data.aws_security_group.ssh-accessible.id], var.additional_security_groups)
  source_dest_check = false
  user_data = var.cloud_config == "" ? "" : base64decode(var.cloud_config)
  iam_instance_profile = "${var.prefix}-node"

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = var.root_volume_type
  }

  tags = {
    Name = "${var.prefix}-master-${var.node_index}"
    "kubernetes.io/cluster/${var.cluster_uuid}" = "shared"
    "kubernetes.io/cluster/${var.prefix}" = "shared"
  }

  lifecycle {
    ignore_changes = [
      user_data
    ]
  }
}

resource "aws_eip" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  vpc = true
  tags = {
    Name = "${var.prefix}-master-${var.node_index}"
  }
}

resource "aws_eip_association" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  instance_id = aws_instance.master.id
  allocation_id = aws_eip.eip[0].id
}
