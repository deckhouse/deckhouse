---
title: Системная сеть
description: Дополнительная сервисная сеть на узлах кластера, которую модуль [`sdn`](/modules/sdn/) создаёт поверх существующей underlay-сети для служебного трафика на уровне узла.
lang: ru
---

Системную сеть создают с помощью объекта [SystemNetwork](/modules/sdn/cr.html#systemnetwork). Интерфейсы, подключённые к системной сети, получают адреса в масштабе кластера через [ClusterIPAddress](/modules/sdn/cr.html#clusteripaddress), как и интерфейсы [кластерных сетей](glossary.html#cluster-network). Она относится к функциям [программно-определяемых сетей](glossary.html#sdn) уровня узла, вместе с настройкой интерфейсов узлов и [underlay-сетями](glossary.html#underlay-network).

Подробнее в разделе [«Программно-определяемые сети (SDN)»](admin/configuration/network/sdn/).
