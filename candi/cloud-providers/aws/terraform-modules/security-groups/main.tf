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

resource "aws_security_group" "node" {
  name        = "${var.prefix}-node"
  vpc_id      = var.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.tags, {
    "kubernetes.io/cluster/${var.cluster_uuid}" = "shared"
    "kubernetes.io/cluster/${var.prefix}" = "shared"
  })
}

resource "aws_security_group_rule" "lb-to-node" {
  type                     = "ingress"
  protocol                 = "-1"
  from_port                = 0
  to_port                  = 65535
  security_group_id        = aws_security_group.node.id
  source_security_group_id = aws_security_group.loadbalancer.id
}

resource "aws_security_group_rule" "node-to-node" {
  type                     = "ingress"
  protocol                 = "-1"
  from_port                = 0
  to_port                  = 65535
  security_group_id        = aws_security_group.node.id
  source_security_group_id = aws_security_group.node.id
}

resource "aws_security_group_rule" "to-node-icmp" {
  type = "ingress"
  from_port = -1
  to_port = -1
  protocol = "icmp"
  cidr_blocks = ["0.0.0.0/0"]
  security_group_id = aws_security_group.node.id
}

resource "aws_security_group" "loadbalancer" {
  name = "${var.prefix}-loadbalancer"
  vpc_id = var.vpc_id
  tags = var.tags
}

resource "aws_security_group_rule" "allow-all-incoming-traffic-to-loadbalancer" {
  type                     = "ingress"
  protocol                 = "-1"
  from_port                = 0
  to_port                  = 65535
  security_group_id        = aws_security_group.loadbalancer.id
  cidr_blocks              = ["0.0.0.0/0"]
}

resource "aws_security_group_rule" "allow-all-outgoing-traffic-to-nodes" {
  type                     = "egress"
  protocol                 = "-1"
  from_port                = 0
  to_port                  = 65535
  security_group_id        = aws_security_group.loadbalancer.id
  source_security_group_id = aws_security_group.node.id
}

resource "aws_security_group" "ssh-accessible" {
  name        = "${var.prefix}-ssh-accessible"
  vpc_id      = var.vpc_id
  tags        = var.tags
}

resource "aws_security_group_rule" "allow-ssh-for-everyone" {
  type = "ingress"
  from_port = 22
  to_port = 22
  protocol = "tcp"
  cidr_blocks = ["0.0.0.0/0"]
  security_group_id = aws_security_group.ssh-accessible.id
}
