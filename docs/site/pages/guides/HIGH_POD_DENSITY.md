---
title: Planning for a high pod count on a node
permalink: en/guides/high-pod-density.html
description: Recommendations for preparing Deckhouse Kubernetes Platform nodes to run a large number of pods (hundreds and thousands per node).
lang: en
layout: sidebar-guides
---

When you plan to place and run more than 100 pods on a single node, a number of additional requirements apply to its configuration. The most critical requirements apply to the disk (unpacking and mounting image layers), kernel, and RAM. Following the recommendations in this guide helps reduce pod startup time and make it more predictable.

{% alert %}
For nodes with high pod density:

- Plan the node subnet size in advance (via [`podSubnetNodeCIDRPrefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix)) — the pod limit per node will adjust automatically.
- Use a Linux kernel version 6.12 or newer.
- Use a fast local disk (NVMe or SSD) and do not use a network file system.
- Use lightweight application images with minimum amount of layers (for example, `distroless`).
{% endalert %}

For such scenarios, Deckhouse Kubernetes Platform (DKP) configures control plane components automatically — no additional manual configuration is required.

## Pod limit per node

{% alert %}
The pod limit per node in DKP is calculated based on the node subnet size set via the [`podSubnetNodeCIDRPrefix`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-podsubnetnodecidrprefix) parameter. To fit up to 1000 pods on a node, set the value ≤ `21` when deploying the cluster.
{% endalert %}

The subnet size allocated to a node determines the maximum number of pods that can be allocated to a node:

| Value of `podSubnetNodeCIDRPrefix` | Pods per node (by default) |
|---|---|
| ≥ `24` | 120 |
| `23` | 250 |
| `22` | 500 |
| ≤ `21` | 1000 |

The `podSubnetNodeCIDRPrefix` parameter is set when the cluster is deployed. If you know that you will be allocating a large number of pods on nodes, it is recommended that you set an appropriate value when the cluster is created.

## Node kernel

{% alert %}
For nodes with a high pod count, it is recommended that you use a Linux kernel version 6.12 or newer with support for the `erofs` kernel module.
{% endalert %}

Starting with version 2.x, containerd supports `EROFS` functionality that depends on the `CONFIG_EROFS_FS_BACKED_BY_FILE` kernel option. This option is enabled by default starting with Linux 6.12 and allows mounting image layers represented as regular files directly, without creating a separate loop device for each layer or container.

Without this option, starting a large number of containers can create many loop devices. On nodes with hundreds or thousands of pods, this increases system load and can significantly slow down container startup.

Therefore, for nodes where you plan to run a large number of pods, it is recommended that you use an operating system with a Linux kernel 6.12 or newer.

## Node disk

{% alert %}
Use a fast local disk (NVMe or SSD). Do not place container runtime files and kubelet data on a network file system.
{% endalert %}

Starting each pod involves disk-intensive operations of unpacking and working with image layers. During a mass startup, these operations are multiplied by the number of pods, so disk performance directly affects startup time.

Recommendations:

- Use fast disks with performance of at least 400+ IOPS (see the ["Resource requirements"](./production.html#resource-requirements) section). For nodes with high pod density, prioritize local NVMe disks.
- Do not place container runtime and kubelet directories in a network or distributed file system. Network latency on mount operations is multiplied by the number of started pods and makes startup time unpredictable.

## Application images

{% alert %}
The more compact and simple the application image, the faster the pod starts. Use minimal images (for example, `distroless`) and reduce the number of layers.
{% endalert %}

Image size and structure affect its pull and preparation time. With a large number of simultaneously started pods, this can become a problem.

Recommendations:

- Use minimal base images (for example, `distroless`, `*-slim`, or `alpine`), without unnecessary packages and tools.
- Reduce the number of image layers and merge them when possible to decrease the amount of unpacking and mounting.
- Keep images small — remove package manager caches, build artifacts, and temporary files.
- Use multi-stage builds.

Pod startup delays can also be caused not by the image itself, but by the container registry and the network bandwidth to it. During a mass startup or update of multiple applications, a node may download several images at once, which can mane the registry a bottleneck.

Recommendations:

- Place the container registry close to the cluster in network terms and ensure sufficient bandwidth to it.
- For images from external (including public) container registries, use a local mirror or pull-through cache near the cluster.

## Node resources

In addition to disk and kernel, pod density is limited by the node's CPU and RAM. The following are test results where a static NGINX is used:

- A node with 4 vCPU, 8 GB RAM, and an SSD ensures a simultaneous startup of 150 pods.
- A node with 32 vCPU, 64 GB RAM, local NVMe, and a Linux kernel ≥ 6.12 starts about 1000 pods in a few minutes.

Actual values depend on application characteristics, so it is recommended that you conduct a load testing before using similar configurations in a production environment.
