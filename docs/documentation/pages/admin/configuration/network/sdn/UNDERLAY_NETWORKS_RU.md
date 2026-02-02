---
title: "Underlay-сети для проброса аппаратных устройств"
permalink: ru/admin/configuration/network/sdn/underlay-networks.html
description: |
  Программно-определяемые сети: underlay-сети для проброса аппаратных устройств
lang: ru
---

Ресурс [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork) обеспечивает прямое подключение физических сетевых интерфейсов (Physical Functions и Virtual Functions) к подам через Kubernetes Dynamic Resource Allocation (DRA). Эта функция предназначена для высокопроизводительных рабочих нагрузок, требующих прямого доступа к оборудованию, таких как приложения DPDK.

## Основные возможности

В DKP реализованы следующие возможности по работе с Underlay-сетями:

* **Проброс аппаратных устройств**: Физические сетевые интерфейсы (PF/VF) напрямую предоставляются подам, обходя сетевой стек ядра для максимальной производительности.
* **Настройка SR-IOV**: Автоматическая настройка SR-IOV на выбранных Physical Functions для создания Virtual Functions, что позволяет нескольким подам совместно использовать одно и то же оборудование.
* **Поддержка DPDK**: Устройства могут быть привязаны в различных режимах, подходящих для рабочих нагрузок DPDK:
  * **VFIO-PCI**: Явно подключает сетевое устройство в под, привязывая его к драйверу `vfio-pci`. Внутрь пода монтируются соответствующие VFIO dev-устройства (например, `/dev/vfio/vfio0`) для доступа из пользовательского пространства.
  * **DPDK**: Универсальный режим, который автоматически выбирает подходящий драйвер для вендора сетевого адаптера. Для сетевых карт Mellanox устройство привязывается к драйверу `mlx5_core` с прокидыванием как netdev-интерфейса, так и необходимых dev-устройств (файлы InfiniBand verbs, `/dev/net/tun`, а также соответствующий sysfs-каталог). Для остальных вендоров устройство привязывается через VFIO (так же, как в режиме VFIO-PCI).
  * **NetDev**: Внутрь пода прокидывается только сетевой интерфейс Linux как стандартное сетевое устройство ядра.

## Режимы работы

Поддерживаются следующие режимы выделения устройств, определяющие, как физические интерфейсы предоставляются подам:

* **Shared mode**: Создает Virtual Functions (VF) из Physical Functions (PF) с использованием SR-IOV, позволяя нескольким подам совместно использовать одно и то же оборудование. Каждый под получает одну или несколько VF.
* **Dedicated mode**: Предоставляет каждый Physical Function как эксклюзивное устройство без SR-IOV. Каждый под получает эксклюзивный доступ к полному PF.

## Автоматическая группировка интерфейсов

При включенном `autoBonding` контроллер группирует интерфейсы от нескольких совпавших PF в одно DRA устройство. Интерфейсы пробрасываются в под как отдельные сетевые интерфейсы, позволяя приложениям (например, DPDK) обрабатывать bonding/агрегацию на уровне приложения. Обратите внимание, что это не создает bonding-интерфейсы на уровне ядра внутри пода.

## Настройка и подключение физических интерфейсов в прикладные поды

### Предварительные требования для DPDK-приложений

Перед настройкой ресурсов UnderlayNetwork необходимо подготовить рабочие узлы кластера для DPDK-приложений:

* настроить [hugepages](#настройка-hugepages);
* настроить [Topology Manager](#настройка-topology-manager).

#### Настройка hugepages

DPDK-приложения требуют hugepages для эффективного управления памятью. Настройте hugepages на всех рабочих узлах с помощью NodeGroupConfiguration:

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

#### Настройка Topology Manager

Включите Topology Manager на NodeGroup рабочих узлов, где будут запускаться DPDK-приложения. Это обеспечивает выделение ресурсов CPU, памяти и устройств из одного NUMA узла.

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

* [topologyManager.enabled](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled)
* [topologyManager.policy](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy)

### Предварительные требования

Перед созданием UnderlayNetwork убедитесь, что:

1. Физические сетевые интерфейсы (NIC) доступны на узлах и обнаружены как ресурсы NodeNetworkInterface.
1. Интерфейсы, которые вы планируете использовать, являются Physical Functions (PF), а не Virtual Functions (VF).
1. Сетевые карты поддерживают SR-IOV (для режима [Shared](#режимы-работы)).

### Подготовка ресурсов NodeNetworkInterface

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
Вы можете проверить PCI информацию и статус поддержки SR-IOV для каждого интерфейса:

```shell
d8 k get nni worker-01-nic-0000:17:00.0 -o json | jq '.status.nic.pci.pf'
```

Ищите `status.nic.pci.pf.sriov.supported` для проверки поддержки SR-IOV.
{% endalert %}

### Создание UnderlayNetwork в режиме Dedicated

В режиме Dedicated каждый Physical Function предоставляется как эксклюзивное устройство. Этот режим подходит, когда:

* SR-IOV недоступен или не нужен;
* каждому поду требуется эксклюзивный доступ к полному PF.

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
          nic-group: dpdk
```

Когда `autoBonding` установлен в `true`, все совпавшие PF на узле группируются в одно DRA устройство, предоставляя поду все PF как отдельные интерфейсы. Когда `false`, — каждый PF публикуется как отдельное DRA устройство.

Проверьте статус созданного UnderlayNetwork:

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

### Создание UnderlayNetwork в режиме Shared

В режиме Shared создаются Virtual Functions (VF) из Physical Functions (PF) с использованием SR-IOV, позволяя нескольким подам совместно использовать одно и то же оборудование. Этот режим требует поддержки SR-IOV на сетевых картах.

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
          nic-group: dpdk
  shared:
    sriov:
      enabled: true
      numVFs: 8
```

В этом примере:

* `mode: Shared` включает SR-IOV и создание VF;
* `autoBonding: true` группирует одну VF от каждого совпавшего PF в одно DRA устройство;
* `shared.sriov.enabled: true` включает SR-IOV на выбранных PF;
* `shared.sriov.numVFs: 8` создает 8 Virtual Functions на каждый Physical Function.

{% alert level="warning" %}
Поля `mode` и `autoBonding` неизменяемы после установки. Тщательно спланируйте конфигурацию перед созданием ресурса.
{% endalert %}

После создания UnderlayNetwork отслеживайте статус конфигурации SR-IOV:

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

Вы можете убедиться, что VF были созданы, проверив ресурсы NodeNetworkInterface:

```shell
d8 k get nni -l network.deckhouse.io/nic-pci-type=VF
```

### Подготовка неймспейса для использования UnderlayNetwork

Перед тем как пользователи смогут запрашивать устройства UnderlayNetwork в своих подах, неймспейс должен быть помечен для включения поддержки UnderlayNetwork. Это административная задача, которая должна быть выполнена для неймспейса, где будут запускаться DPDK-приложения:

```shell
d8 k label namespace mydpdk direct-nic-access.network.deckhouse.io/enabled=""
```
