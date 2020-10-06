variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition = cidrsubnet(var.providerClusterConfiguration.nodeNetworkCIDR, 0, 0) == var.providerClusterConfiguration.nodeNetworkCIDR
    error_message = "Invalid nodeNetworkCIDR in YandexClusterConfiguration."
  }
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

  dhcp_options = lookup(var.providerClusterConfiguration, "dhcpOptions", null)
  dhcp_domain_name = local.dhcp_options != null ? lookup(local.dhcp_options, "domainName", null) : null
  dhcp_domain_name_servers = local.dhcp_options != null ? lookup(local.dhcp_options, "domainNameServers", null) : null

  labels = lookup(var.providerClusterConfiguration, "labels", {})
}
