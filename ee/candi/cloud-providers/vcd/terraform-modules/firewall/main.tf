# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

# NSX-T resources

resource "vcd_nsxt_ip_set" "internal_network" {
  count           = var.useNSXV ? 0 : 1
  org             = var.providerClusterConfiguration.organization
  edge_gateway_id = var.edgeGatewayId

  name         = var.providerClusterConfiguration.mainNetwork
  description  = format("%s CIDR", var.providerClusterConfiguration.mainNetwork)
  ip_addresses = [var.providerClusterConfiguration.internalNetworkCIDR]
}

data "vcd_nsxt_app_port_profile" "ssh" {
  count = var.useNSXV ? 0 : 1
  org   = var.providerClusterConfiguration.organization

  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_app_port_profile" "node_ports" {
  count       = var.useNSXV ? 0 : 1
  org         = var.providerClusterConfiguration.organization
  description = "Node ports for Kubernetes"

  name  = "NODE PORTS"
  scope = "TENANT"

  app_port {
    protocol = "TCP"
    port     = ["30000-32767"]
  }
}

resource "vcd_nsxt_firewall" "firewall" {
  count           = var.useNSXV ? 0 : 1
  org             = var.providerClusterConfiguration.organization
  edge_gateway_id = var.edgeGatewayId

  rule {
    enabled              = true
    action               = "ALLOW"
    name                 = format("%s-inbound-ssh", var.providerClusterConfiguration.mainNetwork)
    direction            = "IN"
    ip_protocol          = "IPV4"
    source_ids           = []
    destination_ids      = vcd_nsxt_ip_set.internal_network[0].id
    app_port_profile_ids = [data.vcd_nsxt_app_port_profile.ssh[0].id]
  }

  rule {
    enabled         = true
    action          = "ALLOW"
    name            = format("%s-inbound-icmp", var.providerClusterConfiguration.mainNetwork)
    direction       = "IN"
    ip_protocol     = "ICMP"
    source_ids      = []
    destination_ids = vcd_nsxt_ip_set.internal_network[0].id
  }

  rule {
    enabled              = true
    action               = "ALLOW"
    name                 = format("%s-inbound-node-ports", var.providerClusterConfiguration.mainNetwork)
    direction            = "IN"
    ip_protocol          = "IPV4"
    source_ids           = []
    destination_ids      = vcd_nsxt_ip_set.internal_network[0].id
    app_port_profile_ids = [vcd_nsxt_app_port_profile.node_ports[0].id]
  }

  rule {
    enabled         = true
    action          = "ALLOW"
    name            = format("%s-outbound-any", var.providerClusterConfiguration.mainNetwork)
    direction       = "OUT"
    ip_protocol     = "IPV4"
    source_ids      = vcd_nsxt_ip_set.internal_network[0].id
    destination_ids = []
  }
}

# NSX-V resources

resource "vcd_nsxv_firewall_rule" "ssh" {
  count        = var.useNSXV ? 1 : 0
  org          = var.providerClusterConfiguration.organization
  edge_gateway = var.providerClusterConfiguration.edgeGateway.name

  name = format("%s-inbound-ssh", var.providerClusterConfiguration.mainNetwork)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.providerClusterConfiguration.internalNetworkCIDR]
  }

  service {
    protocol = "tcp"
    port     = "22"
  }
}

resource "vcd_nsxv_firewall_rule" "icmp" {
  count        = var.useNSXV ? 1 : 0
  org          = var.providerClusterConfiguration.organization
  edge_gateway = var.providerClusterConfiguration.edgeGateway.name

  name = format("%s-inbound-icmp", var.providerClusterConfiguration.mainNetwork)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.providerClusterConfiguration.internalNetworkCIDR]
  }

  service {
    protocol = "icmp"
  }
}

resource "vcd_nsxv_firewall_rule" "node_ports" {
  count        = var.useNSXV ? 1 : 0
  org          = var.providerClusterConfiguration.organization
  edge_gateway = var.providerClusterConfiguration.edgeGateway.name

  name = format("%s-inbound-node-ports", var.providerClusterConfiguration.mainNetwork)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.providerClusterConfiguration.internalNetworkCIDR]
  }

  service {
    protocol = "tcp"
    port     = "30000-32767"
  }
}

resource "vcd_nsxv_firewall_rule" "outbound_any" {
  count        = var.useNSXV ? 1 : 0
  org          = var.providerClusterConfiguration.organization
  edge_gateway = var.providerClusterConfiguration.edgeGateway.name

  name = format("%s-outbound-any", var.providerClusterConfiguration.mainNetwork)

  source {
    ip_addresses = [var.providerClusterConfiguration.internalNetworkCIDR]
  }

  destination {
    ip_addresses = ["any"]
  }

  service {
    protocol = "any"
  }
}
