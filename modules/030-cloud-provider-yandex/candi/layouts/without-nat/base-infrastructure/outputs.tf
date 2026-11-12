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
# and falls back to every zone the subnets cover. The migration module resolves an
# absent, null or empty list to the default zone set for every configured cluster,
# so the fallback below is reachable only in unconfigured or destroy runs, where
# node-manager must still not end up with an empty zone set
# (040-node-manager hooks/get_crds.go).
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
    "zones" = (
      # An adopted map without an explicit selection keeps the discovery list on
      # the static zones, the way it behaved before dynamic zones existed:
      # node-manager derives NodeGroup default zones from this list, and adopted
      # zones outside the static set must not become placement defaults.
      length(local.existing_zone_to_subnet_id_map) > 0 && length(local.raw_zones) == 0
      ? (length(local.adopted_static_discovery_zones) > 0 ? local.adopted_static_discovery_zones : keys(module.vpc_components.zone_to_subnet_id_map))
      : (length(local.zones) > 0 ? tolist(setintersection(keys(module.vpc_components.zone_to_subnet_id_map), local.zones)) : keys(module.vpc_components.zone_to_subnet_id_map))
    )
    "zoneToSubnetIdMap"           = module.vpc_components.zone_to_subnet_id_map
    "shouldAssignPublicIPAddress" = true
  }
}
