---
title: Хаос-инжиниринг
description: Управляемый или случайный вывод из строя компонентов кластера для проверки его устойчивости. В Deckhouse Platform включается для группы узлов параметром [spec.chaos](/modules/node-manager/cr.html#nodegroup-v1-spec-chaos), только для узлов CloudEphemeral.
lang: ru
---

Параметр `spec.chaos` [группы узлов](glossary.html#node-group) с [узлами CloudEphemeral](glossary.html#cloud-ephemeral-node) задаёт режим и период. В режиме `DrainAndDelete` Chaos Monkey раз в интервал `period` освобождает от нагрузки и удаляет один узел группы. Режим `Disabled` пропускает группу. Режим также включается в веб-интерфейсе в настройках группы узлов.

Подробнее в разделе [«Хаос-инжиниринг»](admin/configuration/high-reliability-and-availability/chaos-engineering.html).
