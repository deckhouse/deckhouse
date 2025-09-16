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

module "vpc" {
  source          = "../../../terraform-modules/vpc"
  prefix          = local.prefix
  existing_vpc_id = local.existing_vpc_id
  cidr_block      = local.vpc_network_cidr
  tags            = local.tags
}

module "security-groups" {
  source       = "../../../terraform-modules/security-groups"
  prefix       = local.prefix
  cluster_uuid = var.clusterUUID
  vpc_id = module.vpc.id
  tags = local.tags
  ssh_allow_list = local.ssh_allow_list
  disable_default_security_group = local.disable_default_sg
}

data "aws_availability_zones" "available" {}

locals {
  az_count    = length(data.aws_availability_zones.available.names)
  subnet_cidr = lookup(var.providerClusterConfiguration, "nodeNetworkCIDR", module.vpc.cidr_block)
}

resource "aws_subnet" "kube_public" {
  count                   = local.az_count
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(local.subnet_cidr, ceil(log(local.az_count, 2)), count.index)
  vpc_id                  = module.vpc.id
  map_public_ip_on_launch = true

  tags = merge(local.tags, {
    Name                                       = "${local.prefix}-public-${count.index}"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}"    = "shared"
    "kubernetes.io/role/elb"                   = "1"
  })
}

resource "aws_internet_gateway" "kube" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_route_table" "kube_public" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name                                       = "${local.prefix}-public"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}"    = "shared"
  })
}

resource "aws_route" "internet_access_public" {
  route_table_id         = aws_route_table.kube_public.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = aws_internet_gateway.kube.id
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
  role = lookup(var.providerClusterConfiguration,"iamNodeRole", aws_iam_role.node.id)
}

resource "aws_key_pair" "ssh" {
  key_name   = local.prefix
  public_key = var.providerClusterConfiguration.sshPublicKey

  tags = merge(local.tags, {
    Cluster = local.prefix
  })
}

// vpc peering
locals {
  peer_vpc_ids = lookup(var.providerClusterConfiguration, "peeredVPCs", [])
}

module "vpc-peering" {
  count                  = length(local.peer_vpc_ids) == 0 ? 0 : 1
  source                 = "../../../terraform-modules/vpc-peering"
  prefix                 = local.prefix
  tags                   = local.tags
  vpc_id                 = module.vpc.id
  peer_vpc_ids           = local.peer_vpc_ids
  region                 = var.providerClusterConfiguration.provider.region
  route_table_id         = aws_route_table.kube_public.id
  destination_cidr_block = module.vpc.cidr_block
}
