---
title: "Создание выделенной сети для проекта"
permalink: ru/user/network/sdn/dedicated-network-creating.html
lang: ru
---

Для создания выделенной сети для проекта используйте ресурсы [Network](cr.html#network) и [NetworkClass](cr.html#networkclass), предоставленные администратором:

1. Создайте и примените ресурс Network, указав в поле `spec.networkClass` имя ресурса NetworkClass, полученное у администратора:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class # Имя ресурса NetworkClass, полученное от администратора.
   ```

   > Поддерживается статическое определение номера VLAN ID из пула, выданного администратором кластера или сети `spec.vlan.id`, если значение не указано в спецификации ресурса оно будет назначено динамически.

1. После создания ресурса Network проверьте его статус:

   ```shell
   d8 k -n my-namespace get network my-network -o yaml
   ```

   Пример статуса ресурса Network:

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
