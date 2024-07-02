# Copyright 2021 Flant CJSC
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

module "vpc" {
  source          = "../../../terraform-modules/vpc"
  prefix          = local.prefix
  existing_vpc_id = local.existing_vpc_id
  cidr_block      = local.vpc_network_cidr
  tags            = local.tags
}

module "security-groups" {
  source = "../../../terraform-modules/security-groups"
  prefix = local.prefix
  cluster_uuid = var.clusterUUID
  vpc_id = module.vpc.id
  tags = local.tags
  ssh_allow_list = local.ssh_allow_list
}

data "aws_availability_zones" "available" {}

locals {
  az_count    = length(data.aws_availability_zones.available.names)
  subnet_cidr = lookup(var.providerClusterConfiguration, "nodeNetworkCIDR", module.vpc.cidr_block)
}

resource "aws_subnet" "kube_public" {
  count                   = local.az_count
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(local.subnet_cidr, ceil(log(local.az_count * 2, 2)), count.index)
  vpc_id                  = module.vpc.id
  map_public_ip_on_launch = true

  tags = merge(local.tags, {
    Name                                       = "${local.prefix}-public-${count.index}"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}"    = "shared"
  })
}

resource "aws_subnet" "kube_internal" {
  count                   = local.az_count
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(local.subnet_cidr, ceil(log(local.az_count * 2, 2)), count.index + local.az_count)
  vpc_id                  = module.vpc.id
  map_public_ip_on_launch = false

  tags = merge(local.tags, {
    Name                                       = "${local.prefix}-internal-${count.index}"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}"    = "shared"
  })
}

resource "aws_eip" "natgw" {
  vpc = true

  tags = merge(local.tags, {
    Name = "${local.prefix}-natgw"
  })
}

resource "aws_internet_gateway" "kube" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_nat_gateway" "kube" {
  subnet_id     = aws_subnet.kube_public[0].id
  allocation_id = aws_eip.natgw.id

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_route_table" "kube_internal" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name                                       = "${local.prefix}-internal"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}"    = "shared"
  })
}

resource "aws_route_table" "kube_public" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name = "${local.prefix}-public"
  })
}

resource "aws_route" "internet_access_internal" {
  route_table_id         = aws_route_table.kube_internal.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.kube.id
}

resource "aws_route" "internet_access_public" {
  route_table_id         = aws_route_table.kube_public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.kube.id
}

resource "aws_route_table_association" "kube_internal" {
  count          = local.az_count
  subnet_id      = aws_subnet.kube_internal[count.index].id
  route_table_id = aws_route_table.kube_internal.id
}

resource "aws_route_table_association" "kube_public" {
  count          = local.az_count
  subnet_id      = aws_subnet.kube_public[count.index].id
  route_table_id = aws_route_table.kube_public.id
}

resource "aws_iam_role" "node" {
  name = "${local.prefix}-node"

  assume_role_policy = <<-EOF
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Action": "sts:AssumeRole",
        "Principal": {
          "Service": "ec2.amazonaws.com"
        },
        "Effect": "Allow"
      }
    ]
  }
  EOF

  tags = local.tags
}

resource "aws_iam_role_policy" "node" {
  name = "${local.prefix}-node"
  role = aws_iam_role.node.id

  policy = <<-EOF
  {
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          %{for policy in local.additional_role_policies}
          "${policy}",
          %{endfor}
          "ec2:DescribeTags",
          "ec2:DescribeInstances"
        ],
        "Resource": [
          "*"
        ]
      }
    ]
  }
  EOF
}

resource "aws_iam_instance_profile" "node" {
  name = "${local.prefix}-node"
  role = aws_iam_role.node.id
}

resource "aws_key_pair" "ssh" {
  key_name   = local.prefix
  public_key = var.providerClusterConfiguration.sshPublicKey

  tags = merge(local.tags, {
    Cluster = local.prefix
  })
}

// bastion

locals {
  instance_class             = lookup(local.bastion_instance, "instanceClass", {})
  additional_security_groups = lookup(local.instance_class, "additionalSecurityGroups", [])

  actual_zones = lookup(var.providerClusterConfiguration, "zones", {}) != {} ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names

  zone = lookup(local.bastion_instance, "zone", {}) != {} ? local.bastion_instance.zone : local.actual_zones[0]

  zone_to_subnet_id_map = {
    for subnet in aws_subnet.kube_public :
    subnet.availability_zone => subnet.id
  }

  subnet_id = local.zone_to_subnet_id_map[local.zone]

  root_volume_size = lookup(local.instance_class, "diskSizeGb", 20)
  root_volume_type = lookup(local.instance_class, "diskType", "gp2")
}

resource "aws_instance" "bastion" {
  count                  = local.bastion_instance != {} ? 1 : 0
  ami                    = local.instance_class.ami
  instance_type          = local.instance_class.instanceType
  key_name               = local.prefix
  subnet_id              = local.subnet_id
  vpc_security_group_ids = concat([module.security-groups.security_group_id_node, module.security-groups.security_group_id_ssh_accessible], local.additional_security_groups)
  source_dest_check      = false
  iam_instance_profile   = aws_iam_instance_profile.node.name

  root_block_device {
    volume_size = local.root_volume_size
    volume_type = local.root_volume_type
  }

  tags = merge(local.tags, {
    Name = "${local.prefix}-bastion"
  })

  lifecycle {
    ignore_changes = [
      ebs_optimized
    ]
  }

  depends_on = [
    module.vpc,
    module.security-groups,
    aws_iam_instance_profile.node
  ]
}


resource "aws_eip" "bastion" {
  count = local.bastion_instance != {} ? 1 : 0
  vpc   = true
  tags = merge(local.tags, {
    Name = "${local.prefix}-bastion"
  })
}

resource "aws_eip_association" "bastion" {
  count         = local.bastion_instance != {} ? 1 : 0
  instance_id   = aws_instance.bastion[0].id
  allocation_id = aws_eip.bastion[0].id
}

// vpc peering

locals {
  peer_vpc_ids = lookup(var.providerClusterConfiguration, "peeredVPCs", [])
}

data "aws_caller_identity" "kube" {}

resource "aws_vpc_peering_connection" "kube" {
  count         = length(local.peer_vpc_ids)
  vpc_id        = module.vpc.id
  peer_vpc_id   = local.peer_vpc_ids[count.index]
  peer_owner_id = data.aws_caller_identity.kube.account_id
  peer_region   = var.providerClusterConfiguration.provider.region
  auto_accept   = false

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_vpc_peering_connection_accepter" "kube" {
  count                     = length(local.peer_vpc_ids)
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
  auto_accept               = true

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_route" "kube" {
  count                     = length(local.peer_vpc_ids)
  route_table_id            = aws_route_table.kube_internal.id
  destination_cidr_block    = data.aws_vpc.target[count.index].cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
  depends_on                = [aws_route_table.kube_internal]
}

data "aws_vpc" "target" {
  count = length(local.peer_vpc_ids)
  id    = local.peer_vpc_ids[count.index]
}

data "aws_subnets" "target" {
  count = length(local.peer_vpc_ids)
  filter {
    name   = "vpc-id"
    values = [local.peer_vpc_ids[count.index]]
  }
}

data "aws_route_table" "target" {
  count     = length(local.peer_vpc_ids)
  subnet_id = data.aws_subnets.target[count.index].ids[0]
}

resource "aws_route" "target" {
  count                     = length(local.peer_vpc_ids)
  route_table_id            = data.aws_route_table.target[count.index].id
  destination_cidr_block    = module.vpc.cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
  depends_on                = [aws_route_table.kube_internal]
}
