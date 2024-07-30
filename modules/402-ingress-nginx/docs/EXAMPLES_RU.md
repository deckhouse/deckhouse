---
title: "Модуль ingress-nginx: пример"
---

{% raw %}

## Пример для AWS (Network Load Balancer)

При создании балансировщика будут использованы все доступные в кластере зоны.

В каждой зоне балансировщик получает публичный IP. Если в зоне есть инстанс с Ingress-контроллером, A-запись с IP-адресом балансировщика из этой зоны автоматически добавляется к доменному имени балансировщика.

Если в зоне не остается инстансов с Ingress-контроллером, тогда IP автоматически убирается из DNS.

В том случае, если в зоне всего один инстанс с Ingress-контроллером, при перезапуске пода IP-адрес балансировщика этой зоны будет временно исключен из DNS.

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

> **Внимание!** В GCP на узлах должна присутствовать аннотация, разрешающая принимать подключения на внешние адреса для сервисов с типом NodePort.

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

## Пример для bare metal

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

## Пример для bare metal (при использовании внешнего балансировщика, например Cloudflare, Qrator, Nginx+, Citrix ADC, Kemp и др.)

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

## Пример для bare metal (балансировщик MetalLB в режиме BGP)

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

В случае использования MetalLB его speaker-поды должны быть запущены на тех же узлах, что и поды Ingress–контроллера.

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создается с параметром `externalTrafficPolicy: Local` (запрещая межузловой SNAT), и для удовлетворения данного параметра MetalLB speaker анонсирует этот Service только с тех узлов, где запущены целевые поды.

Таким образом, для данного примера [конфигурация модуля `metallb`](../380-metallb/configuration.html) должна быть такой:

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

## Пример для bare metal (балансировщик L2LoadBalancer)

Модуль [l2-load-balancer](../381-l2-load-balancer/) на текущий момент доступен только в редакции Enterprise Edition Deckhouse.

Включите модуль:
```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: l2-load-balancer
spec:
  enabled: true
  version: 1
```

Создайте ресурс _L2LoadBalancer_:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: L2LoadBalancer
metadata:
  name: ingress
spec:
  addressPool:
  - 192.168.2.100-192.168.2.150
  nodeSelector:
    node-role.kubernetes.io/loadbalancer: "" # селектор узлов-балансировщиков
```

Создайте ресурс _IngressNginxController_: 
* В аннотации __network.deckhouse.io/l2-load-balancer-name__ указывается имя _L2LoadBalancer_ (в примере _L2LoadBalancer_ с именем _ingress_ был создани на предыдущем шаге)
* Аннотация __network.deckhouse.io/l2-load-balancer-external-ips-count__ указывает сколько адресов будет выделено из пула, описанного в _L2LoadBalancer_

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
      network.deckhouse.io/l2-load-balancer-name: ingress
      network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
```

{% endraw %}
