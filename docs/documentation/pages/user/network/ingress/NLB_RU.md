---
title: "NLB"
permalink: ru/user/network/ingress/nlb.html
lang: ru
---

<!-- переработать -->

NLB обеспечивается за счет использования сервисов с типом `LoadBalancer`.

## Примеры настроек для Service

Для создания Services с общими IP адресами необходимо добавить к ним аннотацию `network.deckhouse.io/load-balancer-shared-ip-key`:

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

Для создания Service с принудительно выбранным адресом необходимо добавить аннотацию `network.deckhouse.io/load-balancer-ips`:

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

Создание Service и назначение ему IPAddressPools возможно в режиме BGP LoadBalancer через аннотацию `metallb.universe.tf/address-pool`. Для режима L2 LoadBalancer необходимо использовать настройки MetalLoadBalancerClass (см. выше).

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx
  annotations:
    metallb.universe.tf/address-pool: production-public-ips
spec:
  ports:
  - port: 80
    targetPort: 80
  selector:
    app: nginx
  type: LoadBalancer
```
