---
title: "The cni-cilium module: advanced usage"
description: "Connecting the pod networks of peered clusters: requirements, enabling, neighbours, status, disconnecting, troubleshooting."
---

## Cluster peering

{% alert level="warning" %}
Experimental, available in the Ultimate/EE editions only. The ClusterMesh and ClusterNetworkPeer resources are `v1alpha1`.
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
| Control-plane nodes | Control-plane nodes of the neighbour | TCP, the neighbour's `clusterMesh.hostPort.port` (4294 by default) |

### Enabling

The example meshes clusters A and B. The peering is named `cluster-b` in cluster A and `cluster-a` in cluster B. Run every step in both clusters.

1. Peer the clusters by the [`peering` module guide](/modules/peering/admin_guide.html) up to the `Active` phase. Do not open capabilities yet.

1. Enable ClusterMesh in the `cni-cilium` ModuleConfig (`d8 k edit mc cni-cilium`). In cluster B, `clusterID: 2`:

   ```yaml
   spec:
     settings:
       tunnelMode: VXLAN
       clusterMesh:
         enabled: true
         clusterID: 1
   ```

   The Cilium agents restart. Then restart all pods of the cluster so they get identities with the cluster ID. Do it in a maintenance window. If Cilium runs without a tunnel, switch it to VXLAN in a separate maintenance window first.

   Wait for `READY: True` in `d8 k get clustermesh cluster` and `ACCEPTED: True` in `d8 k get peeringcapability network`.

1. Add `network` to `spec.capabilities` of the `ClusterPeer`, keeping the other capabilities (`d8 k edit clusterpeer cluster-b`). The capability works once both sides list it: check `EFFECTIVE` in `d8 k get clusterpeer cluster-b`.

1. Create a ClusterNetworkPeer named after the `ClusterPeer`. In cluster B, `cluster-a`:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetworkPeer
   metadata:
     name: cluster-b
   spec: {}
   ```

1. Wait for `PHASE: Configured` and `READY: True` in `d8 k get clusternetworkpeers`. It usually takes a minute or two.

If the ModuleConfig is rejected, fix the setting named in the message: set `clusterMesh.clusterID`, an explicit `tunnelMode: VXLAN`, or turn encryption off.

### Several neighbours

The cluster meshes with each neighbour separately: a `ClusterPeer` with the `network` capability and a ClusterNetworkPeer on both sides.

A neighbour's ID and pod subnet stay taken while the cluster has its ClusterNetworkPeer, even if the mesh with it is down. A new neighbour with a taken ID or subnet gets an error (`ClusterIDConflict`, `PodCIDROverlap`); the neighbour already meshed keeps working. To resolve the conflict, change the new neighbour's ID or delete the old neighbour's ClusterNetworkPeer.

### ClusterNetworkPeer status

Phases:

- `Pending`: the neighbour has not agreed or has not opened `network`;
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
| `PeerNotActive` | Name the ClusterNetworkPeer after the `ClusterPeer` and check the peering |
| `CapabilityNotRequested` | Add `network` to `spec.capabilities` of the `ClusterPeer` |
| `CapabilityNotEffective` | Add `network` on the neighbour's side |
| `AwaitingPeer` | Create the ClusterNetworkPeer named in the message on the neighbour |
| `PeerUnreachable`, `ClusterPeerUnreadable`, `PeerAccessNotReady` | Check the peering. With `... not found`, wait until ClusterMesh is deployed on the neighbour |
| `TunnelModeNotVXLAN`, `VXLANPortMismatch` | Switch the neighbour to VXLAN with the same port. Clusters with different ports cannot be meshed |
| `ClusterIDConflict` | Change the neighbour's ID or delete the ClusterNetworkPeer of the neighbour named in the message |
| `ClusterNameConflict` | Delete the extra peering with the same cluster. A cluster cloned with its UUID cannot be meshed |
| `PodCIDROverlap` | The pod subnets (podCIDR) of the clusters overlap. These clusters cannot be meshed |
| `RemoteNotReady` | The neighbour has not published its state yet. If it persists, check `d8 k get clustermesh cluster` and the `clustermesh-apiserver` replicas on the neighbour |
| `AwaitingPeerSignature` | Wait for the next check. If it persists, look at the neighbour's ClusterNetworkPeer |
| `PeerCSRInvalid` | Recreate the ClusterNetworkPeer on the neighbour's side |
| `LastKnownConfiguration` | The peering is unavailable, the mesh runs on the last configuration. Restore the peering |

### Changing settings

- To change `clusterMesh.clusterID`, check that the new ID is free among the cluster and its neighbours. Change it in a maintenance window, then restart all pods of the cluster.
- To change `clusterMesh.hostPort.port`, open the new port in the neighbours' firewalls first.

### Disconnecting a neighbour

To disconnect a neighbour, delete its ClusterNetworkPeer on either side:

```shell
d8 k delete clusternetworkpeer cluster-b
```

The mesh is torn down on both sides, connections between the clusters break. To restore the mesh, create the ClusterNetworkPeer again.

To give the ID or pod subnet of a retired cluster to another cluster, delete the retired cluster's ClusterNetworkPeer on all its neighbours.

### Disabling ClusterMesh

1. Delete all ClusterNetworkPeers.
1. Set `clusterMesh.enabled: false` in the `cni-cilium` ModuleConfig in a maintenance window, then restart all pods of the cluster.
1. Remove `network` from `spec.capabilities` of the `ClusterPeer` objects.
