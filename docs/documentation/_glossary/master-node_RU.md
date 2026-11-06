---
title: Master-узел
description: Узел, на котором работают компоненты control plane. Master-узлы имеют лейбл `node-role.kubernetes.io/control-plane`, на них запускается модуль [`control-plane-manager`](/modules/control-plane-manager/).
lang: ru
---

Deckhouse Platform управляет [control plane](glossary.html#control-plane) на master-узлах: сертификатами, конфигурацией компонентов, версиями, кластером etcd и файлами kubeconfig. В облачном кластере master-узлы являются [узлами CloudPermanent](glossary.html#cloud-permanent-node).

Количество master-узлов определяет, является кластер [кластером с одним master-узлом](glossary.html#single-master) или [мультимастерным](glossary.html#multi-master). При нескольких master-узлах [режим HA](glossary.html#ha-mode) включается автоматически.

Подробнее в разделе [«Общее управление и настройка control plane»](admin/configuration/platform-scaling/control-plane/control-plane-management-and-configuration.html).
