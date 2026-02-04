---
title: "Подготовка кластера и включение SDN"
permalink: ru/admin/configuration/network/sdn/cluster-preparing.html
description: |
  Подготовка кластера к использованию программно-определяемых сетей
lang: ru
---

Функции программно-определяемых сетей (SDN) в рамках Deckhouse Kubernetes Platform реализуются с помощью модуля [`sdn`](/modules/sdn/). DKP поддерживает следующие возможности по работе с программно-определяемыми сетями:

* [Управление конфигурацией сети на узлах](node-network-configuration-management.html).
* [Дополнительные сети](additional-networks.html).
* [Underlay-сети для проброса аппаратных устройств](underlay-networks.html).

## Подготовка инфраструктуры к включению модуля sdn

Перед использованием программно-определяемых сетей в кластере требуется предварительная подготовка инфраструктуры:

* **Для создания дополнительных сетей на основе тегированных VLAN:**
  * Выделите диапазоны VLAN ID на коммутаторах в ЦОД и настройте их на соответствующих сетевых интерфейсах коммутаторов.
  * Выберите физические интерфейсы на узлах для последующей настройки тегированных VLAN-интерфейсов. Допускается использование интерфейсов, уже задействованных под служебную локальную межузловую сеть DKP.
* **Для создания дополнительных сетей на основе прямого, нетегированного доступа в сетевой интерфейс:**
  * Выделите отдельные физические интерфейсы на узлах и скоммутируйте их в единую локальную сеть на уровне ЦОД.

## Действия после включения модуля sdn

После включения модуля в кластере автоматически появятся ресурсы [NodeNetworkInterface](cr.html#nodenetworkinterface), которые отражают текущее состояние интерфейсов на узлах.

Чтобы проверить наличие ресурсов, используйте команду:

```shell
d8 k get nodenetworkinterface
```

> Ресурс NodeNetworkInterface в командах может быть сокращён до `nni`.

Пример вывода:

```console
NAME                            MANAGEDBY   NODE           TYPE     IFNAME           IFINDEX   STATE      AGE
virtlab-ap-0-nic-1c61b4a68c2a   Deckhouse   virtlab-ap-0   NIC      eth1             3         Up         35d
virtlab-ap-0-nic-fc34970f5d1f   Deckhouse   virtlab-ap-0   NIC      eth0             2         Up         35d
virtlab-ap-1-nic-1c61b4a6a0e7   Deckhouse   virtlab-ap-1   NIC      eth1             3         Up         35d
virtlab-ap-1-nic-fc34970f5c8e   Deckhouse   virtlab-ap-1   NIC      eth0             2         Up         35d
virtlab-ap-2-nic-1c61b4a6800c   Deckhouse   virtlab-ap-2   NIC      eth1             3         Up         35d
virtlab-ap-2-nic-fc34970e7ddb   Deckhouse   virtlab-ap-2   NIC      eth0             2         Up         35d
```

{% alert level="info" %}
При обнаружении интерфейсов узлов контроллер устанавливает следующие служебные метки:

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

В этом примере у каждого узла в кластере есть по два сетевых интерфейса — eth0 (DKP LAN) и eth1 (выделенный интерфейс для организации дополнительных сетей).

Для дальнейшей работы разметьте выделенные интерфейсы под организацию дополнительных сетей подходящим лейблом.

Пример:

```shell
d8 k label nodenetworkinterface virtlab-ap-0-nic-1c61b4a68c2a nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-1-nic-1c61b4a6a0e7 nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-2-nic-1c61b4a6800c nic-group=extra
```

Также для повышения пропускной способности или резервирования возможно объединение нескольких физических интерфейсов в Bond. Подробнее — в разделе [«Создание интерфейса Bond»](node-network-configuration-management.html#пример-создания-интерфейса-bond).
