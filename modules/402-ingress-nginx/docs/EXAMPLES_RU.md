---
title: "Модуль ingress-nginx: пример"
---

{% raw %}

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

## Пример для bare metal (при использовании внешнего балансировщика

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
