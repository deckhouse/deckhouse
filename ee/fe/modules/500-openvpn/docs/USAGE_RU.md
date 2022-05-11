---
title: "Модуль openvpn: примеры конфигурации"
---

## Пример для кластеров bare metal

```yaml
openvpnEnabled: "true"
openvpn: |
  inlet: ExternalIP
  externalIP: 5.4.54.4
```

## Пример для AWS и Google Cloud

```yaml
openvpnEnabled: "true"
openvpn: |
  inlet: LoadBalancer
```

## Пример для публичного IP-адреса на внешнем балансировщике
```yaml
openvpnEnabled: "true"
openvpn: |
  externalHost: 5.4.54.4
  externalIP: 192.168.0.30 # Внутренний IP-адрес, который примет трафик от внешнего балансировщика.
  inlet: ExternalIP
  nodeSelector:
    kubernetes.io/hostname: node
```
