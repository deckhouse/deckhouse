---
title: Planning for a high pod count on a node
permalink: en/guides/high-pod-density.html
description: Recommendations for preparing Deckhouse Kubernetes Platform nodes to run a large number of pods (hundreds and thousands per node).
lang: en
layout: sidebar-guides
---

When you plan to place and run more than 100 pods on a single node, the node has additional requirements. Disk (unpacking and mounting image layers), kernel, and memory are especially important. The recommendations below help you achieve more predictable pod startup times or speed them up.

{% alert %}
For nodes with high pod density:

- plan the node subnet size in advance ([podSubnetNodeCIDRPrefix](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix)) — the pod limit per node will adjust automatically;
- use a Linux kernel version 6.12 or newer;
- use a fast local disk (NVMe/SSD); do not use a network file system;
- prepare lightweight application images (minimum layers, for example, `distroless`).
{% endalert %}

For such scenarios, Deckhouse Kubernetes Platform configures control plane components automatically — no additional manual configuration is required.

## Pod limit per node

{% alert %}
The pod limit per node in Deckhouse Kubernetes Platform is calculated based on the node subnet size — the [podSubnetNodeCIDRPrefix](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix) parameter. To fit up to 1000 pods on a node, choose `podSubnetNodeCIDRPrefix` ≤ `21` when deploying the cluster.
{% endalert %}

The subnet size allocated to a node determines the pod limit per node:

| `podSubnetNodeCIDRPrefix` | Pods per node (default) |
|---|---|
| ≥ `24` | 120 |
| `23` | 250 |
| `22` | 500 |
| ≤ `21` | 1000 |

The `podSubnetNodeCIDRPrefix` parameter is set when the cluster is deployed. If you already know that you will need high pod density, choose an appropriate value from the start.

## Node kernel

{% alert %}
For nodes with a high pod count, use a Linux kernel version 6.12 or newer with support for the `erofs` kernel module.
{% endalert %}

Starting with version `2.x`, `containerd` supports `EROFS` functionality that depends on the `CONFIG_EROFS_FS_BACKED_BY_FILE` kernel option. This option is enabled by default starting with Linux 6.12 and allows mounting image layers represented as regular files directly, without creating a separate loop device for each layer or container.

Without this option, starting a large number of containers can create many loop devices. On nodes with hundreds or thousands of pods, this increases system load and can significantly slow down container startup.

Therefore, for nodes where you plan to run a large number of pods, choose an OS image with kernel 6.12 or newer.

## Node disk

{% alert %}
Use a fast local disk (NVMe or SSD). Do not place container runtime files and `kubelet` data on a network file system.
{% endalert %}

Starting each pod involves unpacking and working with image layers — disk-intensive operations. During mass startup, these operations are multiplied by the number of pods, so disk performance directly affects startup time.

- The baseline recommendation for nodes is fast disks with performance of at least 400+ IOPS (see the [Resource requirements](./production.html#resource-requirements) section). For nodes with high pod density, choose noticeably more performant disks — preferably local NVMe.
- Do not use a network or distributed file system for container runtime and `kubelet` directories: network latency on mount operations is multiplied by the number of pods and makes startup time unpredictable.

## Application images

{% alert %}
The smaller and more efficient the image, the faster the pod starts. Use minimal images (for example, `distroless`) and reduce the number of layers.
{% endalert %}

Image pull time and disk load at startup depend on image size and structure. With a large number of pods, this can become a problem:

- use minimal base images (for example, `distroless`, `*-slim`, or Alpine Linux), without unnecessary packages and tools;
- reduce the number of image layers and merge them when possible to decrease the amount of unpacking and mounting;
- keep images small — remove package manager caches, build artifacts, and temporary files;
- use multi-stage builds.

Pod startup delays can also be caused not by the image itself, but by the container registry and the network path to it: during mass startup (or rollout) of many applications, a node may download several images at once, and the registry together with the network to it can become a bottleneck. We recommend:

- placing the container registry close to the cluster in network terms and ensuring sufficient bandwidth to it;
- for images from external (including public) container registries, use a local mirror or pull-through cache near the cluster.

## Node resources

In addition to disk and kernel, pod density is limited by the node's CPU and memory. Example test workload (pod — static Nginx):

- a node with 4 CPUs / 8 GB RAM and an SSD reliably handles scaling from zero to 150 pods at once;
- a node with 32 CPUs / 64 GB RAM, local NVMe, and kernel ≥ 6.12 starts about 1000 pods in a few minutes.

Actual values depend on application characteristics, so we recommend load testing before moving to a production environment.
