---
title: "Underlay networks for hardware device passthrough"
permalink: en/admin/configuration/network/sdn/underlay-networks.html
description: |
  Software Defined Networking: underlay networks for hardware device passthrough
---

The [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork) resource enables direct attachment of physical network interfaces (Physical Functions and Virtual Functions) to pods via Kubernetes Dynamic Resource Allocation (DRA). This feature is designed for high-performance workloads that require direct hardware access, such as DPDK applications.

## Key features

DKP implements the following features for working with underlay networks:

* **Hardware device passthrough**: Physical network interfaces (PF/VF) are directly exposed to pods, bypassing the kernel network stack for maximum performance.
* **SR-IOV configuration**: Automatic configuration of SR-IOV on selected Physical Functions to create Virtual Functions, allowing multiple pods to share the same hardware.
* **DPDK support**: Devices can be bound in different modes suitable for DPDK workloads:
  * **VFIO-PCI**: Explicitly connects a network device to the pod by binding it to the `vfio-pci` driver. The corresponding VFIO device files (e.g., `/dev/vfio/vfio0`) are mounted into the pod for userspace access.
  * **DPDK**: A universal mode that automatically selects the appropriate driver for the network adapter vendor. For Mellanox NICs, the device is bound to the `mlx5_core` driver with both the netdev interface and necessary device files mounted (InfiniBand verbs files, `/dev/net/tun`, and the corresponding sysfs directory). For other vendors, the device is bound via VFIO (same as VFIO-PCI mode).
  * **NetDev**: Only the Linux network interface is passed through to the pod as a standard kernel network device.

## Operation modes

The following device allocation modes are supported, which determine how physical interfaces are provided to hosts:

* **Shared mode**: Creates Virtual Functions (VF) from Physical Functions (PF) using SR-IOV, allowing multiple pods to share the same hardware. Each pod receives one or more VFs.
* **Dedicated mode**: Exposes each Physical Function as an exclusive device without SR-IOV. Each pod gets exclusive access to a complete PF, suitable for workloads requiring maximum performance.

## Automatic interface grouping

When `autoBonding` is enabled, the controller groups interfaces from multiple matched PFs into a single DRA device. The interfaces are passed through to the pod as separate network interfaces, allowing applications (e.g., DPDK) to handle bonding/aggregation at the application level. Note that this does not create kernel-level bonding interfaces inside the pod.

## Configuring physical interfaces for direct attachment to application pods

### Prerequisites for DPDK applications

Before configuring UnderlayNetwork resources, you must prepare the cluster's worker nodes for DPDK applications:

