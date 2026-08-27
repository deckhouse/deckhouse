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

# `zones` intentionally treats an empty restriction list the same as an absent one
# and falls back to every zone the subnets cover. Before the migration the test was
# `lookup(providerClusterConfiguration, "zones", null) != null`, so an explicit
# `zones: []` intersected down to an empty list. That is the wrong answer twice over:
# an empty list is self-contradictory for a "globally restricted set of zones", and
# node-manager derives its default zone set from this output, so it would end up with
# no zones at all (040-node-manager hooks/get_crds.go). Neither PCC nor ModuleConfig
# constrains the list, so `zones: []` is accepted upstream and has to be handled here.
#
# The discovery-data schemas no longer help: candi/openapi/cloud_discovery_data.yaml
# and openapi/values.yaml used to reject an empty `zones` via `minItems: 1`, but that
# constraint was removed so a cluster whose infrastructure DKP does not create can
# render with no discovery data at all. This output is the only remaining guard.
output "cloud_discovery_data" {
  value = {
    "apiVersion"                    = "deckhouse.io/v1"
    "kind"                          = "YandexCloudDiscoveryData"
    "region"                        = "ru-central1"
    "routeTableID"                  = module.vpc_components.route_table_id
    "defaultLbTargetGroupNetworkId" = local.network_id
    "internalNetworkIDs"            = [local.network_id]
    "zones"                         = length(local.zones) > 0 ? tolist(setintersection(keys(module.vpc_components.zone_to_subnet_id_map), local.zones)) : keys(module.vpc_components.zone_to_subnet_id_map)
    "zoneToSubnetIdMap"             = module.vpc_components.zone_to_subnet_id_map
    "shouldAssignPublicIPAddress"   = true
  }
}
