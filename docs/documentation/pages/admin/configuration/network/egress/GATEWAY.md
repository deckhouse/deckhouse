---
title: "Egress Gateway configuring"
permalink: en/admin/configuration/network/egress/gateway.html
---

Egress Gateway enables centralized management and control of outbound traffic and provides features such as encryption, routing and monitoring.

{% alert level="warning"%}
To use the Egress Gateway, enable the [cni-cilium](../../reference/mc/cni-cilium/) module in the cluster.
{% endalert %}

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/#using-egress-gateway -->

## Egress Gateway usage modes

{% alert level="warning" %}
The feature is available only in the Enterprise Edition
{% endalert %}

### Basic mode

Pre-configured IP addresses are used on egress nodes.

<div data-presentation="../../../presentations/cni-cilium/egressgateway_base_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1Gp8b82WQQnYr6te_zBROKnKmBicdhtX4SXNXDh3lB6Q/ --->

### Virtual IP mode

The ability to dynamically assign additional IP addresses to nodes is implemented.

<div data-presentation="../../../presentations/cni-cilium/egressgateway_virtualip_en.pdf"></div>
<!--- Source: https://docs.google.com/presentation/d/1jdn39uDFSraQIXVdrREBsRv-Lp4kPidhx4C-gvv1DVk/ --->

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/cni-cilium/examples.html#egress-gateway -->

## Operation principle

Configuring an egress gateway requires two custom resources:

1. `EgressGateway` — describes the group of nodes that perform the egress gateway function in hot-standby mode:
   - Among the group of nodes match the `spec.nodeSelector`, the eligible nodes will be selected. One of them will be assigned as the active gateway. The active node is selected in [alphabetical order](https://docs.cilium.io/en/latest/network/egress-gateway/egress-gateway/index.html#selecting-and-configuring-the-gateway-node).

     Attributes of an eligible node:
     - The node is in `Ready` state.
       - The node is not in the maintenance state (i.e., it is not cordoned).
       - The `cilium-agent` on the node is in the `Ready` state.
     - When using EgressGateway in `VirtualIP` mode, an agent is launched on the active node which emulates a "virtual" IP address using the ARP protocol. The status of this agent's pod is also taken into account when determining the eligibility of a node.
     - Different EgressGateways can use the same nodes for operation. The active node is selected independently for each EgressGateway, which allows for load balancing between them.
1. `EgressGatewayPolicy` — describes the policy for routing network requests from pods in the cluster to a specific egress gateway defined using EgressGateway.

## Node maintenance

To perform maintenance on a node that is currently the active egress gateway, follow these steps:

1. Remove the node label to exclude it from the egress gateway candidate pool. Egress-label is the label specified in `spec.nodeSelector` of your EgressGateway.

    ```bash
    kubectl label node <node-name> <egress-label>-
    ```

1. Cordon the node to prevent new pods from starting:

    ```bash
    kubectl cordon <node-name>
    ```

    After this, Cilium will automatically select a new active node from the remaining candidates.
    Traffic will continue routing through the new gateway without interruption.

1. After maintenance is complete, return the node to service:

   ```bash
    kubectl uncordon <node-name>
    kubectl label node <node-name> <egress-label>=<value>
   ```

> Note: Reapplying the label may cause the node to become active again (if it is first in alphabetical order among candidates).
To avoid immediate failback, temporarily reduce the number of EgressGateway replicas or adjust priorities using additional labels.

## Comparison with CiliumEgressGatewayPolicy

The `CiliumEgressGatewayPolicy` implies configuring only one node as an egress gateway. If it fails, there are no failover mechanisms and the network connection will be broken.

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
      # Each node must have all the necessary routes configured to access all external public services,
      # the "public" interface must be prepared for automatic configuration of the "virtual" IP as a secondary IP address.
      # In case of failure of the active node, traffic will be redirected through the backup node and
      # the source IP address of the network packets will not change.
      ip: 172.18.18.242
      # List of network interfaces for Virtual IP
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
