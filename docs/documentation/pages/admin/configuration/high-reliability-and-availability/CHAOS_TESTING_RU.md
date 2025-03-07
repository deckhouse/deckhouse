---
title: Хаос инжиниринг
permalink: ru/admin/high-reliability-and-availability/chaos-engineering.html
description: Тестирование отказоустройчивости кластера
lang: ru
---

{% alert level="warning" %}
Режим хаос-инжиниринга можно включить только для групп узлов с `nodeType: CloudEphemeral`.
{% endalert %}

Включить режим хаос-инжиниринга возможно для конкретных групп узлов (NodeGroup):

1. Укажите в параметры тестируемой группы узлов параметр `spec.chaos` с двумя параметрами:

   ```yaml
   chaos:
     mode: DrainAndDelete
     period: 24h
   ```

   Здесь:
   
   * `mode` — режим работы, доступны два варианта:
     * `DrainAndDelete` — при срабатывании делает узлу drain, затем удаляет его.
     * `Disabled` — не трогает эту конкретную NodeGroup.
   * `period` — интервал времени срабатывания Chaos Monkey, задается в виде строки с указанием часов и минут: `30m`, `1h`, `2h30m`, `24h`.
   
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

2. Если в кластере включена [Console](/products/kubernetes-platform/modules/console/stable/), перейдите в настройки выбранное гроуппы узлов в разделе «Узлы» — «Группы узлов», выберите там нужную группу и включите Chaos Monkey в пункте «Параметры chaos monkey», указав временные интервалы в соответствующих полях.
