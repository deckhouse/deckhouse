---
title: "Egress Gateway configuring"
permalink: en/admin/egress.html
---

Egress Gateway enables centralized management and control of outbound traffic and provides features such as encryption, routing and monitoring.

{% alert level="warning"%}
To use the Egress Gateway, enable the [cni-cilium](/#) module in the cluster.
{% endalert %}

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#using-egress-gateway -->

## Egress Gateway usage modes

{% alert level="warning" %}
The feature is available only in the Enterprise Edition
{% endalert %}

### Basic mode

Pre-configured IP addresses are used on egress nodes.

<div data-presentation="../../presentations/cni-cilium/egressgateway_base_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1Gp8b82WQQnYr6te_zBROKnKmBicdhtX4SXNXDh3lB6Q/ --->

### Virtual IP mode

The ability to dynamically assign additional IP addresses to nodes is implemented.

<div data-presentation="../../presentations/cni-cilium/egressgateway_virtualip_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1jdn39uDFSraQIXVdrREBsRv-Lp4kPidhx4C-gvv1DVk/ --->

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/examples.html#egress-gateway -->

## Operation principle

To configure an egress gateway, two CRs must be configured:

* `EgressGateway` — describes the group of nodes that perform the egress gateway function in hot-standby mode:
  * Among the group of nodes matching the `spec.nodeSelector`, the eligible nodes will be detected and one of them will be assigned as the active one. Signs of an eligible node:
    * The node is in Ready state.
    * The node is not cordoned.
    * The cilium-agent on the node is in the Ready state.
  * When using `EgressGateway` in `VirtualIP` mode, an agent is launched on the active node which emulates "virtual" IP by ARP protocol. The status of this agent's Pod is also taken into account when determining the eligibility of a node.
  * Different `EgressGateways` can use common nodes for operation, and active nodes will be selected independently for each EgressGateway, thus distributing the load between them.
* `EgressGatewayPolicy` — describes the policy for routing network requests from pods in the cluster to a specific egress gateway described by `EgressGateway`.

## Comparison with CiliumEgressGatewayPolicy

The `CiliumEgressGatewayPolicy` implies configuring only a single node as an egress gateway. If it fails, there are no failover mechanisms and the network connection will be broken.

## Examples

### EgressGateway in PrimaryIPFromEgressGatewayNodeInterface mode (basic mode)

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: my-egressgw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # The "public" interface must have the same name on all nodes that matching the nodeSelector.
      # If the active node fails, traffic will be redirected through the backup node and
      # the source IP address of the network packets will change.
      interfaceName: eth1 
```

### EgressGateway in VirtualIPAddress mode

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: my-egressgw
spec:
  nodeSelector:
    node-role.deckhouse.io/egress: ""
  sourceIP:
    mode: VirtualIPAddress
    virtualIPAddress:
      # Each node must have all necessary routes configured for access to all external public services,
      # the "public" interface must be ready to accept a "virtual" IP as a secondary IP address.
      # In case of failure of the active node, traffic will be redirected through the backup node and
      # the source IP address of the network packets will not change.
      ip: 172.18.18.242
      # List of interfaces for Virtual IP
      interfaces:
      - eth1
```

### EgressGatewayPolicy

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: my-egressgw-policy
spec:
  destinationCIDRs:
  - 0.0.0.0/0
  egressGatewayName: my-egressgw
  selectors:
  - podSelector:
      matchLabels:
        app: backend
        io.kubernetes.pod.namespace: my-ns
```
