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

output "additional_security_groups" {
  value = length(aws_security_group.node) > 0 ? [aws_security_group.node[0].id] : []
}

output "load_balancer_security_group" {
  value = length(aws_security_group.loadbalancer) > 0 ? aws_security_group.loadbalancer[0].id : ""
}

output "security_group_id_node" {
  value = length(aws_security_group.loadbalancer) > 0 ? aws_security_group.node[0].id : ""
}

output "security_group_id_ssh_accessible" {
  value = length(aws_security_group.ssh-accessible) > 0 ? aws_security_group.ssh-accessible[0].id : ""
}
