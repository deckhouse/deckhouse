---
title: "Модуль ingress-nginx: пример"
---

{% raw %}

## Пример для AWS (Network Load Balancer)

При создании балансера будут использованы все доступные в кластере зоны.

В каждой зоне балансер получает публичный IP. Если в зоне есть инстанс с Ingress-контроллером, A-запись с IP-адресом балансера из этой зоны автоматически добавляется к доменному имени балансера.

Если в зоне не остается инстансов с Ingress-контроллером, тогда IP автоматически убирается из DNS.

В случае если в зоне всего один инстанс с Ingress-контроллером, при перезапуске Pod'а, IP-адрес балансера этой зоны будет временно исключен из DNS.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
```

## Пример для GCP / Yandex Cloud / Azure

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
 name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
```

**Внимание!** В GCP на узлах должна присутствовать аннотация, разрешающая принимать подключения на внешние адреса для сервисов с типом NodePort.

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

## Пример для Bare metal (При использовании внешнего балансировщика, например Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и др.)

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
    behindL7Proxy: true
```

## Пример для Bare metal (MetalLB Load Balancer)

Модуль `metallb` на текущий момент доступен только в редакции Enterprise Edition Deckhouse.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    value: frontend
```

В случае использования MetalLB, его speaker Pod'ы должны быть запущены на тех–же узлах, что и Pod'ы Ingress–контроллера.

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создаётся с параметром `externalTrafficPolicy: Local` (запрещая меж узловой SNAT), и для удовлетворения данного параметра MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые Pod'ы.

Таким образом для данного примера [конфигурация модуля metallb](../380-metallb/configuration.html) должна быть такой:

```yaml
metallb:
 speaker:
   nodeSelector:
     node-role.deckhouse.io/frontend: ""
   tolerations:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: frontend
```

{% endraw %}
