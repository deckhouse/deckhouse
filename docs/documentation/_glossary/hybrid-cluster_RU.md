---
title: Гибридный кластер
description: Статический кластер Deckhouse Platform, расширенный узлами в инфраструктуре поддерживаемого облачного провайдера.
lang: ru
---

Control plane и первоначальные узлы остаются частью [статического кластера](glossary.html#static-cluster). Дополнительные узлы добавляются через модуль [облачного провайдера](glossary.html#cloud-provider): либо создаются автоматически через API провайдера как [узлы CloudEphemeral](glossary.html#cloud-ephemeral-node), либо создаются заранее и подключаются [bootstrap-скриптом](glossary.html#bootstrap-script) как [узлы CloudStatic](glossary.html#cloud-static-node).

Гибридному кластеру нужна L3-связность между дата-центром и облаком.

Подробнее в разделе [«Гибридные кластеры»](admin/integrations/hybrid/overview.html).