* Configure [hugepages](#configuring-hugepages).
* Configure [Topology Manager](#configuring-topology-manager).

#### Configuring hugepages

DPDK applications require hugepages for efficient memory management. Configure hugepages on all worker nodes using NodeGroupConfiguration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: hugepages-for-dpdk
spec:
  nodeGroups:
    - "*"  # Apply to all node groups.
  weight: 100
  content: |
    #!/bin/bash
    echo "vm.nr_hugepages = 4096" > /etc/sysctl.d/99-hugepages.conf
    sysctl -p /etc/sysctl.d/99-hugepages.conf
```

This configuration sets `vm.nr_hugepages = 4096` on all nodes, providing 8 GiB of hugepages (4096 pages × 2 MiB per page).

#### Configuring Topology Manager

For optimal performance, enable Topology Manager on NodeGroups of worker nodes where DPDK applications will run. This ensures that CPU, memory, and device resources are allocated from the same NUMA node.

Example NodeGroup configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  kubelet:
    topologyManager:
      enabled: true
      policy: SingleNumaNode
      scope: Container
  nodeType: Static
```

For more information, see:

* [topologyManager.enabled](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled)
* [topologyManager.policy](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy).

### Prerequisites

Before creating an `UnderlayNetwork`, ensure that:

1. Physical network interfaces (NICs) are available on the nodes and are discovered as NodeNetworkInterface resources.
1. The interfaces you plan to use are Physical Functions (PF), not Virtual Functions (VF).
1. For Shared mode, the NICs must support SR-IOV.

### Preparing NodeNetworkInterface resources

First, check which Physical Functions are available on your nodes:

```shell
d8 k get nni -l network.deckhouse.io/nic-pci-type=PF
```

Example output:

```console
NAME                            MANAGEDBY   NODE           TYPE   IFNAME   IFINDEX   STATE   VF/PF   Binding   Driver      Vendor   AGE
worker-01-nic-0000:17:00.0      Deckhouse   worker-01     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-01-nic-0000:17:00.1      Deckhouse   worker-01     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.0      Deckhouse   worker-02     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.1      Deckhouse   worker-02     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
```

Label the interfaces that will be used for UnderlayNetwork:

```shell
d8 k label nni worker-01-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-01-nic-0000:17:00.1 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.1 nic-group=dpdk
```

{% alert level="info" %}
You can check the PCI information and SR-IOV support status for each interface:

```shell
d8 k get nni worker-01-nic-0000:17:00.0 -o json | jq '.status.nic.pci.pf'
```

Look for `status.nic.pci.pf.sriov.supported` to verify SR-IOV support.
{% endalert %}

### Creating UnderlayNetwork in Dedicated mode

In Dedicated mode, each Physical Function is exposed as an exclusive device. This mode is suitable when:

* SR-IOV is not available or not needed.
* Each pod needs exclusive access to a complete PF.

Example configuration:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: dpdk-dedicated-network
spec:
  mode: Dedicated
  autoBonding: false
  memberNodeNetworkInterfaces:
    - labelSelector:
        matchLabels:
          nic-group: dpdk
```

When `autoBonding` is set to `true`, all matched PFs on a node are grouped into a single DRA device, exposing all PFs to the pod as separate interfaces. When `false`, each PF is published as a separate DRA device.

Check the status of the created UnderlayNetwork:

```shell
d8 k get underlaynetwork dpdk-dedicated-network -o yaml
```

Example status of UnderlayNetwork in Dedicated mode:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: dpdk-dedicated-network
...
status:
  observedGeneration: 1
  conditions:
  - message: All 2 member node network interface selectors have matches
    observedGeneration: 1
    reason: AllInterfacesAvailable
    status: "True"
    type: InterfacesAvailable
```

### Creating UnderlayNetwork in Shared mode

In Shared mode, Virtual Functions (VF) are created from Physical Functions (PF) using SR-IOV, allowing multiple pods to share the same hardware. This mode requires SR-IOV support on the NICs.

Example configuration:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: dpdk-shared-network
spec:
  mode: Shared
  autoBonding: true
  memberNodeNetworkInterfaces:
    - labelSelector:
        matchLabels:
          nic-group: dpdk
  shared:
    sriov:
      enabled: true
      numVFs: 8
```

In this example:

* `mode: Shared` enables SR-IOV and VF creation.
* `autoBonding: true` groups one VF from each matched PF into a single DRA device.
* `shared.sriov.enabled: true` enables SR-IOV on selected PFs.
* `shared.sriov.numVFs: 8` creates 8 Virtual Functions per Physical Function.

{% alert level="warning" %}
The `mode` and `autoBonding` fields are immutable once set. Plan your configuration carefully before creating the resource.
{% endalert %}

After creating the UnderlayNetwork, monitor the SR-IOV configuration status:

```shell
d8 k get underlaynetwork dpdk-shared-network -o yaml
```

Example status of UnderlayNetwork in Shared mode:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: UnderlayNetwork
metadata:
  name: dpdk-shared-network
...
status:
  observedGeneration: 1
  sriov:
    supportedNICs: 4
    enabledNICs: 4
  conditions:
  - lastTransitionTime: "2025-01-15T10:30:00Z"
    message: SR-IOV configured on 4 NICs
    reason: SRIOVConfigured
    status: "True"
    type: SRIOVConfigured
  - lastTransitionTime: "2025-01-15T10:30:05Z"
    message: Interfaces are available for allocation
    reason: InterfacesAvailable
    status: "True"
    type: InterfacesAvailable
```

You can verify that VFs have been created by checking NodeNetworkInterface resources:

```shell
d8 k get nni -l network.deckhouse.io/nic-pci-type=VF
```

### Preparing namespaces for UnderlayNetwork usage

Before users can request UnderlayNetwork devices in their pods, the namespace must be labeled to enable UnderlayNetwork support. This is an administrative task that should be done for namespaces where DPDK applications will run:

```shell
d8 k label namespace mydpdk direct-nic-access.network.deckhouse.io/enabled=""
```
