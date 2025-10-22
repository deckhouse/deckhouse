---
title: "Outgoing traffic management"
permalink: en/stronghold/documentation/admin/platform-management/network/egress.html
---

{% alert level="warning" %}
This feature isn't available in Community Edition.
{% endalert %}

In a deployed cluster, some nodes may be isolated from the external network for security reasons.
In such cases, internet access is provided through designated nodes that have access to external resources.

Traffic is then routed through pre-configured Egress gateways (EgressGateway) according to the specified policy (EgressGatewayPolicy).

## Egress gateway node groups

The EgressGateway resource describes a node group that functions as an Egress gateway.
To add a node to this group, assign a corresponding label to it:

```shell
d8 k label node <node name> dedicated/egress=
```

### PrimaryIPFromEgressGatewayNodeInterface mode

The primary IP address bound to the public network interface of the node will be used as the IP address.
If the active node fails and a new node is assigned, the sender IP address in network packets will change.

To create an Egress gateway, apply the following EgressGateway resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    dedicated/egress: ""
  sourceIP:
    # The primary IP address bound to the public network interface of the node will be used as the IP address
    mode: PrimaryIPFromEgressGatewayNodeInterface
    primaryIPFromEgressGatewayNodeInterface:
      # Since the "public" interface must have the same name on all nodes in the group (for example, eth1),
      # make sure to configure the network subsystem first on all Egress nodes
      interfaceName: eth1
EOF
```

### VirtualIPAddress mode

Alternatively, you can assign a virtual IP address to the node group.

This virtual address will be bound to the master node, providing traffic routing to external services.
If the master node fails, all active connections will be terminated,
and a new master node will be selected and assigned the same virtual address.
The client IP address will remain unchanged for external services.

To create an Egress gateway with a virtual address, apply the following EgressGateway resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGateway
metadata:
  name: egress-gw
spec:
  nodeSelector:
    dedicated/egress: ""
  sourceIP:
    # The primary IP address bound to the public network interface of the node will be used as the IP address
    mode: VirtualIPAddress
    virtualIPAddress:
      # All routes required to access external public services must be configured on every node
      # The "public" interface must be prepared to automatically configure the "virtual" IP address as a secondary IP address
      ip: 172.18.18.242
EOF
```

### Egress gateway node suitability

The nodes specified by `spec.nodeSelector` of the EgressGateway resource are evaluated and only the suitable ones are selected.
A node is considered suitable if it meets the following conditions:

1. The node's status is Ready.
1. The node isn't under maintenance, meaning it isn't cordoned.
1. cilium-agent on the node is Ready.

To check whether a node is suitable for inclusion to the Egress gateway group, run the following commands:

```shell
# Display nodes specified by spec.nodeSelector:
d8 k get nodes -l dedicated/egress="" -ojson | jq -r '.items[].metadata.name'

# Display nodes in the Ready status:
d8 k get nodes -ojson | jq -r '.items[] | select(.status.conditions[] | select(.type == "Ready" and .status == "True")) | .metadata.name'

# Display nodes that aren't under maintenance:
d8 k get nodes --field-selector spec.unschedulable=false -ojson | jq -r .items[].metadata.name

# Display nodes with cilium-agent running:
d8 k get pods -n d8-cni-cilium -l app=agent -ojson | jq -r '.items[].spec.nodeName'
```

In a node group, one node is designated as the master node. External traffic is routed through this node.
The other nodes remain in hot-standby mode.
If the active node fails, the active connections will be terminated,
and a new master node will be selected from the remaining nodes.
Traffic will then be routed through this new node.

Different Egress gateways can use common nodes for operation.
Master nodes are selected independently, helping to distribute the load across them.

## Traffic redirection policy

The EgressGatewayPolicy resource describes the policy for redirecting application traffic from virtual machines to the specified Egress gateway.
The policy and virtual machines are matched by labels.

To add a virtual machine to the policy, assign a label to it:

```shell
d8 k label vm <virtual machine name> app=backend
```

To set a traffic redirection policy,
create an EgressGatewayPolicy resource by specifying criteria in the `.spec.selectors` field
to select the virtual machines to which the policy will apply.
Use the following example:

```yaml
d8 k apply -f - <<EOF
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: egress-gw-policy
spec:
  destinationCIDRs:
    - 0.0.0.0/0
  egressGatewayName: egress-gw
  selectors:
    - podSelector:
        matchLabels:
          # This policy will be applied to all pods with the label app=backend in the default namespace on all virtual machines
          app: backend
          io.kubernetes.pod.namespace: default
EOF
```

To ensure the EgressGatewayPolicy policy has been properly applied to the virtual machine,
run the following command on it to verify network connectivity and routing:

```shell
curl ifconfig.me
```

Upon verification, you will see the master node's IP address if the Egress gateway is in `PrimaryIPFromEgressGatewayNodeInterface` mode,
or the virtual IP address if the Egress gateway is in `VirtualIPAddress` mode.
