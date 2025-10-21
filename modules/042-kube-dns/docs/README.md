---
title: "The kube-dns module"
description: "Managing DNS in a Kubernetes cluster using CoreDNS."
---

The module installs CoreDNS components for managing DNS in the Kubernetes cluster.

> **Caution!** The module deletes all the previously installed kubeadm Deployments, ConfigMaps as well as RBAC for CoreDNS. When deploying your own CoreDNS, avoid using the names `coredns` or `system:coredns` for any resources (Deployment, Service, ConfigMap, ServiceAccount, ClusterRole, ClusterRoleBinding). Use alternative names like `infra-dns` to prevent automatic removal by Deckhouse Kubernetes Platform.
