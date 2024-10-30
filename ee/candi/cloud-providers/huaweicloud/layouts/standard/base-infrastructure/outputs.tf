# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "cloud_discovery_data" {
  value = {
    "apiVersion"           = "deckhouse.io/v1"
    "kind"                 = "HuaweiCloudDiscoveryData"
    "layout"               = var.providerClusterConfiguration.layout
    "podNetworkMode"       = local.network_security ? "DirectRoutingWithPortSecurityEnabled" : "DirectRouting"
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
      "imageName"      = local.image_name
      "mainNetwork"    = huaweicloud_vpc.vpc.name
      "securityGroups" = module.network_security.security_group_names
    }
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  }
}
