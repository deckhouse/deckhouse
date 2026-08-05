---
title: Network interaction of the platform components
permalink: en/reference/network_interaction.html
description: |
  Detailed information on configuring network policies for the Deckhouse Kubernetes Platform, particularly in environments with constraints on host-to-host network communications. Outlines the necessary conditions to enable tunneling modes for pod traffic using CNI Cilium and Flannel.
lang: en
---

If the infrastructure where Deckhouse Kubernetes Platform (DKP) is running has requirements to limit host-to-host network communications, the following conditions must be met:

* Tunneling mode for traffic between pods is enabled ([configuration](/modules/cni-cilium/configuration.html#parameters-tunnelmode) for CNI Cilium, [configuration](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) for CNI Flannel).
* Traffic between [podSubnetCIDR](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration) encapsulated within a VXLAN is allowed (if inspection and filtering of traffic within a VXLAN tunnel is performed).
* If there is integration with external systems (e.g. LDAP, SMTP or other external APIs), it is required to allow network communication with them.
* Local network communication is fully allowed within each individual cluster node.
* Inter-node communication is allowed on the ports shown in the tables on the current page. Note that most ports are in the 4200-4299 range. When new platform components are added, they will be assigned ports from this range (if it is possible).

{% offtopic title="How to check the current VXLAN port..." %}

```bash
d8 k -n d8-cni-cilium get cm cilium-config -o yaml | grep tunnel
```

Example output:

```console
routing-mode: tunnel
tunnel-port: "4298"
tunnel-protocol: vxlan
```

{%- endofftopic %}

{% alert level="info" %}
Changes related to the addition, removal, or reassignment of ports in the tables
are listed in the "Network" section of a respective DKP version on the [Release notes](../release-notes.html) page.
{% endalert %}

## Requirements for network latency between master nodes

{% alert level="info" %}
DKP supports HA mode using [arbiter nodes](../../admin/configuration/high-reliability-and-availability/enable.html#configuring-ha-mode-with-two-master-nodes-and-an-arbiter-node). The network latency requirements listed below also apply to arbiter nodes.
{% endalert %}

Etcd only commits a writing after the change has been replicated to a majority of master nodes. Therefore, the round-trip time (RTT) between master nodes directly determines the cluster’s performance.
A single operation in the cluster typically consists of several consecutive writes, so as latency increases, the application of manifests, operator operations, and Kubernetes API responsiveness slow down noticeably.

**The sum of RTT and jitter between master nodes must not exceed 100 ms.**

{% alert level="warning" %}
Round-trip time (RTT) is specified, not one-way delay: for example, the `ping` command returns RTT.

Jitter—the variation in latency—is added to the RTT. For example, with an RTT of 90 ms and jitter of 20 ms, the total is 110 ms, and the requirement is not met, even though the RTT value itself is within the 100 ms limit.
{% endalert %}

When planning a cluster, it is recommended to aim for a latency of no more than 50 ms between future master nodes—that is, a twofold margin relative to the requirement. This margin is necessary for the following reasons:

* Pre-deployment measurements are performed on an idle network and may underestimate actual values.
* You need to account for latency spikes and variation, as well as possible infrastructure operations—for example, migrating a virtual machine to a zone with higher latency.

{% offtopic title="Where does the 100 ms value come from..." %}

DKP uses the standard etcd values:

* `ETCD_HEARTBEAT_INTERVAL` — 100 ms, the frequency at which the leader sends heartbeat messages to the other cluster members.
* `ETCD_ELECTION_TIMEOUT` — 1000 ms, the timeout for heartbeat messages; after this time, a cluster member begins leader re-election.

[The etcd documentation](https://etcd.io/docs/v3.6/tuning/) requires that the election timeout be at least 10x RTT. Hence, the maximum allowed value is:

```text
1000 ms / 10 = 100 ms
```

{%- endofftopic %}

### Potential issues

The main consequence of increased RTT is a proportional slowdown of the cluster. Network latency is directly reflected in write times: with an RTT of 300 ms, each write to etcd will take at least 300 ms. There is no threshold here; performance degrades continuously as latency increases.

The second issue is etcd leader re-elections. These are triggered not by the magnitude of the latency, but by its complete loss: a cluster member initiates an election if it does not receive messages from the leader within the time specified by `ETCD_ELECTION_TIMEOUT` (taking into account etcd’s built-in randomization mechanism—from 1,000 to 2,000 ms). With a 100-ms heartbeat interval, this corresponds to 10–20 consecutive missed heartbeat messages. Typical causes:

* A network outage or packet loss lasting 1–2 seconds.
* Disk write delays on the leader node, preventing etcd from sending heartbeat messages in time.
* A restart of the etcd process.

Isolated spikes in latency do not trigger re-elections: heartbeat messages arrive, albeit late. Only writes that occur during the spike are delayed.

However, a high RTT makes re-elections protracted when they do occur. Votes are collected within a single RTT and must fit within `ETCD_ELECTION_TIMEOUT`. If the RTT is close to this timeout, the election does not complete in time and restarts, leaving the cluster leaderless. A tenfold margin is required to ensure the election succeeds on the first attempt.

During leader re-elections, etcd does not process write requests, so the Kubernetes API returns errors. The `KubeEtcdHighNumberOfLeaderChanges` alert signals frequent leader re-elections—more than three leader re-elections in 10 minutes.

### Checking current latencies

Latency and its variation can be measured using `ping`. The command is run on one of the master nodes, or—when planning the cluster—on the server intended to become a master node. ICMP is sufficient for this check, as it is already included in the requirements for traffic between nodes:

```shell
ping -c 100 -i 0.2 <IP address of another master node>
```

Example of the resulting output:

```console
rtt min/avg/max/mdev = 25.085/25.502/35.933/1.185 ms
```

Here, `avg` is the average RTT, `mdev` is the delay variation (jitter), and `max` is the actual maximum—that is, the RTT including spikes. There is no need to add the RTT and jitter manually: you should compare the `max` value against the requirements. When planning a cluster, we recommend staying within 50 ms; for a live cluster, do not exceed 100 ms.

## Network latency at the other nodes

The control plane has no requirements regarding network latency on nodes that are not master nodes: latency on these nodes does not affect the cluster’s stability. The only factor that matters is the duration of the loss of communication with master nodes—by default, after 40 seconds, a node enters the `Unreachable` state, and after 5 minutes, its pods are migrated to other nodes. Both thresholds are configured using the [nodeMonitorGracePeriodSeconds](/modules/control-plane-manager/configuration.html#parameters-nodemonitorgraceperiodseconds) and [failedNodePodEvictionTimeoutSeconds](/modules/control-plane-manager/configuration.html#parameters-failednodepodevictiontimeoutseconds) parameters.

These delays affect the performance of everything running on the nodes—including the platform’s own components. For components that access the Kubernetes API (DKP operators, Prometheus, Ingress controllers), latency to the master nodes is added to every request. Latency between nodes is added to every network request between pods and to DNS lookups, and—when using replicated storage—to the time it takes to write to disk.

Thus, the acceptable value is determined by the requirements of the running workload. For clusters distributed across remote sites, this means that the platform will continue to operate, but application latencies will reflect the geographic distribution of the nodes.

{% include network_security_setup.liquid %}
