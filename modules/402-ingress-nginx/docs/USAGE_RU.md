---
title: "Модуль ingress-nginx: пример конфигурации"
---

{% raw %}
## Общий пример
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: HostPort
  hostPort:
    httpPort: 80
    httpsPort: 443
  resourcesRequests:
    mode: VPA
    vpa:
      mode: Auto
      cpu:
        max: 100m
      memory:
        max: 200Mi
```

## Пример для AWS (Network Load Balancer)

При создании балансера будут использованы все доступные в кластере зоны.

В каждой зоне балансер получает публичный IP. Если в зоне есть инстанс с ingress-контроллером, A-запись с IP-адресом балансера из этой зоны автоматически добавляется к доменному имени балансера.

Если в зоне не остается инстансов с ingress-контроллером, тогда IP автоматически убирается из DNS.

В случае если в зоне всего один инстанс с ingress-контроллером, при перезапуске пода, IP-адрес балансера этой зоны будет временно исключен из DNS.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

## Пример для GCP
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: "nginx"
  inlet: "LoadBalancer"
```

## Пример для OpenStack
```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main-lbwpp
spec:
  inlet: LoadBalancerWithProxyProtocol
  ingressClass: nginx
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
```

## Пример для Bare metal

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: HostWithFailover
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```
{% endraw %}
