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
## Пример для AWS (Network Load Balancer), Ingress-узлы находятся не во всех зонах

В таком случае нужно указать аннотацию, в которой перечислены все идентификаторы подсетей, где необходимо создать Listener'ы. Подсети должны соответствовать зонам, где находятся Ingress-узлы.
Список текущих подсетей, которые используются для конкретной инсталляции можно получить так: `kubectl -n d8-system exec  deckhouse-94c79d48-lxmj5 -- deckhouse-controller module values cloud-provider-aws -o json | jq -r '.cloudProviderAws.internal.zoneToSubnetIdMap'`.

**Внимание!** Добавление аннотации на существующий Service не сработает, необходимо будет его пересоздать.

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
      service.beta.kubernetes.io/aws-load-balancer-subnets: "subnet-foo, subnet-bar"
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
