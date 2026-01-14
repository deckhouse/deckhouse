---
title: "The cni-cilium module: examples"
---

## Egress Gateway

{% alert level="warning" %}This feature is available in the following editions: SE+, EE.{% endalert %}

### Operation principle

Configuring an egress gateway requires two custom resources:

1. EgressGateway— describes the group of nodes that perform the egress gateway function in hot-standby mode:
   - Among the group of nodes match the `spec.nodeSelector`, the eligible nodes will be selected. One of them will be assigned as the active gateway. The active node is selected in [alphabetical order](https://docs.cilium.io/en/latest/network/egress-gateway/egress-gateway/index.html#selecting-and-configuring-the-gateway-node).

     Attributes of an eligible node:
     - The node is in `Ready` state.
       - The node is not in the maintenance state (i.e., it is not cordoned).
       - The `cilium-agent` on the node is in the `Ready` state.
     - When using EgressGateway in `VirtualIP` mode, an agent is launched on the active node which emulates a "virtual" IP address using the ARP protocol. The status of this agent's pod is also taken into account when determining the eligibility of a node.
     - Different EgressGateways can use the same nodes for operation. The active node is selected independently for each EgressGateway, which allows for load balancing between them.
1. EgressGatewayPolicy — describes the policy for routing network requests from pods in the cluster to a specific egress gateway defined using EgressGateway.

### Node maintenance

To perform maintenance on a node that is currently the active egress gateway, follow these steps:

1. Remove the node label to exclude it from the egress gateway candidate pool. Egress-label is the label specified in `spec.nodeSelector` of your EgressGateway.

    ```bash
    d8 k label node <node-name> <egress-label>-
    ```

1. Cordon the node to prevent new pods from starting:

    ```bash
    d8 k cordon <node-name>
    ```

    After this, Cilium will automatically select a new active node from the remaining candidates.
    Traffic will continue routing through the new gateway without interruption.

1. After maintenance is complete, return the node to service:

   ```bash
    d8 k uncordon <node-name>
    d8 k label node <node-name> <egress-label>=<value>
   ```

> Note: Reapplying the label may cause the node to become active again (if it is first in alphabetical order among candidates).
To avoid immediate failback, temporarily reduce the number of EgressGateway replicas or adjust priorities using additional labels.

### Comparison with CiliumEgressGatewayPolicy

The CiliumEgressGatewayPolicy implies configuring only one node as an egress gateway. If it fails, there are no failover mechanisms and the network connection will be broken.

### Egress Gateway configuration examples

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
      # Each node must have all the necessary routes configured to access all external public services,
      # the "public" interface must be prepared for automatic configuration of the "virtual" IP as a secondary IP address.
      # In case of failure of the active node, traffic will be redirected through the backup node and
      # the source IP address of the network packets will not change.
      ip: 172.18.18.242
      # List of network interfaces for Virtual IP
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

## HubbleMonitoringConfig

The cluster-scoped [HubbleMonitoringConfig](cr.html#hubblemonitoringconfig) resource is intended to configure data export from Hubble, which runs inside Cilium agents.

### HubbleMonitoringConfig configuration examples

#### Enabling extended metrics and flow logs export (with filters and field mask)

{% alert level="warning" %}
The [HubbleMonitoringConfig](cr.html#hubblemonitoringconfig) resource **must be named** `hubble-monitoring-config`.
{% endalert %}

Example of enabling metrics and export:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: HubbleMonitoringConfig
metadata:
  name: hubble-monitoring-config
spec:
  extendedMetrics:
    enabled: true
    collectors:
      - name: drop
        # Add additional labels context for the selected collector.
        contextOptions: "labelsContext=source_ip,source_namespace,source_pod,destination_ip,destination_namespace,destination_pod"
      - name: flow
  flowLogs:
    enabled: true
    # Allow only the specified events to be written to the log file /var/log/cilium/hubble/flow.log.
    allowFilterList:
      verdict:
        - DROPPED
        - ERROR
    # Exclude events matching the denyFilterList from the log file.
    denyFilterList:
      source_pod:
        - kube-system/
      destination_pod:
        - kube-system/
    # Persist only the specified fields in each record.
    fieldMaskList:
      - time
      - verdict
    # Maximum log file size (in MB) before rotation.
    fileMaxSizeMB: 30
```

### Collecting Hubble flow logs with the log-shipper module

To collect flow logs, use the [`log-shipper`](https://deckhouse.ru/modules/log-shipper/) module.

Create a ClusterLoggingConfig resource that reads the log file from the node filesystem:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ClusterLoggingConfig
metadata:
  name: cilium-hubble-flow-logs
spec:
  type: File
  file:
    include:
      - /var/log/cilium/hubble/flow.log
```
