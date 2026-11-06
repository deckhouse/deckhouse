---
title: Узел CloudEphemeral
description: Временный облачный узел, который Deckhouse Platform заказывает, создаёт и удаляет автоматически через API облачного провайдера. Описывается в [NodeGroup](/modules/node-manager/cr.html#nodegroup) с типом узлов CloudEphemeral.
lang: ru
---

Параметры виртуальных машин берутся из [инстанс-класса](glossary.html#instance-class), количество узлов и их зоны — из секции `cloudInstances` [группы узлов](glossary.html#node-group). Узлы создаёт Machine Controller Manager или Cluster API, входящие в модуль [`node-manager`](/modules/node-manager/).

Для групп CloudEphemeral доступны [автомасштабирование группы узлов](glossary.html#node-group-autoscaling), [резервные узлы](glossary.html#standby-node) и [хаос-инжиниринг](glossary.html#chaos-engineering).

Подробнее в разделе [«Добавление и управление облачными узлами»](admin/configuration/platform-scaling/node/cloud-node.html).
