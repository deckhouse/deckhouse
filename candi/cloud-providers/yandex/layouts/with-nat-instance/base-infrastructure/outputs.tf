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
    "kind" = "YandexCloudDiscoveryData"
    "region" = "ru-central1"
    "routeTableID" = module.vpc_components.route_table_id
    "defaultLbTargetGroupNetworkId" = local.network_id
    "internalNetworkIDs" = [local.network_id]
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(keys(module.vpc_components.zone_to_subnet_id_map), var.providerClusterConfiguration.zones)) : keys(module.vpc_components.zone_to_subnet_id_map)
    "zoneToSubnetIdMap" = module.vpc_components.zone_to_subnet_id_map
    "shouldAssignPublicIPAddress" = false
    "natInstanceName"=module.vpc_components.nat_instance_name
  }
}
