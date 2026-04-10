---
title: CSI driver
permalink: en/architecture/cluster-and-infrastructure/infrastructure/csi-driver.html
search: csi driver, csi-driver, container storage interface
description: Overview of the CSI driver architecture in Deckhouse Kubernetes Platform.
---

A CSI driver (plugin) is used to manage persistent storage volumes in Deckhouse Kubernetes Platform (DKP).

[Container Storage Interface (CSI)](https://github.com/container-storage-interface/spec/blob/master/spec.md) is a standard interface that unifies access to storage systems and simplifies integration of different storage systems into clusters.

The CSI driver is included in DKP modules named `cloud-provider-*`. Although each supported cloud provider or storage system uses its own implementation of the CSI specification, the architecture of the CSI driver is the same across all implementations. Implementations may differ in the set of supported capabilities and components.

The following is a description of the reference CSI driver architecture used in DKP. It includes all possible components and implements the full functionality used by DKP modules.

## Driver architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The reference CSI driver architecture at Level 2 of the C4 model and its interactions with other DKP components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Reference CSI driver architecture](../../../../images/architecture/cluster-and-infrastructure/c4-l2-csi-driver-common.png)

## Driver components

The CSI driver consists of the following components:

1. **Csi-controller** (Deployment): Controller Plugin responsible for global volume operations such as creating and deleting volumes, attaching and detaching volumes from nodes, and managing snapshots. For example, in AWS this component calls the EC2 API to create EBS volumes.

   It consists of the following containers:

   * **controller**: Main container implementing CSI driver functionality (capabilities) through the gRPC services Identity Service and Controller Service according to the [CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface).

   * **controller sidecar containers**: Kubernetes community-maintained external controllers.

     These controllers are required because the persistent volume controller running in kube-controller-manager (a component of the [DKP control plane](../../kubernetes-and-scheduling/control-plane.html)) does not provide an interface for direct interaction with CSI drivers. External controllers monitor PersistentVolumeClaim resources and call the corresponding CSI driver functions in the controller container. They also perform auxiliary tasks such as retrieving plugin information and capabilities or checking driver health (liveness probe).

     External controllers communicate with the controller container over gRPC via Unix sockets.

     Csi-controller includes the following external controllers:

     * **provisioner** ([external-provisioner](https://github.com/kubernetes-csi/external-provisioner)): Watches PersistentVolumeClaim resources and calls the RPC methods `CreateVolume` or `DeleteVolume`. It also uses `ValidateVolumeCapabilities` to verify compatibility.

     * **attacher** ([external-attacher](https://github.com/kubernetes-csi/external-attacher)): Monitors VolumeAttachment resources after a pod is scheduled to a node and attaches or detaches volumes using the RPC methods `ControllerPublishVolume` and `ControllerUnpublishVolume`.

     * **resizer** ([external-resizer](https://github.com/kubernetes-csi/external-resizer)): Watches updates to PersistentVolumeClaim resources and expands volumes using the `ControllerExpandVolume` RPC method when a user requests additional storage for a PVC, and the driver supports the `EXPAND_VOLUME` capability.

     * **snapshotter** ([external-snapshotter](https://github.com/kubernetes-csi/external-snapshotter)): Works together with the [`snapshot-controller`](/modules/snapshot-controller/) module, watches VolumeSnapshotContent resources, and manages volume snapshots using the RPC methods `CreateSnapshot`, `DeleteSnapshot`, and `ListSnapshots` (if supported by the driver).

     * [**livenessprobe**](https://github.com/kubernetes-csi/livenessprobe): Monitors the health of the CSI driver through the `Probe` RPC from the Identity Service and exposes the HTTP endpoint `/healthz`, which is checked by [kubelet](../../kubernetes-and-scheduling/kubelet.html). If *livenessProbe* fails, kubelet restarts the csi-controller pod.

2. **Csi-node** (DaemonSet): Node Plugin running on all cluster nodes and responsible for local volume mount and unmount operations.

   > **Warning.** The plugin has privileged access to the filesystem of each node. On Linux, this requires the `CAP_SYS_ADMIN` capability. This is necessary to perform mount operations and interact with block devices.

   It consists of the following containers:

   * **node**: Main container implementing CSI driver functionality through the gRPC services Identity Service and Node Service according to the [CSI specification](https://github.com/container-storage-interface/spec/blob/master/spec.md#rpc-interface).

   * **node-driver-registrar**: Sidecar container that registers the Node Plugin with [kubelet](../../kubernetes-and-scheduling/kubelet.html). It calls the RPC methods `GetPluginInfo` and `NodeGetInfo` in the node container to retrieve plugin and node information. Communication with the node container occurs over gRPC via a Unix socket.

{% alert level="info" %}
Some sidecar containers from the external controller list (for example, snapshotter) may be absent in the Deployment of specific `cloud-provider-*` modules if the corresponding functionality is not supported by the CSI driver implementation.
{% endalert %}

## Driver interactions

The driver interacts with the following components:

1. **Kube-apiserver**: Monitors PersistentVolumeClaim, VolumeAttachment, and VolumeSnapshotContent resources.

2. **Cloud infrastructure** (or a virtualization system): Creates and deletes volumes, attaches and detaches volumes from nodes, and manages snapshots.

The following external components interact with the driver:

1. [Kubelet](../../kubernetes-and-scheduling/kubelet.html):

   * Checks the CSI driver livenessProbe.
   * Registers the Node Plugin.
   * Calls the RPC methods `NodeStageVolume`, `NodeUnstageVolume`, `NodePublishVolume`, `NodeUnpublishVolume`, and `NodeExpandVolume` in Node Plugin.

   [Kubelet](../../kubernetes-and-scheduling/kubelet.html) communicates with Node Plugin via gRPC over a Unix socket.
