---
title: "The cni-cilium module: examples"
---

## Egress Gateway

{% alert level="warning" %}This feature is available in the following editions: SE+, EE.{% endalert %}

### Operation principle

To configure an egress gateway, two CRs must be configured:

* `EgressGateway` — describes the group of nodes that perform the egress gateway function in hot-standby mode:
  * Among the group of nodes matching the `spec.nodeSelector`, the eligible nodes will be detected and one of them will be assigned as the active one - selected in [alphabetical order](https://docs.cilium.io/en/latest/network/egress-gateway/egress-gateway/index.html#selecting-and-configuring-the-gateway-node). Signs of an eligible node:
    * The node is in Ready state.
    * The node is not cordoned.
    * The cilium-agent on the node is in the Ready state.
  * When using `EgressGateway` in `VirtualIP` mode, an agent is launched on the active node which emulates "virtual" IP by ARP protocol. The status of this agent's Pod is also taken into account when determining the eligibility of a node.
  * Different `EgressGateways` can use common nodes for operation, and active nodes will be selected independently for each EgressGateway, thus distributing the load between them.
* `EgressGatewayPolicy` — describes the policy for routing network requests from pods in the cluster to a specific egress gateway described by `EgressGateway`.

### Node Maintenance

To perform maintenance on a node that is currently acting as the active egress gateway, follow these steps:
Remove the node label to exclude it from the egress gateway candidate pool:
```bash
kubectl label node <node-name> <egress-label>-
```
(where egress-label is the label specified in your EgressGateway's spec.nodeSelector)

Cordon the node to prevent new pods from being scheduled:
```bash
kubectl cordon <node-name>
```
After this Cilium will automatically select a new active node from the remaining candidates.
Traffic will continue routing through the new gateway without interruption.

After maintenance is complete, return the node to service:
```bash
kubectl uncordon <node-name>
kubectl label node <node-name> <egress-label>=<value>
```
Note:
Reapplying the label may cause the node to become active again (if it is first in alphabetical order among candidates).
To avoid immediate failback, temporarily reduce EgressGateway replicas or adjust priorities using additional labels.


### Comparison with CiliumEgressGatewayPolicy

The `CiliumEgressGatewayPolicy` implies configuring only a single node as an egress gateway. If it fails, there are no failover mechanisms and the network connection will be broken.

### Examples

#### EgressGateway in PrimaryIPFromEgressGatewayNodeInterface mode (basic mode)

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: my-egressgw
spec:
  nodeSelector:
    dedicated/egress: ""
  sourceIP:
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # The "public" interface must have the same name on all nodes that matching the nodeSelector.
      # If the active node fails, traffic will be redirected through the backup node and
      # the source IP address of the network packets will change.
      interfaceName: eth1
```

#### EgressGateway in VirtualIPAddress mode (Virtual IP mode)

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: my-egressgw
spec:
  nodeSelector:
    dedicated/egress: ""
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

#### EgressGatewayPolicy

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
