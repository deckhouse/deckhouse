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

data "aws_caller_identity" "kube" {}

resource "aws_vpc_peering_connection" "kube" {
  count         = length(var.peer_vpc_ids)
  vpc_id        = var.vpc_id
  peer_vpc_id   = var.peer_vpc_ids[count.index]
  peer_owner_id = data.aws_caller_identity.kube.account_id // peer_owner_id and our local account_id are equal cause we only support peering within single account
  peer_region   = var.region
  auto_accept   = false

  tags = merge(var.tags, {
    Name = var.prefix
  })
}

resource "aws_vpc_peering_connection_accepter" "kube" {
  count                     = length(var.peer_vpc_ids)
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
  auto_accept               = true

  tags = merge(var.tags, {
    Name = var.prefix
  })
}

resource "aws_route" "kube" {
  count                     = length(var.peer_vpc_ids)
  route_table_id            = var.route_table_id
  destination_cidr_block    = data.aws_vpc.target[count.index].cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
}

data "aws_vpc" "target" {
  count = length(var.peer_vpc_ids)
  id    = var.peer_vpc_ids[count.index]
}

data "aws_subnets" "target" {
  count = length(var.peer_vpc_ids)
  filter {
    name   = "vpc-id"
    values = [var.peer_vpc_ids[count.index]]
  }
}

data "aws_route_table" "target" {
  count     = length(var.peer_vpc_ids)
  subnet_id = data.aws_subnets.target[count.index].ids[0]
}

resource "aws_route" "target" {
  count                     = length(var.peer_vpc_ids)
  route_table_id            = data.aws_route_table.target[count.index].id
  destination_cidr_block    = var.destination_cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.kube[count.index].id
}
