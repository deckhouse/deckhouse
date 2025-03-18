---
title: "Managing DNS in a Kubernetes cluster"
permalink: en/admin/network/dns-management-in-cluster.html
---

DNS management in a Kubernetes cluster is implemented using the `kube-dns` module.

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/kube-dns/ -->

The module installs CoreDNS components for managing DNS in the Kubernetes cluster.

> The module deletes all the previously installed kubeadm Deployments, ConfigMaps as well as RBAC for CoreDNS.

<!-- Transferred with minor modifications from https://deckhouse.io/products/kubernetes-platform/documentation/latest/modules/kube-dns/ -->

## Configuration example

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: kube-dns
spec:
  version: 1
  enabled: true
  settings:
    upstreamNameservers:
    - 8.8.8.8
    - 8.8.4.4
    hosts:
    - domain: one.example.com
      ip: 192.168.0.1
    - domain: two.another.example.com
      ip: 10.10.0.128
    stubZones:
    - zone: consul.local
      upstreamNameservers:
      - 10.150.0.1
    enableLogs: true
    clusterDomainAliases:
    - foo.bar
    - baz.qux

```
