---
title: Установщик
description: Инструмент, который развёртывает кластер Kubernetes и устанавливает в него Deckhouse Platform. Доступен как CLI-установщик `dhctl` в образе контейнера и как графический установщик.
lang: ru
---

CLI-установщик `dhctl` развёртывает кластер в поддерживаемом облаке, на bare-metal-серверах, включая [гибридные кластеры](glossary.html#hybrid-cluster), или устанавливает Deckhouse Platform в существующий кластер Kubernetes. В [облачном кластере](glossary.html#cloud-cluster) он создаёт нужную инфраструктуру: виртуальные машины, сети и другие ресурсы. В работающем кластере команда `dhctl converge` применяет изменения конфигурации [узлов CloudPermanent](glossary.html#cloud-permanent-node) и изменения [защищённых параметров](glossary.html#protected-parameters).

Второй вариант — [графический установщик](/products/kubernetes-platform/gs/installer/), доступный на сайте Deckhouse Platform.

Подробнее в разделе [«Установка платформы»](installing/).
