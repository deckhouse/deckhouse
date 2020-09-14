output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "AWSCloudDiscoveryData"
    "keyName" = local.prefix
    "instances" = {
      "iamProfileName": "${local.prefix}-node"
      "additionalSecurityGroups": module.security-groups.additional_security_groups
    }
    "loadBalancerSecurityGroup" = module.security-groups.load_balancer_security_group
    "zones" = data.aws_availability_zones.available.names
    "zoneToSubnetIdMap" = {
      for subnet in aws_subnet.kube_internal:
      subnet.availability_zone => subnet.id
    }
  }
}
