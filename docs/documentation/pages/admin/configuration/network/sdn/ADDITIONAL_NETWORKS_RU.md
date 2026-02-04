---
title: "Дополнительные сети"
permalink: ru/admin/configuration/network/sdn/additional-networks.html
description: |
  Программно-определяемые сети: дополнительные сети в кластере
lang: ru
---

В Deckhouse Kubernetes Platform реализована возможность декларативно управлять дополнительными сетями для прикладных нагрузок (поды, виртуальные машины). При этом предусмотрено следующее:

* Каждая дополнительная сеть подразумевает единственный L2-домен обмена данными.
* Внутри сетевого пространства пода дополнительная сеть представлена в виде tap-интерфейса.
* В качестве технологии организации L2-сети в настоящее время поддерживаются следующие режимы:
  * Тегированный VLAN — для связи между подами на разных узлах сетевые пакеты помечаются соответствующим VLAN ID и используют инфраструктурное сетевое оборудование для коммутации. Этот метод позволяет создать 4096 дополнительных сетей в рамках одного кластера;
  * Прямой доступ в сетевой интерфейс на узлах — для связи между подами на разных узлах используются локальные сетевые интерфейсы на узлах.
* С точки зрения управления сети бывают двух типов:
  * [Кластерная](#пример-создания-общедоступной-сети) — сеть, общедоступная в каждом проекте, управляется администратором. Пример — публичная WAN-сеть или shared-сеть обмена трафиком между проектами;
  * [Сеть проекта](#создание-сети-проекта-пользовательской-сети) — сеть, доступная в рамках неймспейса, управляется пользователем.

## Настройка и подключение дополнительных виртуальных сетей для использования в прикладных подах

Для настройки и подключения дополнительных сетей для прикладных подов используются кастомные ресурсы [ClusterNetwork](/modules/sdn/cr.html#clusternetwork), [Network](/modules/sdn/cr.html#network) и [NetworkClass](/modules/sdn/cr.html#networkclass).

{% alert level="info" %}
Если в ресурсах Network или ClusterNetwork был указан тип VLAN, также создадутся NodeNetworkInterface для VLAN и Bridge.
{% endalert %}

### Пример создания общедоступной сети

Для создания общедоступных сетей в масштабе всего кластера используется кастомный ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork).

#### Создание сети, основанной на тегированном трафике

Чтобы создать сеть, основанную на тегированном трафике, выполните следующие шаги:

1. Создайте и примените ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork):

   В параметре `spec.type` укажите значение `VLAN`. На соответствующих сетевых интерфейсах узлов будут настроены тегированные интерфейсы для обеспечения связности через VLAN, предоставленный инфраструктурой.

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetwork
   metadata:
     name: my-cluster-network
   spec:
     type: VLAN
     vlan:
       id: 900
     parentNodeNetworkInterfaces:
       labelSelector:
         matchLabels:
           nic-group: extra # Лейбл, вручную установленный на ресурсы NodeNetworkInterface.
   ```

1. Проверьте состояние созданного ресурса командой:

   ```shell
   d8 k get clusternetworks.network.deckhouse.io my-cluster-network -o yaml
   ```

   Пример статуса ресурса ClusterNetwork:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetwork
   metadata:
   ...
   status:
     bridgeName: d8-br-900
     conditions:
     - lastTransitionTime: "2025-09-29T14:39:20Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:39:20Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1
    
    ```

После создания ресурса ClusterNetwork будет автоматически создан ресурс NodeNetworkInterfaceAttachment, отражающий присоединение данной сети к интерфейсам на узлах.

Для получения списка ресурсов NodeNetworkInterfaceAttachment и информации о конкретном ресурсе используйте команды:

```shell
d8 k get nnia
d8 k get nnia my-cluster-network-... -o yaml
```

Пример ресурса NodeNetworkInterfaceAttachment:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeNetworkInterfaceAttachment
metadata:
...
  finalizers:
    - network.deckhouse.io/nni-network-interface-attachment
    - network.deckhouse.io/pod-network-interface-attachment
  generation: 1
  name: my-cluster-network-...
...
spec:
  networkRef:
    kind: ClusterNetwork
    name: my-cluster-network
  parentNetworkInterfaceRef:
    name: right-worker-b23d3a26-5fb4b-h2bkv-nic-fa163eebea7b
  type: VLAN
status:
  bridgeNodeNetworkInterfaceName: right-worker-b23d3a26-5fb4b-h2bkv-bridge-900
  conditions:
    - lastTransitionTime: "2025-09-29T14:39:06Z"
      message: Vlan created
      reason: VLANCreated
      status: "True"
      type: Exist
    - lastTransitionTime: "2025-09-29T14:39:06Z"
      message: Bridged successfully
      reason: VLANBridged
      status: "True"
      type: Ready
  nodeName: right-worker-b23d3a26-5fb4b-h2bkv
  vlanNodeNetworkInterfaceName: right-worker-b23d3a26-5fb4b-h2bkv-vlan-900-60f3dc
```

Статус NodeNetworkInterfaceAttachment изменится на `True` сразу после того как соответствующие NodeNetworkInterface появятся и перейдут в состояние `Up`.

Для проверки статусов NodeNetworkInterface используйте команду:

```shell
d8 k get nni
```

Пример вывода:

```console
NAME                                                 MANAGEDBY   NODE                                TYPE     IFNAME      IFINDEX   STATE   AGE
...
right-worker-b23d3a26-5fb4b-h2bkv-bridge-900         Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   Bridge   d8-br-900   684       Up      14h
right-worker-b23d3a26-5fb4b-h2bkv-nic-fa163eebea7b   Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   NIC      ens3        2         Up      19d
right-worker-b23d3a26-5fb4b-h2bkv-vlan-900-60f3dc    Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   VLAN     ens3.900    683       Up      14h
...
```

#### Создание сети, основанной на прямом доступе в интерфейс

Для создания  сети, основанной на прямом доступе в интерфейс, используйте ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork). В параметре `spec.type` укажите значение `Access`.  соответствующие сетевые адаптеры на узлах будут использоваться непосредственно для обеспечения связности.

Пример манифеста для сети, основанной на прямом доступе в интерфейс:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterNetwork
metadata:
  name: my-cluster-network
spec:
  type: Access
  parentNodeNetworkInterfaces:
    labelSelector:
      matchLabels:
        nic-group: extra # Лейбл, вручную установленный на ресурсы NodeNetworkInterface.
```

### Создание сети проекта (пользовательской сети)

Чтобы пользователи имели возможность создавать собственные выделенные сети, основанные на тегированном трафике, необходимо предварительно описать доступный им диапазон тегов и определить сетевые интерфейсы, на которых они могут быть настроены.
Для этого используется кастомный ресурс [NetworkClass](/modules/sdn/cr.html#clusternetworkclass).

Пример:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: NetworkClass
metadata:
  name: my-network-class
spec:
  vlan:
    idPool:
    - 600-800
    - 1200
    parentNodeNetworkInterfaces:
      labelSelector:
        matchLabels:
          nic-group: extra
```

Пример создания пользовательской сети с использованием созданного администратора ресурса NetworkClass описан в разделе [«Создание выделенной сети для проекта»](../../../../user/network/sdn/dedicated-network-creating.html).
