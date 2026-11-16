---
title: "The cni-cilium module: advanced usage"
description: "Connecting the pod networks of peered clusters: requirements, enabling, neighbours, status, disconnecting, troubleshooting."
---

## Cluster peering

{% alert level="warning" %}
Experimental, available in the Ultimate/EE editions only. The NetworkPeering and ClusterNetworkPeer resources are `v1alpha1`.
{% endalert %}

The module connects the pod networks of clusters peered by the [`peering`](/modules/peering/) module: a pod reaches a pod of another cluster by IP address, a service can use backends of other clusters (see [Cluster peering](usage.html#cluster-peering) in usage). The clusters only exchange the mesh settings over peering; traffic goes directly between the nodes.

Limitations:

- VXLAN mode without encryption only; pod traffic between clusters is not encrypted, so mesh clusters over trusted networks only;
- neighbours connect to control-plane nodes directly, by node addresses; a load balancer or NAT between clusters is not supported;
- the pod subnets of the clusters must not overlap;
- a cluster with all its neighbours is at most 255 clusters;
- the mesh is not transitive: if A is meshed with B and B with C, A and C are not connected;
- enabling, disabling and changing the cluster ID change the security identities of all pods: restart all pods of the cluster afterwards, in a maintenance window.

### Requirements

| Requirement | How to check |
|---|---|
| The `peering` module is enabled, `ClusterPeer` is `Active` on both sides | `d8 k get clusterpeers` |
| Cilium runs in VXLAN mode, [`tunnelMode: VXLAN`](configuration.html#parameters-tunnelmode) is set explicitly in the ModuleConfig | `d8 k get mc cni-cilium -o jsonpath='{.spec.settings.tunnelMode}'` prints `VXLAN` |
| Encryption is off: [`encryption.mode`](configuration.html#parameters-encryption-mode) is unset or `Disabled` | `d8 k get mc cni-cilium -o jsonpath='{.spec.settings.encryption.mode}'` |
| The same VXLAN port in all clusters | `d8 k -n d8-cni-cilium get cm cilium-config -o jsonpath='{.data.tunnel-port}'` |
| The pod subnet does not overlap the neighbours' pod subnets | `podSubnetCIDR` in `d8 k -n kube-system get secret d8-cluster-configuration -o jsonpath='{.data.cluster-configuration\.yaml}' \| base64 -d` |
| The cluster ID is unique among the cluster and all its neighbours | Assign IDs from one list for all clusters that may be meshed |

The module sets the VXLAN port. Clusters with different ports cannot be meshed. Enabling the `virtualization` module can change the port.

Service subnets may overlap.

Network access between clusters, by node `InternalIP` addresses, without NAT:

| From | To | Port |
|---|---|---|
| Every node | Every node of the neighbour | UDP, the VXLAN port |
| Control-plane nodes | Control-plane nodes of the neighbour | TCP, the neighbour's `networkPeering.hostPort.port` (4294 by default) |

### Enabling

The example meshes clusters A and B. The peering is named `cluster-b` in cluster A and `cluster-a` in cluster B. Run every step in both clusters.

1. Peer the clusters by the [`peering` module guide](/modules/peering/admin_guide.html) up to the `Active` phase. Do not open capabilities yet.

1. Enable NetworkPeering in the `cni-cilium` ModuleConfig (`d8 k edit mc cni-cilium`). In cluster B, `clusterID: 2`:

   ```yaml
   spec:
     settings:
       tunnelMode: VXLAN
       networkPeering:
         enabled: true
         clusterID: 1
   ```

   The Cilium agents restart. Then restart all pods of the cluster so they get identities with the cluster ID. Do it in a maintenance window. If Cilium runs without a tunnel, switch it to VXLAN in a separate maintenance window first.

   Wait for `READY: True` in `d8 k get networkpeering cluster` and `ACCEPTED: True` in `d8 k get peeringcapability network`.

1. Add `network` to `spec.capabilities` of the `ClusterPeer`, keeping the other capabilities (`d8 k edit clusterpeer cluster-b`). The module creates a ClusterNetworkPeer named after the `ClusterPeer`, it shows the state of the mesh with the neighbour. The capability works once both sides list it: check `EFFECTIVE` in `d8 k get clusterpeer cluster-b`.

1. Wait for `PHASE: Configured` and `READY: True` in `d8 k get clusternetworkpeers`. It usually takes a minute or two.

If the ModuleConfig is rejected, fix the setting named in the message: set `networkPeering.clusterID`, an explicit `tunnelMode: VXLAN`, or turn encryption off.

### Several neighbours

The cluster meshes with each neighbour separately: the `ClusterPeer` lists the `network` capability on both sides. The module keeps a ClusterNetworkPeer for every `ClusterPeer` that lists `network` and deletes it when the capability or the `ClusterPeer` is removed.

A neighbour's ID and pod subnet stay taken while the cluster has its ClusterNetworkPeer, even if the mesh with it is down. A new neighbour with a taken ID or subnet gets an error (`ClusterIDConflict`, `PodCIDROverlap`); the neighbour already meshed keeps working. To resolve the conflict, change the new neighbour's ID or remove `network` from the old neighbour's `ClusterPeer`.

### ClusterNetworkPeer status

Phases:

- `Pending`: the mesh waits for the neighbour or for the peering, the reason is in the conditions;
- `Negotiating`: certificates are being exchanged;
- `Configured`: the mesh is configured;
- `Error`: a check failed, the reason is in the conditions.

To print the conditions, run:

```shell
d8 k get clusternetworkpeer cluster-b \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

| Reason | What to do |
|---|---|
| `PeerNotActive` | Check the peering |
| `CapabilityNotOffered` | On the neighbour's side, add `network` to the `ClusterPeer` and enable NetworkPeering |
| `CapabilityNotAccepted` | This cluster does not offer `network` to the neighbour: check the `Accepted` and `GrantsApplied` conditions in `d8 k get peeringcapability network -o yaml` |
| `CapabilityNotEffective` | Both sides offer `network`, the peering has not applied it yet. If it persists, check `d8 k get clusterpeer cluster-b` |
| `AwaitingPeer` | The neighbour has not created its ClusterNetworkPeer named in the message yet. If it persists, check `d8 k -n d8-cni-cilium logs deploy/clustermesh-controller` on the neighbour |
| `PeerUnreachable`, `ClusterPeerUnreadable`, `PeerAccessNotReady` | Check the peering. With `... not found`, wait until NetworkPeering is deployed on the neighbour |
| `TunnelModeNotVXLAN`, `VXLANPortMismatch` | Switch the neighbour to VXLAN with the same port. Clusters with different ports cannot be meshed |
| `ClusterIDConflict` | Change the neighbour's ID or remove `network` from the `ClusterPeer` of the neighbour named in the message |
| `ClusterNameConflict` | Delete the extra peering with the same cluster. A cluster cloned with its UUID cannot be meshed |
| `PodCIDROverlap` | The pod subnets (podCIDR) of the clusters overlap. These clusters cannot be meshed |
| `RemoteNotReady` | The neighbour has not published its state yet. If it persists, check `d8 k get networkpeering cluster` and the `clustermesh-apiserver` replicas on the neighbour |
| `AwaitingPeerSignature` | Wait for the next check. If it persists, look at the neighbour's ClusterNetworkPeer |
| `PeerCSRInvalid` | On the neighbour's side, remove `network` from the `ClusterPeer`, wait until its ClusterNetworkPeer is deleted, and add `network` back |
| `LastKnownConfiguration` | The peering is unavailable, the mesh runs on the last configuration. Restore the peering |

### Changing settings

- To change `networkPeering.clusterID`, check that the new ID is free among the cluster and its neighbours. Change it in a maintenance window, then restart all pods of the cluster.
- To change `networkPeering.hostPort.port`, open the new port in the neighbours' firewalls first.

### Disconnecting a neighbour

To disconnect a neighbour, remove `network` from `spec.capabilities` of its `ClusterPeer` on either side (`d8 k edit clusterpeer cluster-b`). The module deletes the ClusterNetworkPeer.

The mesh is torn down on both sides, connections between the clusters break. To restore the mesh, add `network` back.

To give the ID or pod subnet of a retired cluster to another cluster, remove `network` from the retired cluster's `ClusterPeer` on all its neighbours, or delete that `ClusterPeer`.

### Disabling NetworkPeering

1. Remove `network` from `spec.capabilities` of the `ClusterPeer` objects. Wait until `d8 k get clusternetworkpeers` lists none: the module deletes them within a minute.
1. Set `networkPeering.enabled: false` in the `cni-cilium` ModuleConfig in a maintenance window, then restart all pods of the cluster.
