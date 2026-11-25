---
title: Network configuration in VMware Cloud Director
permalink: en/admin/integrations/virtualization/vcd/network.html
lang: en
---

## Network configuration for WithNAT

When using the [WithNAT layout](layout.html#withnat), you must confirm the type of network virtualization platform with your administrator and specify it in the [`edgeGateway.type`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-type) parameter. Two options are supported: `NSX-T` and `NSX-V`.

To ensure administrative access to the cluster nodes, a bastion is deployed. The parameters for its configuration are described in [the `bastion` section](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-bastion).

### DHCP

If the Edge Gateway is based on `NSX-T`, a DHCP server will be automatically enabled for the created node network. It will assign IP addresses starting from the 30th address in the subnet up to the penultimate one (before the broadcast address). The starting address of the DHCP pool can be changed using the [`internalNetworkDHCPPoolStartAddress`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-internalnetworkdhcppoolstartaddress) parameter.

If `NSX-V` is used, DHCP must be configured manually. Otherwise, nodes waiting to obtain an IP address via DHCP will not be able to receive one.

{% alert level="warning" %}
It is not recommended to use dynamic addressing for the first [master node](../../../../glossary.html#master-node) together with `NSX-V`.
{% endalert %}

### NAT rules

This layout assumes the automatic creation of the following NAT rules:

- **SNAT** — translation of internal node network addresses to the external address specified in [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip).
- **DNAT** — translating the external address and port, specified in the [`edgeGateway.externalIP`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalip) and [`edgeGateway.externalPort`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-externalport) properties, respectively, to the internal address of the bastion instance on port 22 using the TCP protocol for administrative access to the nodes via SSH.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-V`, you must specify the name and type of the network to which the rule will be bound in the [`edgeGateway.NSX-V.externalNetworkName`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworkname) and [`edgeGateway.NSX-V.externalNetworkType`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-edgegateway-nsx-v-externalnetworktype) properties. Usually, this is the network connected to the Edge Gateway in the `Gateway Interfaces` section and having an external IP address.
{% endalert %}

### Firewall rules

It is also possible to create firewall rules using the [`createDefaultFirewallRules`](/modules/cloud-provider-vcd/cluster_configuration.html#vcdclusterconfiguration-createdefaultfirewallrules) property.

{% alert level="warning" %}
If the Edge Gateway is powered by `NSX-T`, existing Edge Gateway rules will be overwritten. This option is intended for scenarios where only one cluster is deployed on the Edge Gateway.
{% endalert %}

The following rules will be created:

- Allow all outgoing traffic;
- Allow incoming TCP traffic on port 22 for SSH connections to cluster nodes;
- Allow all incoming ICMP traffic;
- Allow incoming TCP and UDP traffic on ports 30000–32767 for `NodePort` services.
