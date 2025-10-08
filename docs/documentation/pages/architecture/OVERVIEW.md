---
title: Kubernetes Cluster
permalink: en/architecture/
---

Deckhouse makes it possible to run the Kubernetes cluster on **any supported infrastructure** and in the **same manner**:

- on clouds (for more info, see the section for the specific cloud provider);
- on virtual or bare metal machines (including on-premises);
- on a hybrid infrastructure.

Deckhouse Kubernetes Platform automatically configures and manages both the [cluster nodes](/modules/node-manager/) and the  [control plane](/modules/control-plane-manager/) components, keeping their configuration up-to-date (using Terraform tools).

Deckhouse facilitates non-trivial operations with control-plane and cluster nodes, such as:

- migrating between single-master and multi-master schemes;
- scaling master nodes;
- updating versions of the components.

All these tasks are based on smart and safe algorithms (the user can monitor/manage the ongoing processes).

Also, Deckhouse configures kubelet and takes care of the certificates used when working with the control plane. It automatically issues certificates and renews them.

Deckhouse replaces `kubeadm`'s `kube-proxy` resources (DaemonSets, ConfigMaps, RBAC) by their tailor-made analogs.

A high level of integration between Deckhouse modules ensures effective monitoring and provides an acceptable level of security. For example, you can safely access the cluster's API server from a public IP address and use an external authentication provider.

Images of all Deckhouse components (including `control plane`) are stored in a highly available and geo-distributed container registry. The latter is accessible from a limited set of IP addresses (to ease access from isolated environments).

From an architectural perspective, Deckhouse consists of the Deckhouse operator and modules. A module is a bundle of Helm chart, [Addon-operator](https://github.com/flant/addon-operator/) hooks, commands for building module components (Deckhouse components) and other files.

Deckhouse uses [addon-operator](https://github.com/flant/addon-operator/) when working with modules. Please refer to its documentation to learn how Deckhouse works with [modules](https://github.com/flant/addon-operator/blob/main/docs/src/MODULES.md), [module hooks](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md) and [module parameters](https://github.com/flant/addon-operator/blob/main/docs/src/VALUES.md). We would appreciate it if you *star* the project.
