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

output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1"
    "kind" = "AWSCloudDiscoveryData"
    "keyName" = local.prefix
    "instances" = {
      "ami": var.providerClusterConfiguration.masterNodeGroup.instanceClass.ami
      "additionalSecurityGroups": module.security-groups.additional_security_groups
      "associatePublicIPAddress": false
      "iamProfileName": "${local.prefix}-node"
    }
    "loadBalancerSecurityGroup" = module.security-groups.load_balancer_security_group
    "zones" = lookup(var.providerClusterConfiguration, "zones", {}) != {} ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names
    "zoneToSubnetIdMap" = {
      for subnet in aws_subnet.kube_internal:
      subnet.availability_zone => subnet.id
    }
  }
}
