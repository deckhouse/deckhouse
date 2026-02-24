---
title: "Дополнительные сети для использования в прикладных подах"
permalink: ru/user/network/sdn/dedicated.html
description: |
  Создание и подключение дополнительных программно-определяемых сетей для подов: кластерные сети и сети проекта.
search: дополнительные сети, сеть проекта, кластерная сеть, Network, NetworkClass
lang: ru
---

В DKP реализована возможность использования дополнительных программно-определяемых сетей (далее — дополнительные сети) для прикладных нагрузок (поды, виртуальные машины). Вы можете использовать сети следующих типов:

- Кластерная (общедоступная) — сеть, общедоступная в каждом проекте, настраивается и управляется администратором. Пример — публичная WAN-сеть или shared-сеть обмена трафиком между проектами. Для создания такой сети и ее использования для прикладных подов обратитесь к администратору кластера.
- Сеть проекта (пользовательская сеть) — сеть, доступная в рамках неймспейса, создается и управляется пользователем c использованием предоставленного администратором манифеста NetworkClass.

Подробнее о дополнительных программно-определяемых сетях — в разделе [«Настройка и подключение дополнительных виртуальных сетей для использования в прикладных подах»](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#настройка-и-подключение-дополнительных-виртуальных-сетей-для-использования-в-прикладных-подах).

## Создание сети проекта (пользовательской сети)

Для создания сети для проекта используйте кастомные ресурсы [Network](/modules/sdn/cr.html#network) и [NetworkClass](/modules/sdn/cr.html#networkclass) (предоставляется администратором):

1. Создайте и примените манифест объекта Network, указав в поле `spec.networkClass` имя NetworkClass, полученное у администратора:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class # Имя NetworkClass, полученное от администратора.
   ```

   > Поддерживается статическое определение номера VLAN ID из пула, выданного администратором кластера. Если значение поля `spec.vlan.id` не указано, VLAN ID будет назначен динамически.

1. После создания объекта Network проверьте его статус:

   ```shell
   d8 k -n my-namespace get network my-network -o yaml
   ```

   Пример статуса объекта Network:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
   ...
   status:
     bridgeName: d8-br-600
     conditions:
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1
     vlanID: 600
   ```

После создания сети ее можно [подключать к подам](#подключение-дополнительных-сетей-к-подам).

## Подключение дополнительных сетей к подам

Вы можете подключать к подам кластерные сети и сети проекта. Для этого используйте аннотацию пода, в которой укажите параметры подключаемых дополнительных сетей.

Пример манифеста пода с добавлением двух дополнительных сетей (кластерной `my-cluster-network` и сети проекта `my-network`):

> В поле `ifName` (опционально) задается имя TAP-интерфейса внутри пода. В поле `mac` (опционально) задается MAC-адрес, который следует назначить TAP-интерфейсу.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: app-with-additional-networks
  namespace: my-namespace
  annotations:
    network.deckhouse.io/networks-spec: |
      [
        {
          "type": "Network",
          "name": "my-network",
          "ifName": "veth_mynet",
          "mac": "aa:bb:cc:dd:ee:ff"
        },
        {
          "type": "ClusterNetwork",
          "name": "my-cluster-network",
          "ifName": "veth_public"
        }
      ]
spec:
  containers:
    - name: app
    # остальные параметры...
```
