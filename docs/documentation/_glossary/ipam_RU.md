---
title: IPAM
description: Механизм управления IP-адресами, который выделяет адреса сетевым интерфейсам подов и виртуальных машин и следит, чтобы один адрес не был выдан дважды. В Deckhouse Platform свой IPAM есть у CNI-модулей, у модуля sdn и у модуля virtualization.
lang: ru
---

Модули `cni-cilium`, `cni-flannel` и `cni-simple-bridge` назначают подам адреса из диапазона адресов подов, закреплённого за узлом. Этот IPAM не настраивается.

В модуле [`sdn`](/modules/sdn/) IPAM выделяет IPv4-адреса из пулов дополнительным сетевым интерфейсам подов и виртуальных машин. Он включается для каждой сети отдельно ссылкой на пул адресов в [Network](/modules/sdn/cr.html#network) или [ClusterNetwork](/modules/sdn/cr.html#clusternetwork). Для [кластерных сетей](glossary.html#cluster-network) пулы создаёт администратор, для [сетей проекта](glossary.html#project-network) — пользователи. Если к поду подключено несколько сетей с IPAM, объекты адресов для каждого интерфейса указывают явно.

Адресами [виртуальных машин](glossary.html#virtual-machine) в [основной сети кластера](glossary.html#main-network) по умолчанию управляет модуль [`virtualization`](/modules/virtualization/) с помощью [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) и [VirtualMachineIPAddressLease](modules/virtualization/cr.html#virtualmachineipaddresslease). Если для основной сети настроен пул в модуле `sdn`, адресами машин управляет общий IPAM этого модуля: адрес запрашивается автоматически, а существующие машины переходят под его управление без смены адреса и без перезапуска.

Подробнее в разделах [«Дополнительные сети для использования в прикладных подах»](user/network/sdn/dedicated.html) и [«IP-адреса виртуальных машин»](user/network/virtualization/vm-ip-addresses.html).
