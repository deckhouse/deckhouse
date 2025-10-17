---
title: "Балансировка в кластерах на облачных платформах"
permalink: ru/admin/configuration/network/ingress/nlb/cloud-balancing.html
description: "Настройка балансировки нагрузки в облачных кластерах для платформы Deckhouse Kubernetes Platform. Интеграция с балансировщиками AWS, GCP, Azure и настройка облачной балансировки нагрузки."
lang: ru
---

Перечень поддерживаемых провайдеров:

* [Amazon Web Services](https://aws.amazon.com/),
* [Google Cloud Platform](https://cloud.google.com/),
* [Microsoft Azure](https://azure.microsoft.com/),
* [OpenStack](https://www.openstack.org/),
* [Huawei Cloud](https://cloud.huawei.com/),
* [VMware Cloud DirectorExperimental](https://www.vmware.com/products/cloud-infrastructure/cloud-director),
* [VMware vSphere](https://www.vmware.com/products/cloud-infrastructure/vsphere),
* [Yandex Cloud](https://yandex.cloud/),
* [zVirtExperimental](https://www.orionsoft.ru/zvirt),
* [Базис.DynamiX](https://basistech.ru/products/dynamix).

Настройка балансировки входящего трафика в кластерах на облачных платформах включает в себя создание Ingress-контроллера с указанием параметров LoadBalancer.
На их основе облачный провайдер автоматически создаёт внешний балансировщик нагрузки.
В кластере при этом создаётся ресурс Service, через который трафик от внешнего балансировщика будет направляться к приложениям.

## Пример создания Ingress-контроллера для провайдера OpenStack

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

## Пример создания сервиса с типом ClusterIP

```yaml
apiVersion: v1
kind: Service
metadata:
  name: backend-resolver-cluster-ip
spec:
  ports:
  - name: http
    port: 8000
    protocol: TCP
  selector:
    app: lab-4-backend
  type: ClusterIP
```

### Пример создания внутреннего балансировщика для провайдера VKCloud 

Пример подходит при необходимости заказать балансировщик внутри сети облака, без внешнего IP-адреса.

```yaml
apiVersion: deckhouse.io/v1
kind: IngressNginxController
metadata:
  name: nginx
spec:
  ingressClass: nginx
  inlet: LoadBalancer
  loadBalancer:
    annotations:
      service.beta.kubernetes.io/openstack-internal-load-balancer: "true"
  nodeSelector:
    node.deckhouse.io/group: worker
```