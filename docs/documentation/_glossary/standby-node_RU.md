---
title: Резервный узел
description: Предварительно подготовленный узел, который держится в кластере, чтобы новые реплики приложений запускались без ожидания создания узла при горизонтальном масштабировании. Количество резервных узлов задаётся параметром [cloudInstances.standby](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances).
lang: ru
---

Резервные узлы доступны для [узлов CloudEphemeral](glossary.html#cloud-ephemeral-node). Количество задаётся абсолютным значением или процентом от максимального количества узлов в [группе узлов](glossary.html#node-group). В группе уже должен работать хотя бы один узел. Долю ресурсов узла, занятую служебными компонентами вне Deckhouse Platform, задаёт параметр `standbyHolder.overprovisioningRate`. Резервные узлы увеличивают затраты на инфраструктуру.

Подробнее в разделе [«Добавление и управление облачными узлами»](admin/configuration/platform-scaling/node/cloud-node.html).
