---
title: Static pod
description: A pod that kubelet starts directly from a manifest in the `/etc/kubernetes/manifests` directory of the node, without the Kubernetes API. Control plane components and kubernetes-api-proxy run this way in Deckhouse Platform.
lang: en
---

In Deckhouse Platform kubelet processes only files with the `.yaml` or `.yml` extension in this directory. Backup copies, editor swap files and other temporary files are ignored. The [control plane](glossary.html#control-plane) components kube-apiserver, etcd, kube-controller-manager and kube-scheduler and the [kubernetes-api-proxy](glossary.html#kubernetes-api-proxy) are static pods.

For details, see [Kubelet](architecture/kubernetes-and-scheduling/kubelet.html).
