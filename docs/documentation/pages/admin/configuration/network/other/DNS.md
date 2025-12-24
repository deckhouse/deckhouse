---
title: "Managing DNS in a Kubernetes cluster"
permalink: en/admin/configuration/network/other/dns.html
---

DNS management in a Kubernetes cluster is implemented using the [`kube-dns`](/modules/kube-dns/) module.

The module installs CoreDNS components for managing DNS in the Kubernetes cluster.

{% alert level="info" %}
The module deletes Deployments, ConfigMaps as well as RBAC for CoreDNS that were previously created using the `kubeadm` tool. When deploying your own CoreDNS, avoid using the names `coredns` or `system:coredns` for any resources (Deployment, Service, ConfigMap, ServiceAccount, ClusterRole, ClusterRoleBinding). Use alternative names like `infra-dns` to prevent automatic removal by Deckhouse Kubernetes Platform.
{% endalert %}

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
