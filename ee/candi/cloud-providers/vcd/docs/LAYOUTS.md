---
title: "Cloud provider - VMware Cloud Director: Layouts"
description: "Schemes of placement and interaction of resources in VMware Cloud Director when working with the Deckhouse cloud provider."
---

## Standard

![Standard layout](../../images/cloud-provider-vcd/vcd-standard.png)
<!--- Source: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11247&t=IvETjbByf1MSQzcm-0 --->

Example of the layout configuration:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

## StandardWithNetwork

When choosing this deployment scheme, you must ask your administrator for the type of network virtualization platform and specify it in the `edgeGateway.type` property. The deployment scheme supports `NSX-T` and `NSX-V`.

If the Edge Gateway is based on NSX-T, a DHCP server will be automatically enabled in the created network for the nodes, providing IP addresses starting from the 30th address in the network up to the penultimate address (just before the broadcast address). The starting address for the IP address pool range can be configured via the `internalNetworkDHCPPoolStartAddress` property. If the Edge Gateway is provided by `NSX-V`, you must manually configure DHCP for the planned node network. Otherwise, nodes that are expected to obtain an address dynamically will not be able to do so.

{% alert level="warning" %}
It is not recommended to use dynamic addressing for the first master node in combination with `NSX-V`.
{% endalert %}

The deployment scheme assumes automated creation of NAT rules:
- An SNAT rule for translating the addresses of the internal node network to the external address specified in the `edgeGateway.externalIP` property.
- A DNAT rule for translating the external address and port, specified in the `edgeGateway.externalIP` and `edgeGateway.externalPort` properties, respectively, to the internal address of the first master node on port 22 using the `TCP` protocol for administrative access to the nodes via SSH.

{% alert level="warning" %}
If the Edge Gateway is provided by `NSX-V`, you must specify the name and type of the network to which the rule will be bound in the `edgeGateway.NSX-V.externalNetworkName` and `edgeGateway.NSX-V.externalNetworkType` properties, respectively. Typically, this is a network connected to the Edge Gateway in `Gateway Interface` and having an external IP address.
{% endalert %}

Additionally, you can enable the creation of default firewall rules using the `createDefaultFirewallRules` property.

{% alert level="warning" %}
If the Edge Gateway is provided by `NSX-T`, existing rules on the Edge Gateway will be overwritten. It is assumed that using this option implies that only one cluster will be deployed per Edge Gateway.
{% endalert %}

The following rules will be created:
- Allow any outgoing traffic
- Allow incoming traffic over the `TCP` protocol on port 22 to enable SSH access to the cluster nodes
- Allow any incoming traffic over the `ICMP` protocol
- Allow incoming traffic over the `TCP` and `UDP` protocols on ports 30000–32767 for NodePort usage

Example of the layout configuration using NSX-T:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-T"
  externalIP: 10.0.0.1
  externalPort: 10022
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```

Example of the layout configuration using NSX-V:

```yaml
---
apiVersion: deckhouse.io/v1alpha1
kind: VCDClusterConfiguration
layout: Standard
provider:
  server: '<SERVER>'
  username: '<USERNAME>'
  password: '<PASSWORD>'
  insecure: true
sshPublicKey: ssh-rsa AAAABBBBB
organization: deckhouse
virtualDataCenter: MSK-1
virtualApplicationName: deckhouse
internalNetworkCIDR: 192.168.199.0/24
mainNetwork: internal
edgeGateway:
  name: "edge-gateway-01"
  type: "NSX-V"
  externalIP: 10.0.0.1
  externalPort: 10022
  NSX-V:
    externalNetworkName: external
    externalNetworkType: ext
masterNodeGroup:
  replicas: 1
  instanceClass:
    storageProfile: "Fast vHDD"
    sizingPolicy: 4cpu8mem
    template: "catalog/Ubuntu 22.04 Server"
    mainNetworkIPAddresses:
    - 192.168.199.2
```
