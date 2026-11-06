---
title: Инстанс-класс
description: Ресурс конкретного провайдера, например OpenStackInstanceClass или AWSInstanceClass, который описывает параметры виртуальных машин, создаваемых Deckhouse Platform для группы узлов.
lang: ru
---

[Группа узлов](glossary.html#node-group) с [узлами CloudEphemeral](glossary.html#cloud-ephemeral-node) ссылается на инстанс-класс в параметре `cloudInstances.classReference`. Инстанс-класс задаёт тип машины, образ, диски и другие параметры провайдера, а количество узлов и их зоны задаёт группа узлов.

Подробнее в разделе [«Добавление и управление облачными узлами»](admin/configuration/platform-scaling/node/cloud-node.html).
