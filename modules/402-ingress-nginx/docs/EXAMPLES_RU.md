---
title: "Модуль ingress-nginx: примеры"
---

{% raw %}

## Пример для AWS (Network Load Balancer)

При создании балансировщика используются все доступные зоны в кластере.

В каждой зоне балансировщик получает публичный IP. Если в зоне есть instance с Ingress-контроллером, A-запись с IP-адресом балансировщика из этой зоны автоматически добавляется к доменному имени балансировщика.

Если в зоне не остается instance с Ingress-контроллером, тогда IP автоматически удаляется из DNS.

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

{% endraw %}

{% alert level="warning" %}
В GCP на узлах должна присутствовать аннотация, разрешающая принимать подключения на внешние адреса для сервисов с типом NodePort.
{% endalert %}

{% raw %}

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

{% endraw %}

## Пример для bare metal (балансировщик MetalLB в режиме BGP LoadBalancer)

{% alert level="warning" %}Доступно в следующих редакциях: EE, CSE Pro (1.67).{% endalert %}

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

Контроллер должен получать реальные IP-адреса клиентов — поэтому его Service создается с параметром `externalTrafficPolicy: Local` (запрещая межузловой SNAT), и для принятия данного параметра MetalLB speaker анонсирует этот Service только с тех узлов, в которых запущены целевые поды.

Для этого примера [конфигурация модуля `metallb`](../metallb/configuration.html) должна быть следующей:

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

## Пример для bare metal (балансировщик MetalLB в режиме L2 LoadBalancer)

{% alert level="warning" %}Доступно в следующих редакциях: SE, SE+, EE, CSE Lite (1.67), CSE Pro (1.67).{% endalert %}

1. Включите модуль `metallb`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: metallb
   spec:
     enabled: true
     version: 2
   ```

1. Создайте ресурс _MetalLoadBalancerClass_:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: MetalLoadBalancerClass
   metadata:
     name: ingress
   spec:
     addressPool:
       - 192.168.2.100-192.168.2.150
     isDefault: false
     nodeSelector:
       node-role.kubernetes.io/loadbalancer: "" # селектор узлов-балансировщиков
     type: L2
   ```

1. Создайте ресурс _IngressNginxController_:

   ```yaml
   apiVersion: deckhouse.io/v1
   kind: IngressNginxController
   metadata:
     name: main
   spec:
     ingressClass: nginx
     inlet: LoadBalancer
     loadBalancer:
       loadBalancerClass: ingress
       annotations:
         # Количество адресов, которые будут выделены из пула, объявленного в MetalLoadBalancerClass.
         network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
     # Селектор и tolerations. Поды ingress-controller должны быть размещены на тех же узлах, что и поды MetalLB speaker.
     nodeSelector:
        node-role.kubernetes.io/loadbalancer: ""
     tolerations:
     - effect: NoSchedule
       key: node-role/loadbalancer
       operator: Exists
   ```

1. Платформа создаст сервис с типом `LoadBalancer`, которому будет присвоено заданное количество адресов:

   ```shell
   $ d8 k -n d8-ingress-nginx get svc
   NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)                      AGE
   main-load-balancer     LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30689/TCP,443:30668/TCP   11s
   ```

## Пример разделения доступа между публичной и административной зонами

Во многих приложениях один и тот же backend обслуживает как публичную часть, так и административный интерфейс. Например:

- `https://example.com` — публичная зона;
- `https://admin.example.com` — административная зона, к которой доступ должен быть ограничен (`ACL`, `mTLS`, `IP whitelist` и т.д.).

При таком сценарии рекомендуем выносить административный трафик в отдельный Ingress-контроллер (при необходимости с отдельным Ingress-классом) и ограничивать доступ к нему с помощью параметра [`spec.acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom).

При такой конфигурации оба Ingress-ресурса указывают на один и тот же Service:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: admin-ingress
  annotations:
    nginx.ingress.kubernetes.io/whitelist-source-range: "1.2.3.4/32"
spec:
  ingressClassName: nginx
  rules:
    - host: admin.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: public-ingress
spec:
  ingressClassName: nginx
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: backend
                port:
                  number: 80
```

Приложение при этом может опираться на заголовок `Host` или заголовки `X-Forwarded-*` при принятии решений об авторизации. В такой схеме важно не только настроить правила доступа на уровне Ingress-ресурсов, но и ограничить, с каких адресов можно подключаться к самому Ingress-контроллеру.

Пример Ingress-контроллера, который обслуживает административные Ingress-ресурсы и принимает подключения только из заданных подсетей:

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: admin
spec:
  ingressClass: nginx
  inlet: HostPort
  acceptRequestsFrom:
    - 1.2.3.4/32
    - 10.0.0.0/16
  hostPort:
    httpPort: 80
    httpsPort: 443
    behindL7Proxy: true
```

В этом примере:

- Ingress-контроллер доступен на портах узлов через инлет `HostPort`;
- Параметр [`acceptRequestsFrom`](cr.html#ingressnginxcontroller-v1-spec-acceptrequestsfrom) разрешает подключение к контроллеру только из перечисленных подсетей;
- Даже если внешний балансировщик или клиент может передавать свои значения заголовков `X-Forwarded-*`, решение о допуске соединения до контроллера принимается по реальному адресу подключения, а не по заголовкам.
- Административные Ingress-ресурсы (в данном примере `admin-ingress`) обслуживаются этим контроллером согласно настроенному Ingress-классу.
