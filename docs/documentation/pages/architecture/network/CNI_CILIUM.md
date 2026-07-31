---
title: Cni-cilium module
permalink: en/architecture/network/cni-cilium.html
search: cni-cilium, cilium, ebpf
description: Architecture of the cni-cilium module in Deckhouse Kubernetes Platform.
---

The `cni-cilium` module provides a network in a cluster. It is based on the [Cilium](https://github.com/cilium/cilium) project.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/cni-cilium/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cni-cilium`](/modules/cni-cilium/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![Cni-cilium module architecture](../../images/architecture/network/c4-l2-cni-cilium.ru.png)

## Module components

The module consists of the following components:

1. **Operator** (Deployment): A component in the Cilium architecture that takes over the centralized management of global tasks in a Kubernetes cluster. Operator component performs the following functions:

   * **Custom resources management**: Automatically registers the Custom Resource Definitions (CRD) required for Cilium operation in the Kubernetes API, for example, CiliumEndpoint, CiliumEndpointSlice, CiliumNetworkPolicy. The full list of CRDs can be found [in the Cilium documentation](https://docs.cilium.io/en/stable/internals/cilium_operator/#crd-registration).

   * **Garbage collection for security identifiers**: Maintains a local cache of active identifiers (CiliumIdentity) and periodically scans it, deleting records about identifiers that have stopped signaling activity (heartbeat). This is important because identifiers are represented by a 16-bit integer, and running out of them (maximum 65536) can lead to problems.

   * **CiliumEndpointSlices (CES) Management**: Maintains up-to-date information about endpoints in the cluster, which is critical for the correct operation of network policies and load balancing.

   It consists of the following containers:

   * **operator**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to operator metrics. It is an [open source project](https://github.com/brancz/kube-rbac-proxy).

1. **Agent** (DaemonSet): Agent running on each node of the Kubernetes cluster. It is the main management component in the Cilium architecture. It is responsible for translating high-level configurations from the Kubernetes API into low-level settings for network interfaces and eBPF programs that actually handle traffic. Agent component performs the following functions:

   * **Endpoints management**: Monitors the status of all endpoints on its node. Each pod is assigned a unique IP address and a security identity (numeric identifier), which is generated based on Kubernetes labels.

   * **Network policies enforcement**: Calculates and applies network policies rules set in the cluster. Cilium uses the identity-based policies concept: the rules are based not on IP addresses, but on the security identity of the pod. The agent converts these rules into data structures (BPF maps) that the Linux kernel uses to filter traffic.

   * **IP Address Management (IPAM)**: Allocates IP addresses for pods scheduled on its node using IP address ranges from the `spec.podCIDRs` or `spec.PodCidr` fields of the specification of the corresponding Node resource.

   * **Datapath Orchestration**: Compiles, loads eBPF programs into the Linux kernel and binds them to network interfaces. It is these programs in the kernel that perform real packet processing: forwarding, policy filtering, connection tracking, NAT and load balancing.

   * **Service Load Balancing**: Configures load balancing rules for services in a cluster using eBPF capabilities.

   * **Cluster event handling**: The Agent works according to the event model: it constantly monitors changes in the Kubernetes API (creating pods, updating services, changing Endpoints, updating custom resources) and updates the configuration of eBPF programs on the fly.

   * **Oservability**: The agent runs a local Hubble server, which collects metrics and data on network flows. This allows you to monitor the network status, identify problems, and analyze traffic between pods. The Hubble server starts when the [`cilium-hubble`](/modules/cilium-hubble) module is enabled.

   How it works: when an event occurs in the cluster (for example, a new pod starts), the agent receives a notification about it and, based on the current cluster status and policies, configures eBPF programs so that traffic for this pod passes correctly.

   It consists of the following containers:

   * **cni-migration-init-checker**: Init container waiting for the migration process from another CNI plugin to complete, if such a migration has been started.
   * **check-wg-kernel-compat**: Init container checking the Linux kernel version for compliance with the minimum requirements for working with WireGuard used in CNI Cilium.
   * **check-linux-kernel**: Init container checking the Linux kernel version for compliance with the minimum requirements for working with CNI Cilium.
   * **clearing-unnecessary-iptables**: Init container running script for cleaning up unnecessary `iptables` rules.
   * **handle-vxlan-offload**: Init container correcting UPD segmentation settings (if [tunnel mode](/modules/cni-cilium/configuration.html#parameters-tunnelmode) is set to `VXLAN`).
   * **config**: Init container generating configuration file used to configure the agent.
   * **mount-cgroup**: Init container configuring Linux cgroups (control groups).
   * **apply-sysctl-overwrites**: Init container configuring the Linux kernel parameters.
   * **mount-bpf-fs**: Init container mounting the BPF file system.
   * **clean-cilium-state**: Init container running a script for clearing the CNI Cilium state.
   * **install-cni-binaries**: Init container running a script for installing binary files used by the Agent.
   * **agent**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing authorized access to agent metrics (described above).

1. **Cilium-cni**: Executable file run by the containerd component that passes certain commands as an argument in accordance with the [CNI specification](https://www.cni.dev/docs/spec/#cni-operations ), for example, ADD when starting the container and DEL when deleting it. Cilium-cni executable interacts with the Cilium agent API via a Unix socket and initiates a datapath configuration to provide network connectivity, load balancing, and network policies for pod. Datapath in Cilium is a component that runs in the Linux kernel and is responsible for real low—level processing of network packets in a Kubernetes cluster. Simply put, this is the "road" along which data travels: how packets travel from one pod to another, how they are routed, how network policies and load balancing are applied.

1. **Safe-agent-updater** (DaemonSet): A special application designed to eliminate disruptive situations that may occur when updating the version of the Cilium agent and lead to problems with the network availability of DKP components.

   It consists of the following containers:

   * **check-wg-kernel-compat**: Init container checking the Linux kernel version for compliance with the minimum requirements for working with WireGuard used in CNI Cilium.
   * **check-linux-kernel**: Init container checking the Linux kernel version for compliance with the minimum requirements for working with CNI Cilium.

   * A set of init containers for prepulling new versions of images of the corresponding containers of the agent component:

     * **prepull-image-cilium**.
     * **prepull-image-kube-rbac-proxy**.

   * **safe-agent-updater**: Init container that compares the values of special annotations in the DaemonSet manifest of the updated Cilium agent with the values of the corresponding annotations in the metadata of the agent pod running on the node. In particular, the `safe-agent-updater-daemonset-generation` annotation stores the hash sum of the agent image. If the hash sums do not match, safe-agent-updater deletes the running pod and waits until the pod with the new agent version enters the `Ready` state.

   * A set of sidecar containers for prepulling new versions of images of the corresponding containers of the agent component. Containers are on pause and perform only the function of storing images:

     * **pause-cilium**.
     * **pause-check-linux-kernel**.
     * **pause-kube-rbac-proxy**.
     * **pause-pause-handle-vxlan-offload**.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Requests the IP ranges of cluster nodes via the `PodCidr`/`podCIDRs` attributes of the Node resource specification.
   * Authorizes requests for agent and operator metrics.
   * Manages Cilium custom resources.

The following external components interact with the module:

1. **Prometheus-main**: Collects metrics from agent and operator.
2. **Containerd**: Runs the cilium-cni executable file with certain commands according to the CNI specification, for example, ADD when starting the container and DEL when deleting it.
