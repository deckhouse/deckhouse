---
title: Deckhouse Engine node architecture
permalink: en/architecture/cluster-and-infrastructure/node-management/deckhouse-engine.html
description: Boot, disk layout, components, and configuration of Deckhouse Engine nodes.
---

The boot process verifies the system image. The disk layout determines which data persists across reboots.

## Boot and chain of trust

The node boots only via UEFI. The Unified Kernel Image (UKI) combines the kernel, the initramfs, and
the kernel command line in one signed file. The Deckhouse root certificate is built into the kernel,
and it is the only key the kernel trusts.

The same key signs:

- the root hash of the root image: the kernel verifies the signature before mounting the root;
- kernel modules: an unsigned module is not loaded;
- system extensions: an unsigned extension is not merged.

The root image is built in the erofs format and mounted via dm-verity. Every block read
is checked against a hash tree, so a change to the root on disk is detected on read, not
at the next check.

## Disk

The installer partitions the system disk. The partition sizes depend on whether the node boots from the ISO or a prebuilt qcow2 disk:

| Partition | ISO installation | qcow2 disk | Purpose |
|---|---|---|---|
| `BOOT` | 512 MiB | 120 MiB | UKI and two root image slots (A and B) |
| `CONFIG` | 10 MiB | 16 MiB | Node configuration and installed image metadata |
| `DATA` | Remainder of the disk | 512 MiB before first boot | `/var`: containers, images, kubelet state, and logs |

On the first boot from a qcow2 disk, `DATA` expands to fill the remaining space.

Additional disks, for example a separate etcd disk on a master node, are attached with the
`spec.storage.mounts` parameter of the NodeConfig resource.

## Writable directories

The root, including `/etc` and `/usr`, is read-only. Writing is possible in four places:

| Path | What it is | Survives a reboot |
|---|---|---|
| `/var` | the `DATA` partition | yes |
| `/run` | tmpfs | no |
| `/tmp` | tmpfs | no |
| `/mnt` | tmpfs, with partitions from `spec.storage.mounts` mounted on top | partitions: yes |

`/var/run` is a link to `/run`, so anything written to this path does not survive a
reboot.

## Node components

PID 1 is systemd. The set of units on the node is closed: only what is allowed in the root image
is started. containerd, kubelet and the other Kubernetes components are added as system
extensions (systemd-sysext). An extension is merged only if its signature is verified.
The cluster defines which extensions the node needs in NodeConfig.

## How a node receives its configuration

Each node stores its configuration in a NodeConfig document. The source depends on how the node is added:

- for the first master nodes, `dhctl bootstrap` writes it;
- for static nodes after the cluster is installed, the `node` plugin for Deckhouse CLI writes it;
- for cloud nodes, the cloud provider writes it via cloud-init.

After that, the cluster maintains the node's NodeConfig: the `node-controller` component of the `node-manager` module
recalculates it when NodeGroup and cluster parameters change. The machine part of the document
(disk, network) is set at installation and preserved.

Changes that interrupt the node (for example, a root image update with
a reboot) are applied by `nodelet` only after the cluster allows them, following the same
[`disruptions`](/modules/node-manager/cr.html#nodegroup-v1-spec-disruptions) rules as for
other nodes.

For setup instructions, see [Installing and adding nodes](../../../installing/deckhouse-engine.html).
