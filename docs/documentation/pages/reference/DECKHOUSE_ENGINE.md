---
title: Deckhouse Engine
permalink: en/reference/deckhouse-engine.html
search: Deckhouse Engine, systemType, DeckhouseEngine, NodeConfig, nodelet, immutable node, dm-verity, type-1 hypervisor
description: What Deckhouse Engine is, how a node is built, and how it receives its configuration, gets updated and is maintained.
relatedLinks:
  - title: Installing a bare-metal cluster with Deckhouse Engine
    url: ../installing/deckhouse-engine.html
  - title: Deckhouse Engine node architecture
    url: ../architecture/cluster-and-infrastructure/node-management/deckhouse-engine.html
---

Deckhouse Engine is a type-1 hypervisor for Deckhouse Platform nodes. The machine boots
directly into Deckhouse Engine, without an intermediate general-purpose system. The node root is read-only
and is verified on every read. There is no package manager, `sshd` or shell on the node.
The node is managed by the `nodelet` agent: it brings the node to the state described in the
[NodeConfig](/modules/node-manager/cr.html#nodeconfig) resource and reports the result in its status.

Deckhouse Engine nodes are combined into groups in the same way as other cluster nodes. A group is marked by the
[`spec.systemType: DeckhouseEngine`](/modules/node-manager/cr.html#nodegroup-v1-spec-systemtype)
parameter of the NodeGroup resource. Groups with `systemType: GenericOS` or without this parameter consist of
nodes configured by [bashible](../architecture/cluster-and-infrastructure/node-management/bashible.html).

## Differences between Deckhouse Engine and GenericOS nodes

The node system type determines how you configure, update, and access the node.

| | GenericOS | DeckhouseEngine |
|---|---|---|
| What configures the node | bashible, step by step on the node | `nodelet`, from the NodeConfig document |
| Root | mutable, distribution packages | erofs image, dm-verity verification, read-only |
| Kubernetes components | registry-packages packages | signed system extensions (systemd-sysext) |
| Update | with packages, in place | by replacing the root image, with rollback |
| Access to the node | SSH | cluster API, console, maintenance portal |
| [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration) | runs | does not run |

## Update

The system image is updated as a whole. When the image digest changes in NodeConfig, `nodelet`
downloads the new image, verifies its signature, and reboots the node with the new image.
If the node fails to confirm a successful boot, initramfs boots the previous image.

Deckhouse Engine images are shipped with Deckhouse Platform as images of the [`node-manager`](/modules/node-manager/) module, and
the image version matches the [platform release](../release-notes.html). Downgrading the image is supported in the same way
as upgrading.

## Node requirements

Check the machine hardware and network access before installing Deckhouse Engine.

- x86-64 architecture, UEFI boot. Legacy BIOS is not supported.
- System disk of at least 20 GiB.
- To place etcd on a separate disk, provide a blank disk of at least 10 GiB. On a single-disk master node, etcd uses the `DATA` partition.
- Access from the machine to the cluster API and to the Deckhouse container image registry.
- Network access to TCP port `50000` from the installer host or the bastion used to add the machine.
  During cluster installation, `dhctl` also connects to TCP port `50001` on the first master node.

## Diagnostics without logging in to the node

You cannot log in to a Deckhouse Engine node over SSH. The node state is visible:

- in the status of the NodeConfig resource: conditions for each subsystem (disk mounts, containerd,
  kubelet, the image, and others) and the node phase;
- on the machine console (monitor and serial port): the `nodelet` screen with the node state,
  the maintenance portal address and network interfaces, and the full log on the serial
  console;
- in the node logs and files via a debug pod: `d8 k debug node/<NODE_NAME> -it --image=<IMAGE>`.

If the node has lost connection to the cluster, it has a maintenance portal at
`https://<NODE_ADDRESS>:50000`. Log in with the one-time token from `status.maintenanceToken`
of the NodeConfig resource (the `nodeconfigs/sensitive` permission is required). You can change anything through the portal
only if the node has not received a response from the cluster API for more than a minute: for example, temporarily
fix the network or upload a corrected NodeConfig.

## Limitations

Consider the following limitations when choosing how to install and configure nodes:

- A Deckhouse Engine node cannot be configured with the NodeGroupConfiguration resource: everything the node needs
  is set via NodeConfig.
- `systemType: DeckhouseEngine` cannot be removed from a group or replaced with `GenericOS`. A group
  can become `DeckhouseEngine` only while it has no nodes configured by bashible.
- Installing the first master nodes from a bundle (the `Local` mode of the [registry](/modules/registry/) module)
  is not supported. For an air-gapped environment, use the registry module in `Managed` mode
  with an image registry inside the perimeter, populated with `d8 mirror push`.
