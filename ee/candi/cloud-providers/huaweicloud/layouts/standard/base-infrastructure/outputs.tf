# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1"
    "kind"       = "HuaweiCloudDiscoveryData"
    "layout"     = var.providerClusterConfiguration.layout
    "instances" = {
      "vpcSubnetId"     = huaweicloud_vpc_subnet.subnet.id
      "vpcIPv4SubnetId" = huaweicloud_vpc_subnet.subnet.ipv4_subnet_id
      "securityGroupId" = module.network_security.security_group_id
    }
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  }
}
