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

data "aws_availability_zone" "node_az" {
  name = aws_instance.node.availability_zone
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
  count = var.disable_default_security_group ? 0 : 1
  name = "${var.prefix}-node"
}

resource "aws_instance" "node" {
  ami             = var.node_group.instanceClass.ami
  instance_type   = var.node_group.instanceClass.instanceType
  key_name        = var.prefix
  subnet_id       = local.zone_to_subnet_id_map[local.zone]
  vpc_security_group_ids = var.disable_default_security_group ? var.additional_security_groups : concat([data.aws_security_group.node[0].id], var.additional_security_groups)
  source_dest_check = false
  associate_public_ip_address = var.associate_public_ip_address
  user_data = var.cloud_config == "" ? "" : base64decode(var.cloud_config)
  iam_instance_profile = "${var.prefix}-node"

  root_block_device {
    volume_size = var.root_volume_size
    volume_type = var.root_volume_type
    tags        = var.tags
  }

  tags = merge(var.tags, {
    Name = "${var.prefix}-${var.node_group.name}-${var.node_index}"
    "kubernetes.io/cluster/${var.cluster_uuid}" = "shared"
    "kubernetes.io/cluster/${var.prefix}" = "shared"
  })

  lifecycle {
    ignore_changes = [
      # user_data in our case is node bootstrap.sh template, which depends on kubernetes version, registry, etc. If we do not suppress
      # user_data, the state of the terraform will change when we change the cluster parameters.
      # how aws calculates user_data:
      # root@kube-master-1:~# curl 169.254.169.254/latest/user-data 2>/dev/null | shasum
      # 3539ff5cb43fb326f4faa6fa5d5aeb9dec1ea141
      user_data,
      # https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/instance#user_data_replace_on_change
      # When used in combination with user_data or user_data_base64 will trigger a destroy and recreate when set to true. Defaults to false if not set.
      # In older versions of terraform-provider-aws this parameter was absent, so now terraform tries to add them.
      # we ignore user_data_replace_on_change to avoid manual converge.
      user_data_replace_on_change,
      ebs_optimized,
      #TODO: remove ignore after we enable automatic converge for master nodes
      volume_tags,
      root_block_device[0].tags_all
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}

resource "aws_eip" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  network_border_group = data.aws_availability_zone.node_az.network_border_group
  domain = "vpc"
  tags = merge(var.tags, {
    Name = "${var.prefix}-${var.node_group.name}-${var.node_index}"
  })
  lifecycle {
    ignore_changes = [network_border_group]
  }
}

resource "aws_eip_association" "eip" {
  count = var.associate_public_ip_address ? 1 : 0
  instance_id = aws_instance.node.id
  allocation_id = aws_eip.eip[0].id
}
