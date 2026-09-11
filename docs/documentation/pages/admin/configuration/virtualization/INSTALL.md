---
title: "Installing and updating the virtualization module"
permalink: en/admin/configuration/virtualization/install.html
description: "How to enable the virtualization module, place its components on cluster nodes, and update it through release channels."
search: install virtualization, enable module, component placement, module update
---

Virtualization is enabled with a ModuleConfig resource and deploys its components to the `d8-virtualization` namespace. The sections below cover how to enable it, how its components are placed on nodes, and how to update it.

## Installation

1. Deploy the Deckhouse Platform cluster following the [instructions](/products/kubernetes-platform/gs/).

1. To store virtual machine data (virtual disks and images), enable one or multiple [supported storages](requirements.html#supported-storage-systems).

1. Set the default `StorageClass`:

   ```shell
   # Specify the name of your StorageClass object.
   DEFAULT_STORAGE_CLASS=replicated-storage-class
   sudo -i d8 k patch mc global --type='json' -p='[{"op": "replace", "path": "/spec/settings/defaultClusterStorageClass", "value": "'"$DEFAULT_STORAGE_CLASS"'"}]'
   ```

1. Turn on the [`console`](/modules/console/) module, which will allow you to manage virtualization components through the Deckhouse web UI.

1. Enable the `virtualization` module:

   {% alert level="warning" %}
   Enabling the `virtualization` module involves restarting kubelet/containerd and cilium agents on all nodes where virtual machines are supposed to start. This is necessary to configure the connectivity of containerd and DVCR.
   {% endalert %}

   To enable the `virtualization` module, create a `ModuleConfig` resource with the module settings.

   {% alert level="warning" %}
   Before enabling the module, carefully review its settings in [Virtualization parameters](settings.html).
   {% endalert %}

   Example of module configuration:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: virtualization
   spec:
     enabled: true
     settings:
       dvcr:
         storage:
           persistentVolumeClaim:
             size: 50G
           type: PersistentVolumeClaim
       virtualMachineCIDRs:
         - 10.66.10.0/24
     version: 1
   EOF
   ```

   To check if the module is ready, use the following command:

   ```bash
   d8 k get modules virtualization
   ```

   Example output:

   ```txt
   NAME             WEIGHT   SOURCE      PHASE   ENABLED   READY
   virtualization   900      deckhouse   Ready   True      True
   ```

   The module phase should be `Ready`.

## Component placement across nodes

The distribution of components across cluster nodes depends on the cluster's configuration. For example, a cluster may consist of:

- Only master nodes, for running the control plane and workload components.
- Only master nodes and worker nodes.
- Master nodes, system nodes, and worker nodes.
- Other combinations (depending on the architecture).

{% alert level="warning" %}
In this context, worker nodes are nodes that don't have taints preventing regular workloads from running.
{% endalert %}

What each component is responsible for is described in [Virtualization subsystem](/products/kubernetes-platform/documentation/v1/architecture/virtualization/). The table below lists the components of the virtualization control plane and the nodes where they can be placed. Components are scheduled by priority, and if a suitable node type is available in the cluster, the component lands on it.

| Component name                | Node group        | Comment                                                                                                        |
|-------------------------------|-------------------|----------------------------------------------------------------------------------------------------------------|
| `virt-operator-*`             | system/master     |                                                                                                                |
| `virt-api-*`                  | master            |                                                                                                                |
| `virt-controller-*`           | system/worker     |                                                                                                                |
| `virt-handler-*`              | All cluster nodes |                                                                                                                |
| `virtualization-api-*`        | master            |                                                                                                                |
| `virtualization-controller-*` | master            |                                                                                                                |
| `dvcr-*`                      | system            | Storage must be available on the node. If there are no system nodes, the component is placed on a worker node. |
| `virtualization-audit-*`      | master            | Available in DP EE and Ultimate.                                                                               |
| `virtualization-dra-*`        | Selected nodes    | Available in commercial DP editions.                                                                           |
| `vm-route-forge-*`            | All cluster nodes |                                                                                                                |

The `virtualization-dra-*` component runs only on nodes labeled `virtualization.deckhouse.io/usbip`.

Components used to create and import virtual machine images or disks (they run only for the duration of the creation or import operation):

| Component name                                   | Node group    | Comment                                                                            |
|--------------------------------------------------|---------------|--------------------------------------------------------------------------------------|
| `d8v-vi-importer-*`, `d8v-cvi-importer-*`        | system/worker | Pulls an image from an external source or another resource into the image storage. |
| `d8v-vi-uploader-*`, `d8v-cvi-uploader-*`        | system/worker | Receives a file that you upload from the command line or the web interface.        |
| `d8v-vd-pvc-importer-*`, `d8v-vi-pvc-importer-*` | system/worker | Moves an image from the image storage onto a disk volume.                          |
| `d8v-pvc-pvc-source-importer-*`                  | system/worker | Serves the data of the source volume over the network when a disk is cloned.       |
| `d8v-pvc-pvc-target-importer-*`                  | system/worker | Receives the data on the target volume when a disk is cloned.                       |
| `d8v-vi-bounder-*`                               | system/worker | Holds the volume on the right node while the image is being created on it.         |

### Cluster with taints on all nodes

In some clusters, taints are configured on every node. This lets administrators explicitly control which nodes pods and virtual machines can be scheduled onto.

When running the `virtualization` module in this setup, keep the following in mind:

1. When creating a [VirtualDisk](/modules/virtualization/cr.html#virtualdisk), pay attention to the StorageClass `volumeBindingMode`. With `Immediate`, a PersistentVolume is created as soon as the disk is created — before the virtual machine is scheduled. Make sure the provisioner can create volumes on nodes that are allowed for virtual machines through [placement settings](../../../user/virtualization/vm-placement.html), including `nodeSelector`, `tolerations`, and settings in the virtual machine `spec` or [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass). Otherwise, the disk may end up on a node where the virtual machine cannot run. With `WaitForFirstConsumer`, the volume is created on the node where the virtual machine is scheduled, and this issue does not occur.

1. [VirtualImage](/modules/virtualization/cr.html#virtualimage) and [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage) require the temporary components from the table above. They have a toleration for the `dedicated.deckhouse.io=system` taint.

1. The cluster must have a `system` NodeGroup, or the administrator can add the `dedicated.deckhouse.io=system` taint to selected nodes without creating a NodeGroup. Without such nodes, these components will not be scheduled, and images will not reach the `Ready` phase.

## Module update

The `virtualization` module uses five update channels designed for use in different environments that have different requirements in terms of reliability:

| Update Channel | Description                                                                                                                                                                                                                                                        |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Alpha          | The least stable update channel with the most frequent appearance of new versions. It is oriented to development clusters with a small number of developers.                                                                                                       |
| Beta           | Focused on development clusters, like the Alpha update channel. Receives versions that have been pre-tested on the Alpha update channel.                                                                                                                           |
| Early Access   | Recommended update channel if you are unsure. Suitable for clusters where there is a lot of activity going on (new applications being launched, finalized, etc.). Functionality updates will not reach this update channel until one week after they are released. |
| Stable         | Stable update channel for clusters where active work is finished and mostly operational. Functionality updates to this update channel do not reach this update channel until two weeks after they appear in the release.                                           |
| Rock Solid     | The most stable update channel. Suitable for clusters that need a higher level of stability. Feature updates do not reach this channel until one month after they are released.                                                                                    |

The `virtualization` module components can be updated automatically or with manual confirmation, as updates are released in update channels.

{% alert level="warning" %}
When considering updates, the module components can be divided into two categories:

- Virtualization resource management components (the control plane).
- Virtual machine launch components (the firmware).

Updating control plane components does not affect the operation of already running virtual machines, but may cause a brief interruption of established VNC/serial port connections while the control plane component is restarted.

Updates to virtual machine firmware during a DP upgrade may require virtual machines to be migrated to the new "firmware" version.
DP migrates a machine once, and if the migration fails, the machine owner has to move or reboot it themselves.
{% endalert %}

For information on versions available at the update channels, see the [release channels site](https://releases.deckhouse.io/).
