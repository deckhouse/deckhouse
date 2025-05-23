---
title: Хаос-инжиниринг
permalink: ru/admin/high-reliability-and-availability/chaos-engineering.html
description: Тестирование отказоустойчивости кластера
lang: ru
---

{% alert level="warning" %}
Режим хаос-инжиниринга можно включить только для групп узлов с [`nodeType: CloudEphemeral`](../../reference/cr/nodegroup.html#nodegroup-v1-spec-nodetype).
{% endalert %}

Включить режим хаос-инжиниринга для конкретных групп узлов (NodeGroup) можно одним из следующих способов:

- Укажите в параметрах тестируемой группы узлов параметр `spec.chaos` с двумя вложенными параметрами:

   ```yaml
   chaos:
     mode: DrainAndDelete
     period: 24h
   ```

   Здесь:

  * `mode` — режим работы, доступны два варианта:
    * `DrainAndDelete` — при срабатывании делает узлу drain, затем удаляет его.
    * `Disabled` — не трогает эту конкретную NodeGroup.
  * `period` — интервал времени срабатывания Chaos Monkey. Задается в виде строки с указанием часов и минут: `30m`, `1h`, `2h30m`, `24h`.

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
   ...
   ```

- Если в кластере включен модуль [`console`](/products/kubernetes-platform/modules/console/stable/), откройте веб-интерфейс Deckhouse, перейдите в настройки выбранной группы узлов в разделе «Узлы» — «Группы узлов» и включите Chaos Monkey в пункте «Параметры chaos monkey», указав временные интервалы в соответствующих полях.
