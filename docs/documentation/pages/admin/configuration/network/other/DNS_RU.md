---
title: "Управление DNS в кластере Kubernetes"
permalink: ru/admin/configuration/network/other/dns.html
lang: ru
---

Управление DNS в кластере Kubernetes реализуется с помощью модуля [`kube-dns`](/modules/kube-dns/).

<!-- Перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/kube-dns/ -->

Модуль устанавливает компоненты CoreDNS для управления DNS в кластере Kubernetes.

{% alert level="info" %}
Модуль удаляет объекты Deployment, ConfigMap и RBAC для CoreDNS, установленные через утилиту `kubeadm`.
{% endalert %}

<!-- Перенесено с небольшими изменениями из https://deckhouse.ru/products/kubernetes-platform/documentation/latest/modules/kube-dns/examples.html -->

## Пример конфигурации

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
