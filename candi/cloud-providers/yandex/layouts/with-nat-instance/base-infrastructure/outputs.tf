output "cloud_discovery_data" {
  value = {
    "region" = "ru-central1"
    "routeTableID" = module.vpc_components.route_table_id
    "defaultLbTargetGroupNetworkId" = local.network_id
    "internalNetworkIDs" = [local.network_id]
    "zones" = keys(module.vpc_components.zone_to_subnet_id_map)
    "zoneToSubnetIdMap" = module.vpc_components.zone_to_subnet_id_map
    "shouldAssignPublicIPAddress" = false
  }
}
