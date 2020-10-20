---
title: "Сloud provider — OpenStack: FAQ"
---

## Как поднять гибридный (вручную заведённые ноды) кластер?

1. Удалить flannel из kube-system: `kubectl -n kube-system delete ds flannel-ds`;
2. [Включите](configuration.html) модуль, или передайте флаг `--extra-config-map-data base64_encoding_of_custom_config` с [параметрами модуля](configuration.html#параметры) в скрипт установки `install.sh`.
3. Создайте один или несколько custom resource [OpenStackInstanceClass](cr.html#openstackinstanceclass).
4. Создайте один или несколько custom resource [NodeManager](/modules/040-node-manager/cr.html#nodegroup) для управления количеством и процессом заказа машин в облаке.

**Важно!** Cloud-controller-manager синхронизирует состояние между OpenStack и Kubernetes, удаляя из Kubernetes те узлы, которых нет в OpenStack. В гибридном кластере такое поведение не всегда соответствует потребности, поэтому если узел Kubernetes запущен не с параметром `--cloud-provider=external`, то он автоматически игнорируется (Deckhouse прописывает `static://` в ноды в в `.spec.providerID`, а cloud-controller-manager такие узлы игнорирует).


## Как настроить LoadBalancer?

**Внимание!!! Для корректного определения клиентского IP необходимо использовать LoadBalancer с поддержкой Proxy Protocol.**

##### Пример IngressNginxController

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: IngressNginxController
metadata:
  name: main
spec:
  ingressClass: nginx
  inlet: LoadBalancerWithProxyProtocol
  loadBalancerWithProxyProtocol:
    annotations:
      loadbalancer.openstack.org/proxy-protocol: "true"
  nodeSelector:
    node-role.deckhouse.io/frontend: ""
  tolerations:
  - effect: NoExecute
    key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
```
