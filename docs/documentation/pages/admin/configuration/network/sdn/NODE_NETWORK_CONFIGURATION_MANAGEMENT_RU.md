---
title: "Управление конфигурацией сети на узлах"
permalink: ru/admin/configuration/network/sdn/node-network-configuration-management.html
description: |
  Программно-определяемые сети: управление конфигурацией сети на узлах
lang: ru
---

Для конфигурации сетевых интерфейсов на узлах используется декларативный API.

Поддерживаются следующие возможности по настройке сетевых интерфейсов на узлах:

* агрегация портов;
* объединение сетевых интерфейсов в бридж;
* настройка VLAN-интерфейсов.

## Пример создания интерфейса Bond

Объединение нескольких физических интерфейсов в Bond используется для повышения пропускной способности или резервирования.

{% alert level="info" %}
В интерфейс Bond могут быть объединены только сетевые интерфейсы, расположенные на одном физическом или виртуальном хосте.
{% endalert %}

Пример создания интерфейса Bond:

1. Установите на интерфейсы, предназначенные для объединения в Bond-интерфейс, пользовательские лейблы:

   > Ресурс NodeNetworkInterface в командах может быть сокращён до `nni`.

   ```shell
   d8 k label nni node-0-nic-fa163efbde48 nni.example.com/bond-group=bond0
   d8 k label nni node-0-nic-fa40asdxzx78 nni.example.com/bond-group=bond0
   ```

1. Подготовьте и примените конфигурацию для создания интерфейса Bond.

   Пример конфигурации:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: NodeNetworkInterface
   metadata:
     name: nni-node-0-bond0
   spec:
     nodeName: node-0
     type: Bond
     heritage: Manual
     bond:
       bondName: bond0
       memberNetworkInterfaces:
         - labelSelector:
             matchLabels:
               network.deckhouse.io/node-name: node-0 # Служебный лейбл, который необходимо указывать для объединения с интерфейсом Bond на определенном узле.
               nni.example.com/bond-group: bond0 # Пользовательский лейбл, вы должны установить самостоятельно на выбранные интерфейсы (был добавлен на интерфейсы на предыдущем шаге).
   ```

1. Проверьте статус созданного интерфейса Bond:

   Получите список интерфейсов:

   ```shell
   d8 k get nni
   ```

   Пример вывода:

   ```console
   NAME                                                       MANAGEDBY   NODE                          TYPE     IFNAME      IFINDEX   STATE   AGE
   nni-node-0-bond0                                           Manual      node-0-b23d3a26-5fb4b-5s9fp   Bond     bond0       76        Up      7m48s
   ...
   ```

   Посмотрите информацию о статусе нужного интерфейса:

   ```shell
   d8 k get nni nni-node-0-bond0 -o yaml
   ```

   Пример статуса интерфейса:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: NodeNetworkInterface
   metadata:
   ...
   status:
     conditions:
     - lastProbeTime: "2025-09-30T09:00:54Z"
       lastTransitionTime: "2025-09-30T09:00:39Z"
       message: Interface created
       reason: Created
       status: "True"
       type: Exists
     - lastProbeTime: "2025-09-30T09:00:54Z"
       lastTransitionTime: "2025-09-30T09:00:39Z"
       message: Interface is up and ready to send packets
       reason: Up
       status: "True"
       type: Operational
     deviceMAC: 6a:c7:ab:2a:a6:1e
     groupedLinks:
     - deviceMAC: fa:16:3e:92:14:40
       type: NIC
     ifIndex: 76
     ifName: bond0
     managedBy: Manual
     operationalState: Up
     permanentMAC: ""
   
   ```
