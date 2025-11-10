---
title: "The metallb module: примеры"
---

Metallb можно использовать в статических кластерах (bare metal), когда нет возможности воспользоваться балансировщиком от облачного провайдера. Metallb может работать в режимах L2 LoadBalancer или BGP LoadBalancer.

## Пример использования metallb в режиме L2 LoadBalancer

{% raw %}

Включите модуль:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  version: 2
```

Подготовьте приложение, которое хотите опубликовать:

```shell
d8 k create deploy nginx --image=nginx
```

Создайте ресурс MetalLoadBalancerClass:

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

Создайте ресурс Service с аннотацией и именем MetalLoadBalancerClass:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: nginx-deployment
  annotations:
    network.deckhouse.io/l2-load-balancer-external-ips-count: "3"
spec:
  type: LoadBalancer
  loadBalancerClass: ingress # имя MetalLoadBalancerClass
  ports:
  - port: 8000
    protocol: TCP
    targetPort: 80
  selector:
    app: nginx
```

В результате, созданному сервису с типом `LoadBalancer` будут присвоены адреса в заданном количестве:

```shell
d8 k get svc
NAME                   TYPE           CLUSTER-IP      EXTERNAL-IP                                 PORT(S)        AGE
nginx-deployment       LoadBalancer   10.222.130.11   192.168.2.100,192.168.2.101,192.168.2.102   80:30544/TCP   11s
```

Полученные EXTERNAL-IP можно прописывать в качестве A-записей для прикладного домена:

```shell
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.100:8000
200
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.101:8000
200
$ curl -s -o /dev/null -w "%{http_code}" 192.168.2.102:8000
200
```

{% endraw %}

## Пример использования metallb в режиме BGP LoadBalancer

{% raw %}

Включите модуль и настройте все необходимые параметры<sup>*</sup>:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: metallb
spec:
  enabled: true
  settings:
    addressPools:
    - addresses:
      - 192.168.219.100-192.168.219.200
      name: mypool
      protocol: bgp
    bgpPeers:
    - hold-time: 3s
      my-asn: 64600
      peer-address: 172.18.18.10
      peer-asn: 64601
    speaker:
      nodeSelector:
        node-role.deckhouse.io/metallb: ""
  version: 2
```

<sup>*</sup> — в будущих версиях настройки режима BGP будут задаваться через ресурс MetalLoadBalancerClass.

Настройте BGP-пиринг на сетевом оборудовании.

{% endraw %}

## Дополнительные примеры настроек для Service

{% raw %}

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

Для создания _Services_ с общими IP адресами необходимо добавить к ним аннотацию `network.deckhouse.io/load-balancer-shared-ip-key`. Общий ключ является значением аннотации. _Services_ могут совместно использовать IP-адрес при следующих условиях:

- Они имеют одинаковый ключ совместного использования.
- Они запрашивают использование разных портов (например, tcp/80 для одного и tcp/443 для другого).
- Они используют Cluster external traffic policy или указывают на один и тот же набор pods (т.е. селекторы pods идентичны).

Если эти условия выполняются, MetalLB может разместить два сервиса на одном IP-адресе, но **не обязан этого делать**! Если вы хотите гарантировать, что они будут совместно использовать определенный адрес, используйте аннотацию `network.deckhouse.io/load-balancer-ips`, описанную выше.

Ниже приведен пример конфигурации двух сервисов, которые совместно используют один IP-адрес:

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

Создание Service и назначение ему _IPAddressPools_ возможно в режиме BGP LoadBalancer через аннотацию `metallb.io/address-pool`. Для режима L2 LoadBalancer необходимо использовать настройки MetalLoadBalancerClass (см. выше).

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

{% endraw %}
