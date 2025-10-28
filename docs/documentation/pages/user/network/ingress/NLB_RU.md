---
title: "NLB"
permalink: ru/user/network/ingress/nlb.html
lang: ru
---

<!-- переработать -->

NLB обеспечивается за счет использования сервисов с типом LoadBalancer.

## Примеры настроек для объектов Service

### Общий IP-адрес для нескольких сервисов

Для того, чтобы сервисы использовали одни и те же IP-адреса, добавьте к ним аннотацию `network.deckhouse.io/load-balancer-shared-ip-key`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: dns-service-tcp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnstcp
      protocol: TCP
      port: 53
      targetPort: 53
  selector:
    app: dns
---
apiVersion: v1
kind: Service
metadata:
  name: dns-service-udp
  namespace: default
  annotations:
    network.deckhouse.io/load-balancer-shared-ip-key: "key-to-share-1.2.3.4"
spec:
  type: LoadBalancer
  ports:
    - name: dnsudp
      protocol: UDP
      port: 53
      targetPort: 53
  selector:
    app: dns
```

### Принудительное назначение IP-адреса

Чтобы принудительно задать адрес для сервиса, добавьте аннотацию `network.deckhouse.io/load-balancer-ips`:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    network.deckhouse.io/load-balancer-ips: 192.168.217.217
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```

### Назначение IPAddressPool (режим BGP)

В режиме BGP LoadBalancer получение IP-адреса возможно из определённого пула адресов через аннотацию `metallb.io/address-pool`.
Для режима L2 LoadBalancer необходимо использовать настройки [MetalLoadBalancerClass](../../../admin/configuration/network/ingress/nlb/metallb.html#пример-использования-metallb-в-режиме-l2-loadbalancer).

Пример:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.io/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```
