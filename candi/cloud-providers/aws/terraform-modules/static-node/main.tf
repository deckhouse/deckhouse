# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

data "aws_availability_zones" "available" {}
locals {
  az_count = length(data.aws_availability_zones.available.names)
}

data "aws_subnet" "kube" {
  count = local.az_count
  tags = {
    Name = "${var.prefix}-${var.associate_public_ip_address ? "public" : "internal" }-${count.index}"
  }
}
locals {
  zone_to_subnet_id_map = {
    for subnet in data.aws_subnet.kube:
       subnet.availability_zone => subnet.id
  }
  zone = element(local.zones, var.node_index)
}

data "aws_security_group" "node" {
  name = "${var.prefix}-node"
}

resource "aws_instance" "node" {
  ami             = var.node_group.instanceClass.ami
  instance_type   = var.node_group.instanceClass.instanceType
  key_name        = var.prefix
  subnet_id       = local.zone_to_subnet_id_map[local.zone]
  vpc_security_group_ids = concat([data.aws_security_group.node.id], var.additional_security_groups)
  source_dest_check = false
  associate_public_ip_address = var.associate_public_ip_address
  user_data = var.cloud_config == "" ? "" : base64decode(var.cloud_config)
  iam_instance_profile = "${var.prefix}-node"

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = var.root_volume_type
  }

  tags = merge(var.tags, {
    Name = "${var.prefix}-${var.node_group.name}-${var.node_index}"
    "kubernetes.io/cluster/${var.cluster_uuid}" = "shared"
    "kubernetes.io/cluster/${var.prefix}" = "shared"
  })

  lifecycle {
    ignore_changes = [
      user_data,
      ebs_optimized
    ]
  }
}

resource "aws_eip" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  vpc = true
  tags = merge(var.tags, {
    Name = "${var.prefix}-${var.node_group.name}-${var.node_index}"
  })
}

resource "aws_eip_association" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  instance_id = aws_instance.node.id
  allocation_id = aws_eip.eip[0].id
}
