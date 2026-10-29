---
title: Installation into an existing cluster
description: An installation mode in which Deckhouse Platform runs on top of an existing Kubernetes cluster and installs and updates platform modules, but does not manage the operating system, control plane or node lifecycle.
lang: en
---

In this mode the existing infrastructure keeps managing the operating system, kubelet, containerd, etcd, the control plane and Kubernetes updates, the existing CNI provides pod networking, and machines are created and deleted by an external provisioner or the user. The [module bundle](glossary.html#module-bundle) `Managed` is selected during installation and cannot be changed afterwards. [ClusterConfiguration](reference/api/cr.html#clusterconfiguration) is not required in the [installation configuration file](glossary.html#installation-config).

For details, see [Installing Deckhouse Platform in an existing Talos cluster](guides/talos-existing-cluster.html).
