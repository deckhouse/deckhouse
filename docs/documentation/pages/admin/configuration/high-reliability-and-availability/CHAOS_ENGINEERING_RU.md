---
title: Хаос-инжиниринг
permalink: ru/admin/configuration/high-reliability-and-availability/chaos-engineering.html
description: Тестирование отказоустойчивости кластера
lang: ru
---

{% alert level="warning" %}
Режим хаос-инжиниринга включается только для групп узлов с [`nodeType: CloudEphemeral`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetype).
{% endalert %}

Включите режим хаос-инжиниринга для конкретных групп узлов (NodeGroup) одним из следующих способов:

1. Укажите в параметрах тестируемой группы узлов [параметр `spec.chaos`](/modules/node-manager/cr.html#nodegroup-v1-spec-chaos) с двумя вложенными параметрами:

   ```yaml
   chaos:
     mode: DrainAndDelete
     period: 24h
   ```

   Здесь:

   * `mode` — режим работы, доступны два варианта:
     * `DrainAndDelete` — при срабатывании выполняет drain узла, затем удаляет его.
     * `Disabled` — не воздействует на данную NodeGroup.
   * `period` — интервал между срабатываниями Chaos Monkey. Задается в виде строки с указанием часов и минут: `30m`, `1h`, `2h30m`, `24h`.

   Пример настройки для группы узлов:

   ```yaml
   # NodeGroup для облачных узлов в AWS.
   apiVersion: deckhouse.io/v1
   kind: NodeGroup
   metadata:
     name: test
   spec:
     nodeType: CloudEphemeral
     chaos:
       mode: DrainAndDelete
       period: 24h
   ```

1. Если в кластере включен модуль [`console`](/modules/console/), откройте веб-интерфейс Deckhouse, перейдите в настройки выбранной группы узлов в разделе «Узлы» — «Группы узлов» и включите Chaos Monkey в пункте «Параметры chaos monkey», указав временные интервалы в соответствующих полях.
