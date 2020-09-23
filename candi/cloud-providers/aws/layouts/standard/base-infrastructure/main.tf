module "vpc" {
  source = "../../../terraform-modules/vpc"
  prefix = local.prefix
  existing_vpc_id = local.existing_vpc_id
  cidr_block = local.vpc_network_cidr
  tags = local.tags
}

module "security-groups" {
  source = "../../../terraform-modules/security-groups"
  prefix = local.prefix
  cluster_uuid = var.clusterUUID
  vpc_id = module.vpc.id
  tags = local.tags
}

data "aws_availability_zones" "available" {}

locals {
  az_count = length(data.aws_availability_zones.available.names)
  subnet_cidr = lookup(var.providerClusterConfiguration, "nodeNetworkCIDR", module.vpc.cidr_block)
}

resource "aws_subnet" "kube_public" {
  count                   = local.az_count
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(local.subnet_cidr, ceil(log(local.az_count * 2, 2)), count.index)
  vpc_id                  = module.vpc.id
  map_public_ip_on_launch = true

  tags = merge(local.tags, {
    Name = "${local.prefix}-public-${count.index}"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}" = "shared"
  })
}

resource "aws_subnet" "kube_internal" {
  count                   = local.az_count
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  cidr_block              = cidrsubnet(local.subnet_cidr, ceil(log(local.az_count * 2, 2)), count.index + local.az_count)
  vpc_id                  = module.vpc.id
  map_public_ip_on_launch = false

  tags = merge(local.tags, {
    Name = "${local.prefix}-internal-${count.index}"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}" = "shared"
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
  subnet_id = aws_subnet.kube_public[0].id
  allocation_id = aws_eip.natgw.id

  tags = merge(local.tags, {
    Name = local.prefix
  })
}

resource "aws_route_table" "kube_internal" {
  vpc_id = module.vpc.id

  tags = merge(local.tags, {
    Name = "${local.prefix}-internal"
    "kubernetes.io/cluster/${var.clusterUUID}" = "shared"
    "kubernetes.io/cluster/${local.prefix}" = "shared"
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
  key_name = local.prefix
  public_key = var.providerClusterConfiguration.sshPublicKey

  tags = merge(local.tags, {
    Cluster = local.prefix
  })
}
