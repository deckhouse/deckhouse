---
title: "Модуль node-local-dns: примеры"
---

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

[Подробнее](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#pod-s-dns-config) про настройку DNS.

## Пример конфигурации

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: node-local-dns
spec:
  version: 1
  enabled: true
  settings:
    enableLogs: true
```
