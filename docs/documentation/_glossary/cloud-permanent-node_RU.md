---
title: Узел CloudPermanent
description: Постоянный облачный узел, конфигурация которого берётся из конфигурации кластера провайдера, а не из [NodeGroup](/modules/node-manager/cr.html#nodegroup). Master-узлы облачного кластера являются узлами CloudPermanent.
lang: ru
---

Узлы CloudPermanent описываются в секции `nodeGroups` структуры `<PROVIDER>ClusterConfiguration`, их количество задаётся параметром `replicas`. Узлами управляет Terraform, встроенный в Deckhouse Platform, а изменения конфигурации применяются запуском [установщика](glossary.html#installer) с командой `dhctl converge`.

Подробнее в разделе [«Добавление и управление облачными узлами»](admin/configuration/platform-scaling/node/cloud-node.html).
