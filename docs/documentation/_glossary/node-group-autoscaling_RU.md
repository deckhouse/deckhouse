---
title: Автомасштабирование группы узлов
description: Автоматическое изменение количества узлов в группе компонентом Cluster Autoscaler модуля [`node-manager`](/modules/node-manager/) на основе потребности в CPU и памяти. Срабатывает только тогда, когда поды в состоянии Pending не могут быть размещены на существующих узлах.
lang: ru
---

Автомасштабирование доступно для [узлов CloudEphemeral](glossary.html#cloud-ephemeral-node). Границы задаются в секции `cloudInstances` [группы узлов](glossary.html#node-group): `minPerZone` и `maxPerZone` ограничивают количество узлов в зоне, `maxUnavailablePerZone` ограничивает количество недоступных узлов при изменениях, `priority` определяет, какие группы Cluster Autoscaler масштабирует первыми. Автомасштабирование не распространяется на [статические узлы](glossary.html#static-node): для них [CAPS](glossary.html#caps) поддерживает количество, заданное параметром `staticInstances.count`.

Подробнее в разделе [«Добавление и управление облачными узлами»](admin/configuration/platform-scaling/node/cloud-node.html).
