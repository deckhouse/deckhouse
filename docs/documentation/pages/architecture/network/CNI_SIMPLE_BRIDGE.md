---
title: Cni-simple-bridge module
permalink: en/architecture/network/cni-simple-bridge.html
search: cni-simple-bridge, bridge, host-local
description: Architecture of the cni-simple-bridge module in Deckhouse Platform.
---

The [`cni-simple-bridge`](/modules/cni-simple-bridge/) module provides basic networking for pods in Deckhouse Platform and implements the [Container Network Interface (CNI)](https://github.com/containernetworking/CNI).

{% alert level="info" %}
The `cni-simple-bridge` CNI plugin was used by default for the [`cloud-provider-aws`](/modules/cloud-provider-aws/), [`cloud-provider-azure`](/modules/cloud-provider-azure/), [`cloud-provider-gcp`](/modules/cloud-provider-gcp/), and [`cloud-provider-yandex`](/modules/cloud-provider-yandex/) modules in Deckhouse platform versions earlier than 1.77.0. Starting with platform 1.77.0, these modules use Cilium CNI provided by the [`cni-cilium`](/modules/cni-cilium/) module by default.
{% endalert %}

The module has no configurable parameters.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cni-simple-bridge`](/modules/cni-simple-bridge/) module and its interactions with other components of Deckhouse Platform are shown in the following diagram:

![Cni-simple-bridge module architecture](../../images/architecture/network/c4-l2-cni-simple-bridge.svg)

## Module components

The module consists of the following components:

1. **Simple-bridge** (DaemonSet): Runs on every cluster node and prepares the CNI plugin for operation. To do this, it obtains the pod subnet from the `spec.podCIDR` field of the Node resource and builds the configuration for running the `bridge` executable, which containerd invokes on every creation or removal of a pod's network namespace. When invoked, `bridge` requests an IP address from the `host-local` IP Address Management ([IPAM](../../glossary.html#sdn-ipam)) plugin.

   `bridge` and `host-local` are standard [Open Source CNI plugins](https://github.com/containernetworking/plugins), installed on every cluster node independently of this module, as part of kubelet installation.

   It consists of the following containers:

   * **cni-migration-init-checker**: Init container waiting for the migration process from another CNI plugin to complete, if such a migration has been started.
   * **iptables-wrapper**: Init container that prepares executables required for working with iptables.
   * **simple-bridge**: Main container.

## Module interactions

The module interacts with the **Kube-apiserver** component:

- Reads Node resources.
- Watches [CNIMigration](../../reference/api/cr.html#cnimigration) and [CNINodeMigration](../../reference/api/cr.html#cninodemigration) custom resources.
