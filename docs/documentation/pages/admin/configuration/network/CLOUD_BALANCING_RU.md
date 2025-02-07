---
title: "Балансировка в кластерах на облачных платформах"
permalink: ru/admin/network/cloud-balancing.html
lang: ru
---

Перечень поддерживаемых провайдеров:

* Amazon Web Services
* Google Cloud Platform
* Microsoft Azure
* OpenStack
* Huawei Cloud
* VMware Cloud DirectorExperimental
* VMware vSphere
* Yandex Cloud
* zVirtExperimental
* Базис.DynamiX

Настройка балансировки входящего трафика в кластерах на облачных платформах включает создание Ingres-контроллера.
Для него необходимо указать параметры LoadBalancer, который автоматически будет заказан у вашего облачного провайдера.
При создании LoadBalancer создается Service, на который будет направляться трафик с заказанного у провайдера LoadBalancer.

Пример (через UI и yaml)

## Пример создания Ingres-контроллера для провайдера OpenStack

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
      loadbalancer.openstack.org/timeout-member-connect: "2000"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```
