---
title: "Кэширование DNS-запросов на узлах кластера"
permalink: ru/admin/configuration/network/other/dns-caching.html
lang: ru
---

В Deckhouse Kubernetes Platform можно развернуть локальный кэширующий DNS-сервер на каждом узле кластера. Он экспортирует метрики в Prometheus для визуализации в [дашборде Grafana](/modules/node-local-dns/#grafana-dashboard).

Функциональность реализуется модулем [`node-local-dns`](/modules/node-local-dns/). Модуль состоит из оригинального CoreDNS, разворачиваемого в DaemonSet на всех узлах кластера, с добавлением алгоритма настройки сети и правил iptables.

Подробная информация о проблемах, которые позволяет решить кэширующий DNS-сервер и о принципе его работы — в разделе [Кэширующий DNS-сервер в кластере](../../../../architecture/network/dns-caching.html).

<!-- Перенесено из https://deckhouse.ru/modules/node-local-dns/configuration.html -->

## Пример настройки кастомного DNS в поде

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: dns-example
spec:
  dnsPolicy: "None"
  dnsConfig:
    nameservers:
      - 169.254.20.10
  containers:
    - name: test
      image: nginx
```

Подробную информацию о настройке DNS можно найти [в документации Kubernetes](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config).
