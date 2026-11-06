---
title: Набор модулей
description: Именованный набор встроенных модулей Deckhouse Platform, которые включены по умолчанию. Default, Minimal или Managed.
lang: ru
---

Набор задаётся в параметре `bundle` [модуля](glossary.html#module) [`deckhouse`](/modules/deckhouse/) и не может быть изменён в развёрнутом кластере. `Default` — рекомендуемый набор для работы кластера: мониторинг, авторизация, сеть и другие базовые модули. `Managed` — набор для managed-кластеров облачных провайдеров. `Minimal` — минимальный набор.

Независимо от набора модуль можно явно включить или выключить в его [ModuleConfig](reference/api/cr.html#moduleconfig).

Подробнее в разделе [«Конфигурация Deckhouse Platform»](admin/configuration/).
