---
title: "Настройка SDN в кластере"
permalink: ru/admin/configuration/network/sdn/configure.html
description: |
  Подготовка кластера к настройке программно-определяемых сетей.
search: программно-определяемые сети, VLAN интерфейсы, дополнительные сети, underlay сети
lang: ru
---

Для использования SDN в кластере DKP требуется подготовка инфраструктуры к включению модуля [`sdn`](/modules/sdn/), а также некоторые подготовительные действия после его включения.

## Подготовка инфраструктуры к включению модуля sdn

Перед использованием дополнительных программно-определяемых сетей (далее — дополнительные сети) в кластере требуется предварительная подготовка инфраструктуры:

* **Для создания дополнительных программно-определяемых сетей на основе тегированных VLAN:**
  * Выделите диапазоны VLAN ID на коммутаторах в ЦОД и настройте их на соответствующих сетевых интерфейсах коммутаторов.
  * Выберите физические интерфейсы на узлах для последующей настройки тегированных VLAN-интерфейсов. Допускается использование интерфейсов, уже задействованных для служебной межузловой сети DKP.
* **Для создания дополнительных программно-определяемых сетей на основе прямого нетегированного доступа через сетевой интерфейс:**
  * Выделите отдельные физические интерфейсы на узлах и объедините их в единую сеть на уровне ЦОД.

## Включение модуля `sdn`

Включите модуль `sdn` согласно [инструкции](/modules/sdn/configuration.html).

## Действия после включения модуля `sdn`

