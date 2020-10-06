variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  existing_network_id = lookup(var.providerClusterConfiguration, "existingNetworkID", "")
  node_network_cidr = var.providerClusterConfiguration.nodeNetworkCIDR
  nat_instance_internal_subnet_id = lookup(var.providerClusterConfiguration.withNATInstance, "internalSubnetID", null)
  nat_instance_external_subnet_id = lookup(var.providerClusterConfiguration.withNATInstance, "externalSubnetID", null)
  nat_instance_external_address = lookup(var.providerClusterConfiguration.withNATInstance, "natInstanceExternalAddress", null)

  dhcp_options = lookup(var.providerClusterConfiguration, "dhcpOptions", null)
  dhcp_domain_name = local.dhcp_options != null ? lookup(local.dhcp_options, "domainName", null) : null
  dhcp_domain_name_servers = local.dhcp_options != null ? lookup(local.dhcp_options, "domainNameServers", null) : null

  labels = lookup(var.providerClusterConfiguration, "labels", {})
}
