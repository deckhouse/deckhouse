---
title: "Модуль openvpn: примеры"
description: "Примеры конфигурации модуля openvpn Deckhouse Kubernetes Platform для различных сценариев, включая кластеры bare metal, AWS, Google Cloud и публичные IP-адреса на внешнем балансировщике."
---

## Пример для кластеров bare metal

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    inlet: ExternalIP
    externalIP: 5.4.54.4
```

## Пример для AWS и Google Cloud

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    inlet: LoadBalancer
```

## Пример для публичного IP-адреса на внешнем балансировщике

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: openvpn
spec:
  version: 2
  enabled: true
  settings:
    externalHost: 5.4.54.4
    externalIP: 192.168.0.30 # Внутренний IP-адрес, который примет трафик от внешнего балансировщика.
    inlet: ExternalIP
    nodeSelector:
      kubernetes.io/hostname: node
```