После включения модуля в кластере автоматически будут созданы объекты [NodeNetworkInterface](/modules/sdn/cr.html#nodenetworkinterface), которые отражают текущее состояние сетевых интерфейсов на узлах.

Проверьте их наличие командой:

```shell
d8 k get nodenetworkinterface
NAME                            MANAGEDBY   NODE           TYPE     IFNAME           IFINDEX   STATE      AGE
virtlab-ap-0-nic-1c61b4a68c2a   Deckhouse   virtlab-ap-0   NIC      eth1             3         Up         35d
virtlab-ap-0-nic-fc34970f5d1f   Deckhouse   virtlab-ap-0   NIC      eth0             2         Up         35d
virtlab-ap-1-nic-1c61b4a6a0e7   Deckhouse   virtlab-ap-1   NIC      eth1             3         Up         35d
virtlab-ap-1-nic-fc34970f5c8e   Deckhouse   virtlab-ap-1   NIC      eth0             2         Up         35d
virtlab-ap-2-nic-1c61b4a6800c   Deckhouse   virtlab-ap-2   NIC      eth1             3         Up         35d
virtlab-ap-2-nic-fc34970e7ddb   Deckhouse   virtlab-ap-2   NIC      eth0             2         Up         35d
```

{% alert level="info" %}
При обнаружении интерфейсов узлов контроллер устанавливает на них следующие служебные лейблы (пример):

```yaml
labels:
  network.deckhouse.io/interface-mac-address: fa163eebea7b
  network.deckhouse.io/interface-type: NIC
  network.deckhouse.io/nic-pci-bus-info: 0000-17-00.0
  network.deckhouse.io/nic-pci-type: PF
  network.deckhouse.io/node-name: worker-01
annotations:
  network.deckhouse.io/heritage: NetworkController
```

{% endalert %}

В примере выше у каждого узла в кластере есть по два сетевых интерфейса — eth0 (DKP LAN) и eth1 (выделенный интерфейс для организации дополнительных программно-определяемых сетей).

### Разметка интерфейсов для организации дополнительных программно-определяемых сетей

Чтобы использовать [дополнительные программно-определяемые сети](#настройка-и-подключение-дополнительных-виртуальных-сетей-для-использования-в-прикладных-подах), назначьте выделенным интерфейсам (в примере выше, eth1) подходящий лейбл.

Пример:

```shell
d8 k label nodenetworkinterface virtlab-ap-0-nic-1c61b4a68c2a nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-1-nic-1c61b4a6a0e7 nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-2-nic-1c61b4a6800c nic-group=extra
```

### Объединение нескольких физических интерфейсов в интерфейс агрегации каналов (bond-интерфейс)

Для увеличения пропускной способности или резервирования можно объединить несколько физических интерфейсов в агрегированный канал (интерфейс агрегации каналов, bond-интерфейс).

{% alert level="info" %}
Объединять можно только сетевые интерфейсы, расположенные на одном физическом или виртуальном хосте.
{% endalert %}

Пример создания интерфейса агрегации каналов:

1. Установите на интерфейсы, предназначенные для агрегации, пользовательские лейблы.

   Пример установки лейбла `nni.example.com/bond-group=bond0` на интерфейсы:

   ```shell
   d8 k label nni node-0-nic-fa163efbde48 nni.example.com/bond-group=bond0
   d8 k label nni node-0-nic-fa40asdxzx78 nni.example.com/bond-group=bond0
   ```

1. Подготовьте и примените конфигурацию для создания интерфейса агрегации.

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
               # Служебный лейбл, который необходимо указывать для объединения с интерфейсом Bond на определенном узле.
               network.deckhouse.io/node-name: worker-01
               # Пользовательский лейбл, (был добавлен на интерфейсы на предыдущем шаге).
               nni.example.com/bond-group: bond0
   ```

1. Проверьте статус созданного интерфейса агрегации.

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

## Настройка и подключение дополнительных виртуальных сетей для использования в прикладных подах

В Deckhouse Kubernetes Platform можно декларативно управлять дополнительными сетями для прикладных нагрузок (поды, виртуальные машины). При этом предусмотрено следующее:

* Каждая дополнительная сеть подразумевает единственный L2-домен обмена данными.
* Внутри сетевого пространства пода дополнительная сеть представлена в виде tap-интерфейса.
* В качестве технологии организации L2-сети в настоящее время поддерживаются следующие режимы:
  * **Тегированный VLAN** — для связи между подами на разных узлах сетевые пакеты помечаются соответствующим VLAN ID и используют инфраструктурное сетевое оборудование для коммутации. Этот метод позволяет создать 4096 дополнительных сетей в рамках одного кластера;
  * **Прямой доступ в сетевой интерфейс на узлах** — для связи между подами на разных узлах используются локальные сетевые интерфейсы на узлах.
* По модели управления сети бывают двух типов:
  * **[Кластерная сеть](#создание-общедоступной-кластерной-сети)** — сеть, общедоступная в каждом проекте, управляется администратором. Пример — публичная WAN-сеть или shared-сеть обмена трафиком между проектами;
  * **[Сеть проекта](#создание-сети-проекта-пользовательской-сети)** — сеть, доступная в рамках неймспейса, управляется пользователем.

Для настройки и подключения дополнительных сетей для прикладных подов используются кастомные ресурсы [ClusterNetwork](/modules/sdn/cr.html#clusternetwork), [Network](/modules/sdn/cr.html#network) и [NetworkClass](/modules/sdn/cr.html#networkclass).

{% alert level="info" %}
Если в ресурсах [Network](/modules/sdn/cr.html#network) или [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) был указан тип VLAN, также создадутся [NodeNetworkInterface](/modules/sdn/cr.html#nodenetworkinterface) для VLAN и Bridge-интерфейсов.
{% endalert %}

{% alert level="warning" %}
Перед созданием дополнительной сети [разметьте интерфейсы](#разметка-интерфейсов-под-организацию-дополнительных-программно-определяемых-сетей), которые будут использоваться для её подключения.
{% endalert %}

### Создание общедоступной (кластерной) сети

Для создания общедоступных сетей в масштабе всего кластера используется кастомный ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork).

#### Создание общедоступной сети, основанной на тегированном трафике

Чтобы создать сеть, основанную на тегированном трафике, выполните следующие шаги:

1. Создайте и примените ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork).

   В параметре `spec.type` укажите значение `VLAN`. На соответствующих сетевых интерфейсах узлов будут настроены тегированные интерфейсы для обеспечения связности через VLAN, предоставленный инфраструктурой.

   Пример манифеста ClusterNetwork для создания общедоступной сети, основанной на тегированном трафике:

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
           # Лейбл, установленный на ресурсы NodeNetworkInterface на этапе разметки интерфейсов под организацию дополнительных программно-определяемых сетей.
           nic-group: extra
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

1. Проверьте [присоединение дополнительной сети к интерфейсам на узлах](#проверка-присоединения-дополнительной-сети-к-интерфейсам-на-узлах).

#### Создание сети, основанной на прямом доступе в интерфейс

Для создания сети на основе прямого доступа к интерфейсу используйте ресурс [ClusterNetwork](/modules/sdn/cr.html#clusternetwork). В параметре `spec.type` укажите значение `Access`. Выбранные сетевые адаптеры на узлах будут использоваться напрямую для обеспечения связности.

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
        # Лейбл, установленный на ресурсы NodeNetworkInterface на этапе разметки интерфейсов под организацию дополнительных программно-определяемых сетей.
        nic-group: extra
```

После создания сети проверьте ее [присоединение к интерфейсам на узлах](#проверка-присоединения-дополнительной-сети-к-интерфейсам-на-узлах).

### Создание сети проекта (пользовательской сети)

Чтобы пользователи имели возможность создавать собственные выделенные сети, основанные на тегированном трафике, необходимо предварительно описать доступный им диапазон тегов и определить сетевые интерфейсы, на которых они могут быть настроены.
Для этого используется кастомный административный ресурс [NetworkClass](/modules/sdn/cr.html#clusternetworkclass).

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
          nic-group: extra # Лейбл, установленный на ресурсы NodeNetworkInterface на этапе разметки интерфейсов под организацию дополнительных программно-определяемых сетей.
```

По запросу пользователя администратор предоставляет ему название созданного NetworkClass, который используется при создании сети проекта.

Пример создания пользовательской сети с использованием созданного административного ресурса NetworkClass описан в разделе [«Создание сети проекта (пользовательской сети)»](../../../../user/network/sdn/dedicated.html#создание-сети-проекта-пользовательской-сети).

### Проверка присоединения дополнительной сети к интерфейсам на узлах

После создания ресурса ClusterNetwork или Network будет автоматически создан ресурс NodeNetworkInterfaceAttachment, отражающий присоединение данной сети к интерфейсам на узлах.

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

### IPAM для дополнительных сетей

Механизм IPAM (IP Address Management) позволяет выделять IPv4-адреса из пулов и назначать их на дополнительные сетевые интерфейсы подов, подключаемых к [кластерным сетям](#создание-общедоступной-кластерной-сети) и [сетям проекта](#создание-сети-проекта-пользовательской-сети).

#### Принципы и особенности работы IPAM в DKP

Для каждого выделяемого IP-адреса создаётся и используется объект [IPAddress](/modules/sdn/cr.html#ipaddress) ([ClusterIPAddress](/modules/sdn/cr.html#clusteripaddress) — для кластерных сетей), который ссылается на сеть проекта или кластерную сеть. Контроллер выделяет адрес из пула и сохраняет результат в `status.address`, `status.network`, `status.routes` объекта IPAddress (ClusterIPAddress). Агент на узле назначает IP-адрес и маршруты на интерфейс внутри пода и устанавливает поля `status.conditions[Attached]` и `status.usedByPods` объекта IPAddress (ClusterIPAddress).

##### Защита от конфликтов при переиспользовании IP-адресов

Для защиты от конфликтов создаётся cluster-scoped объект [IPAddressLease](/modules/sdn/cr.html#ipaddresslease), который резервирует IP-адрес. При удалении объекта IPAddress (ClusterIPAddress) соответствующий ему IPAddressLease помечается как `orphaned` (для этого используется поле `status.orphaningTimestamp`) и удерживает адрес в течение времени, указанном в параметре [`spec.ttl`](/modules/sdn/cr.html#ipaddresslease-v1alpha1-spec-ttl) (чтобы избежать быстрых переиспользований IP-адресов).

#### Ресурсы и параметры для настройки IPAM

Для управления выделением и назначением IP-адресов используются:

* Пулы адресов: для кластерных сетей (ресурс [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool)) или сетей проекта (ресурс [IPAddressPool](/modules/sdn/cr.html#ipaddresspool)).
* Параметры для включения IPAM для конкретной сети: [`Network.spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#network-v1alpha1-spec-ipam-ipaddresspoolref) —  для сетей проекта, [`ClusterNetwork.spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#clusternetwork-v1alpha1-spec-ipam-ipaddresspoolref) — для кластерных сетей.
* Ресурс [IPAddress](/modules/sdn/cr.html#ipaddress) ([ClusterIPAddress](/modules/sdn/cr.html#clusteripaddress) — для кластерных сетей) — запрос (автоматический или статический) на выделение адреса, который затем назначается на интерфейс пода, подключаемого к кластерной сети или сети проекта.

#### Пример выделения пула IP-адресов для кластерной сети

> Чтобы выделить пул адресов для [кластерной сети](#создание-общедоступной-кластерной-сети), используйте ресурс [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool).

Для выделения пула адресов, предназначенных для назначения на сетевые интерфейсы подов, подключаемых к кластерной сети, выполните следующие действия:

1. Создайте пул адресов. Для этого используйте ресурс [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool).

   Пример:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterIPAddressPool
   metadata:
     name: public-net-pool
   spec:
     leaseTTL: 24h
     pools:
       - network: 203.0.113.0/24
         ranges:
           - 203.0.113.10-203.0.113.200
   ```

   > Параметр [`spec.pools[].ranges`](/modules/sdn/cr.html#clusteripaddresspool-v1alpha1-spec-pools-ranges) опционален. Если он не указан, доступным считается весь CIDR из [`spec.pools[].network`](/modules/sdn/cr.html#clusteripaddresspool-v1alpha1-spec-pools-network) (за исключением network/broadcast адресов, см. поведение `/31` и `/32`).

1. Включите IPAM в сети. Для этого в параметре [`spec.ipam.ipAddressPoolRef`](/modules/sdn/cr.html#clusternetwork-v1alpha1-spec-ipam-ipaddresspoolref) ресурса ClusterNetwork укажите параметры созданного на предыдущем шаге ClusterIPAddressPool.

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
           nic-group: extra
     ipam:
       ipAddressPoolRef:
         kind: ClusterIPAddressPool
         name: public-net-pool
   ```

После выделения пула IP-адресов для кластерной сети их можно назначать на сетевые интерфейсы подов, подключаемых к этой сети. Подробнее — в разделе [«Назначение IP-адресов на сетевые интерфейсы подов, подключаемых к дополнительной сети»](../../../../user/network/sdn/dedicated.html#назначение-ip-адресов-на-сетевые-интерфейсы-подов-подключаемых-к-дополнительной-сети).

## Настройка и подключение underlay-сетей для проброса аппаратных устройств

Эта функция предназначена для высокопроизводительных рабочих нагрузок, требующих прямого доступа к оборудованию, таких как приложения DPDK.

### Основные возможности

В DKP реализованы следующие возможности по работе с Underlay-сетями:

* **Проброс аппаратных устройств** — физические сетевые интерфейсы (PF/VF) напрямую предоставляются подам, обходя сетевой стек ядра для максимальной производительности.
* **Настройка SR-IOV** — автоматическая настройка SR-IOV на выбранных Physical Functions для создания Virtual Functions, что позволяет нескольким подам совместно использовать одно и то же оборудование.
* **Поддержка DPDK** — устройства могут быть привязаны в различных режимах, подходящих для рабочих нагрузок DPDK.
  * **VFIO-PCI** — явно подключает сетевое устройство в под, привязывая его к драйверу `vfio-pci`. Внутрь пода монтируются соответствующие VFIO dev-устройства (например, `/dev/vfio/vfio0`) для доступа из пользовательского пространства.
  * **DPDK** — универсальный режим, который автоматически выбирает подходящий драйвер для вендора сетевого адаптера. Для сетевых карт Mellanox устройство привязывается к драйверу `mlx5_core` с пробрасыванием как netdev-интерфейса, так и необходимых dev-устройств (файлы InfiniBand verbs, `/dev/net/tun`, а также соответствующий sysfs-каталог). Для остальных вендоров устройство привязывается через VFIO (также, как в режиме VFIO-PCI).
  * **NetDev** — в под пробрасывается только сетевой интерфейс Linux как стандартное сетевое устройство ядра.

### Режимы работы

Поддерживаются следующие режимы выделения устройств, определяющие, как физические интерфейсы предоставляются подам:

* [**Shared mode**](#создание-underlay-сети-в-режиме-shared) — создает Virtual Functions (VF) из Physical Functions (PF) с использованием SR-IOV, позволяя нескольким подам совместно использовать одно и то же оборудование. Каждый под получает одну или несколько VF.
* [**Dedicated mode**](#создание-underlay-сети-в-режиме-dedicated) — предоставляет каждый Physical Function как эксклюзивное устройство без SR-IOV. Каждый под получает эксклюзивный доступ к полному PF.

### Автоматическая группировка интерфейсов

При включенном [`autoBonding`](/modules/sdn/cr.html#underlaynetwork-v1alpha1-spec-autobonding) контроллер группирует интерфейсы от нескольких совпавших PF в одно DRA-устройство. Интерфейсы пробрасываются в под как отдельные сетевые интерфейсы, позволяя приложениям (например, DPDK) обрабатывать bonding/агрегацию на уровне приложения. Обратите внимание — bonding-интерфейсы на уровне ядра внутри пода не создаются.

### Порядок настройки и подключения физических интерфейсов в прикладные поды

Для создания Underlay-сетей для проброса аппаратных устройств в поды используется кастомный ресурс [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork). Он обеспечивает прямое подключение физических сетевых интерфейсов (Physical Functions и Virtual Functions) к подам через Kubernetes Dynamic Resource Allocation (DRA).

#### Предварительные требования для DPDK-приложений

Перед созданием и настройкой ресурсов UnderlayNetwork необходимо подготовить рабочие узлы кластера для DPDK-приложений:

* настроить [hugepages](#настройка-hugepages);
* настроить [Topology Manager](#настройка-topology-manager).

##### Настройка hugepages

DPDK-приложения требуют hugepages для эффективного управления памятью. Настройте hugepages на всех рабочих узлах с помощью [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: hugepages-for-dpdk
spec:
  nodeGroups:
    - '*'  # Применить ко всем группам узлов.
  weight: 100
  content: |
    #!/bin/bash
    echo "vm.nr_hugepages = 4096" > /etc/sysctl.d/99-hugepages.conf
    sysctl -p /etc/sysctl.d/99-hugepages.conf
```

Эта конфигурация устанавливает `vm.nr_hugepages = 4096` на всех узлах, предоставляя 8 GiB hugepages (4096 страниц × 2 MiB на страницу).

##### Настройка Topology Manager

Включите Topology Manager на [NodeGroup](/modules/node-manager/cr.html#nodegroup) рабочих узлов, где будут запускаться DPDK-приложения. Это обеспечит выделение ресурсов CPU, памяти и устройств из одного NUMA узла.

Пример конфигурации NodeGroup:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  kubelet:
    topologyManager:
      enabled: true
      policy: SingleNumaNode
      scope: Container
  nodeType: Static
```

Для получения дополнительной информации см.:

* [topologyManager.enabled](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled);
* [topologyManager.policy](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy).

#### Предварительные действия перед созданием UnderlayNetwork

Перед созданием UnderlayNetwork убедитесь, что:

1. Физические сетевые интерфейсы (NIC) доступны на узлах и обнаружены как ресурсы NodeNetworkInterface.
1. Интерфейсы, которые вы планируете использовать, являются Physical Functions (PF), а не Virtual Functions (VF).
1. Сетевые карты поддерживают SR-IOV (для режима [Shared](#режимы-работы)).

##### Проверка и настройка сетевых интерфейсов (ресурсов NodeNetworkInterface)

Сначала проверьте, какие Physical Functions доступны на ваших узлах:

```shell
d8 k get nni -l network.deckhouse.io/nic-pci-type=PF
```

Пример вывода:

```console
NAME                            MANAGEDBY   NODE           TYPE   IFNAME   IFINDEX   STATE   VF/PF   Binding   Driver      Vendor   AGE
worker-01-nic-0000:17:00.0      Deckhouse   worker-01     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-01-nic-0000:17:00.1      Deckhouse   worker-01     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.0      Deckhouse   worker-02     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.1      Deckhouse   worker-02     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
```

Пометьте интерфейсы, которые будут использоваться для UnderlayNetwork:

```shell
d8 k label nni worker-01-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-01-nic-0000:17:00.1 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.1 nic-group=dpdk
```

{% alert level="info" %}
Вы можете проверить PCI-информацию и статус поддержки SR-IOV для каждого интерфейса с помощью команды:

```shell
d8 k get nni worker-01-nic-0000:17:00.0 -o json | jq '.status.nic.pci.pf'
```

В секции `status.nic.pci.pf.sriov.supported` можно найти информацию и поддержке SR-IOV.
{% endalert %}

#### Создание Underlay-сети в режиме Dedicated

В режиме Dedicated каждый Physical Function предоставляется как эксклюзивное устройство. Этот режим подходит, когда:

* SR-IOV недоступен или не нужен;
* каждому поду требуется эксклюзивный доступ к полному PF.

Чтобы создать Underlay-сеть в режиме Dedicated, выполните следующие шаги:

1. Создайте и примените ресурс [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork). В поле `spec.mode` укажите значение `Dedicated`.

   Пример конфигурации:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-dedicated-network
   spec:
     mode: Dedicated
     autoBonding: false
     memberNodeNetworkInterfaces:
       - labelSelector:
           matchLabels:
             nic-group: dpdk # Лейбл, которым помечены интерфейсы на этапе проверки и настройки сетевых интерфейсов.
   ```

   Если `autoBonding` установлен в `true`, все совпавшие PF на узле группируются в одно DRA-устройство, предоставляя поду все PF как отдельные интерфейсы. Когда `false`, — каждый PF публикуется как отдельное DRA-устройство.

1. Проверьте статус созданного UnderlayNetwork:

   ```shell
   d8 k get underlaynetwork dpdk-dedicated-network -o yaml
   ```

   Пример статуса UnderlayNetwork в режиме Dedicated:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-dedicated-network
   ...
   status:
     observedGeneration: 1
     conditions:
     - message: All 2 member node network interface selectors have matches
       observedGeneration: 1
       reason: AllInterfacesAvailable
       status: "True"
       type: InterfacesAvailable
   ```

#### Создание Underlay-сети в режиме Shared

В режиме Shared создаются Virtual Functions (VF) из Physical Functions (PF) с использованием SR-IOV, позволяя нескольким подам совместно использовать одно и то же оборудование. Этот режим требует поддержки SR-IOV на сетевых картах.

Чтобы создать Underlay-сеть в режиме Shared, выполните следующие шаги:

1. Создайте и примените ресурс UnderlayNetwork. В поле `spec.mode` укажите значение `Shared`.

   Пример конфигурации:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-shared-network
   spec:
     mode: Shared
     autoBonding: true
     memberNodeNetworkInterfaces:
       - labelSelector:
           matchLabels:
             nic-group: dpdk # Лейбл, которым помечены интерфейсы на этапе проверки и настройки сетевых интерфейсов.
     shared:
       sriov:
         enabled: true
         numVFs: 8
   ```

   В этом примере:

   * `mode: Shared` включает SR-IOV и создание VF;
   * `autoBonding: true` группирует одну VF от каждого совпавшего PF в одно DRA-устройство;
   * `shared.sriov.enabled: true` включает SR-IOV на выбранных PF;
   * `shared.sriov.numVFs: 8` создает 8 Virtual Functions на каждый Physical Function.

   > Поля `mode` и `autoBonding` неизменяемы после установки. Тщательно спланируйте конфигурацию перед созданием ресурса.

1. После создания UnderlayNetwork отслеживайте статус конфигурации SR-IOV:

   ```shell
   d8 k get underlaynetwork dpdk-shared-network -o yaml
   ```

   Пример статуса UnderlayNetwork в режиме Shared:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-shared-network
   ...
   status:
     observedGeneration: 1
     sriov:
       supportedNICs: 4
       enabledNICs: 4
     conditions:
     - lastTransitionTime: "2025-01-15T10:30:00Z"
       observedGeneration: 1
       message: SR-IOV configured on 4 NICs
       reason: SRIOVConfigured
       status: "True"
       type: SRIOVConfigured
     - lastTransitionTime: "2025-01-15T10:30:05Z"
       observedGeneration: 1
       message: Interfaces are available for allocation
       reason: InterfacesAvailable
       status: "True"
       type: InterfacesAvailable
   ```

1. Убедитесь, что VF были созданы, проверив ресурс [NodeNetworkInterface](/modules/sdn/cr.html#nodenetworkinterface):

   ```shell
   d8 k get nni -l network.deckhouse.io/nic-pci-type=VF
   ```

### Подготовка неймспейса для использования UnderlayNetwork

Перед тем как пользователи смогут запрашивать устройства UnderlayNetwork в своих подах, неймспейс должен быть помечен для включения поддержки UnderlayNetwork. Это административная задача, которая должна быть выполнена для неймспейса, где будут запускаться DPDK-приложения.

Чтобы пометить неймспейс для включения поддержки UnderlayNetwork, используйте команду:

```shell
d8 k label namespace mydpdk direct-nic-access.network.deckhouse.io/enabled=""
```

## Настройка и подключение системных сетей (сервисных сетей)

Системные сети (сервисные сети) предназначены для служебного трафика на уровне узлов (например, для нужд хранилища, управления и т. д.) и не используются как дополнительные сети подов.

Дополнительные сервисные сети создаются на узлах кластера поверх существующих underlay-сетей. Для этого используется кастомный ресурс [SystemNetwork](/modules/sdn/cr.html#systemnetwork). Системные сети получают IP-адреса из [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool) при настройке IPAM.

Принципы и особенности работы системных сетей:

* **Работа поверх underlay-сетей**. Системная сеть подключается к underlay-сети ([UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork)). Для подключения в параметре [`spec.underlayNetworkName`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-underlaynetworkname) ресурса SystemNetwork указывается имя underlay-сети, к которой должна быть подключена системная сеть. Набор интерфейсов узла (PF или VF), которые будет использовать системная сеть, определяется в параметре [`memberNodeNetworkInterfaces`](/modules/sdn/cr.html#underlaynetwork-v1alpha1-spec-membernodenetworkinterfaces) объекта UnderlayNetwork.
* **Поддержка разных типов подключения системной сети к underlay-сети**. Можно создавать VLAN-интерфейсы для подов (`type: VLAN`), использовать прямой доступ к интерфейсам на узлах, подключенным к underlay-сети (`type: Access`) или подключиться через SR-IOV виртуальную функцию (`type: SRIOVVirtualFunction`) с опциональной настройкой (MTU, MAC, spoof checking и т. д.).
* **Поддержка механизма IPAM**. Опциональный параметр [`spec.ipam`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-ipam) ссылается на [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool). Контроллер и агент выделяют адреса из этого пула и назначают их сетевым интерфейсам узла для данной системной сети.
* **Отслеживание статусов системных сетей**. Агент отчитывается об адресах узла (включая IP-адреса системных сетей) в ресурсах NodeNetworkStatus. Внутренние ресурсы SystemNetworkNodeNetworkInterfaceAttachment отслеживают привязку каждой системной сети к родительскому интерфейсу на каждом узле.

### Предварительные требования для создания и использования системных сетей

Для создания и использования системных сетей в кластере необходимо выполнение следующих требований:

1. Должна существовать [underlay-сеть](#настройка-и-подключение-underlay-сетей-для-проброса-аппаратных-устройств). Системная сеть подключается к ней с помощью параметра [`spec.underlayNetworkName`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-underlaynetworkname). Селекторы, указанные в параметре [`memberNodeNetworkInterfaces`](/modules/sdn/cr.html#underlaynetwork-v1alpha1-spec-membernodenetworkinterfaces) объекта UnderlayNetwork, определяют, какие интерфейсы узлов используются системной сетью.
1. **Опционально**. Для автоматической выдачи IP-адресов на интерфейсах, принадлежащих системной сети, [создайте пул адресов для системной сети](#создание-пула-ip-адресов-для-настройки-ipam-системной-сети). Его необходимо указать в параметре [`spec.ipam.clusterIPAddressPoolName`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-ipam-clusteripaddresspoolname) ресурса SystemNetwork при создании сети.

### Создание системной сети

Для создания системной сети используйте ресурс [SystemNetwork](/modules/sdn/cr.html#systemnetwork). Поддерживается создание системных сетей со следующими типами подключения к underlay-сетям, поверх которых они будут работать:

* [`VLAN`](#тип-vlan) — на интерфейсах, подключенных к underlay-сети (underlay-интерфейсах) создаются VLAN-интерфейсы;
* [`Access`](#тип-access) — прямой доступ (без VLAN) к интерфейсам на узлах, подключенным к underlay-сети;
* [`SRIOVVirtualFunction`](#тип-sriovvirtualfunction) — подключение к физическому интерфейсу через SR-IOV.

#### Тип VLAN

Создаёт VLAN-интерфейс на каждом совпавшем underlay-интерфейсе. При создании сети требуется указать VLAN ID (параметр [`spec.vlan.id`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-vlan-id)).

Пример манифеста системной сети с типом подключения к underlay-сети `VLAN`:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: SystemNetwork
metadata:
  name: storage-network
spec:
  type: VLAN
  underlayNetworkName: my-underlay
  vlan:
    id: 100
  ipam:
    clusterIPAddressPoolName: storage-pool
```

Для проверки статуса сети после создания воспользуйтесь разделом [«Проверка статуса системной сети»](#проверка-статуса-системной-сети).

#### Тип Access

Использует underlay-интерфейс напрямую (без VLAN). Удобно, когда underlay уже представляет один L2-сегмент.

Пример манифеста системной сети с типом подключения к underlay-сети `Access`:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: SystemNetwork
metadata:
  name: mgmt-network
spec:
  type: Access
  underlayNetworkName: my-underlay
  ipam:
    clusterIPAddressPoolName: mgmt-pool
```

Для проверки статуса сети после создания воспользуйтесь разделом [«Проверка статуса системной сети»](#проверка-статуса-системной-сети).

#### Тип SRIOVVirtualFunction

Подключение через SR-IOV виртуальную функцию. Физический сетевой интерфейс должен быть в режиме [`Shared`](#создание-underlay-сети-в-режиме-shared), чтобы существовали VF. Опционально можно настроить параметры VF.

Пример манифеста системной сети с типом подключения к underlay-сети `SRIOVVirtualFunction`:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: SystemNetwork
metadata:
  name: vf-service-network
spec:
  type: SRIOVVirtualFunction
  underlayNetworkName: dpdk-shared-network
  sriovVirtualFunction:
    vlan: 200
    mtu: 1500
    linkState: Auto
  ipam:
    clusterIPAddressPoolName: vf-pool
```

Для проверки статуса сети после создания воспользуйтесь разделом [«Проверка статуса системной сети»](#проверка-статуса-системной-сети).

### Создание пула IP-адресов для настройки IPAM системной сети

Чтобы создать пул адресов для настройки IPAM системной сети, используйте ресурс [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool):

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterIPAddressPool
metadata:
  name: storage-pool
spec:
  leaseTTL: 24h
  pools:
    - network: 10.20.30.0/24
      ranges:
        - 10.20.30.10-10.20.30.250
```

При [создании системной сети](#создание-системной-сети) укажите этот пул в параметре [`spec.ipam.clusterIPAddressPoolName`](/modules/sdn/cr.html#systemnetwork-v1alpha1-spec-ipam-clusteripaddresspoolname) ресурса SystemNetwork.

### Проверка статуса системной сети

Для получения списка системных сетей используйте команду:

```shell
d8 k get systemnetworks
```

Для просмотра статуса конкретной системной сети используйте команду:

```shell
d8 k get systemnetwork storage-network -o yaml
```

В `status` отображаются:

* `nodeAttachementsCount` — общее число привязок (по одной на совпавший интерфейс узла);
* `readyNodeAttachementsCount` — привязки в состоянии готовности (настроены и работают);
* `conditions` — например, `Ready`, когда все привязки готовы.

Для просмотра внутренних привязок (по одной на пару «системная сеть + родительский интерфейс») используйте команду:

```shell
d8 k get systemnetworknodenetworkinterfaceattachments
```

Для просмотра IP-адресов на уровне узла (включая IP системных сетей) для всех узлов используйте команду (на каждый узел назначается по одному NodeNetworkStatus):

```shell
d8 k get nodenetworkstatus
```

Для просмотра информации об IP-адресах на уровне узла (включая IP системных сетей) для конкретного узла используйте команду:

```shell
d8 k get nodenetworkstatus -l network.deckhouse.io/node-name=worker-01 -o yaml
```

В `status.addresses` ищите записи с `type: SystemNetworkIP` и полем `systemNetworkName`, значение которого равно имени вашей системной сети.
