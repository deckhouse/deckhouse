---
title: "The kube-dns module"
description: "Managing DNS in a Kubernetes cluster using CoreDNS."
---

The module installs CoreDNS components for managing DNS in the Kubernetes cluster.

> **Caution!** The module deletes all the previously installed kubeadm Deployments, ConfigMaps as well as RBAC for CoreDNS.
