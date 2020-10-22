---
title: "Модуль openvpn: примеры конфигурации"
---

## Пример для кластеров bare metal

```
openvpnEnabled: "true"
openvpn: |
  inlet: ExternalIP
  externalIP: 5.4.54.4
```

## Пример для AWS и Google Cloud

```
openvpnEnabled: "true"
openvpn: |
  inlet: LoadBalancer
```

## Пример для публичного IP на внешнем балансировщике
```
openvpnEnabled: "true"
openvpn: |
  externalHost: 5.4.54.4
  externalIP: 192.168.0.30 # Внутренний IP, который примет трафик от внешнего балансировщика
  inlet: ExternalIP
  nodeSelector:
    kubernetes.io/hostname: node
```
