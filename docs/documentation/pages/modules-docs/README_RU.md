---
title: "Модули Deckhouse Kubernetes Platform"
permalink: ru/modules-docs/
lang: ru
---

<p align="center">
<img src="../images/modules-docs/modules_logo.png" alt="deckhouse modules logo" />
</p>

## Введение

В этом репозитории представлена документация для создания собственного модуля Deckhouse.

## Полезно перед чтением

Принципы, по которым работают модули Deckhouse, можно понять ознакомившись с [addon-operator](https://github.com/flant/addon-operator) и [shell-operator](https://github.com/flant/addon-operator).

* 📚 Про концепцию хуков можно почерпнуть из документации операторов
  * [Что такое конфигурация хука и какие есть опции](https://flant.github.io/shell-operator/HOOKS.html#hook-configuration). При помощи конфигурации мы настраиваем как будет выглядеть данные, которые будут доступны из хука.
  * Отдельного внимания достойны [биндинги](https://flant.github.io/addon-operator/HOOKS.html#bindings) -  события, при которых будет срабатывать хук. Биндинги указываются в конфигурации хука. Хук может срабатывать не тольк из-за событий в Kubernetes, но и, например, по расписанию, или каждый раз перед запуском модуля.
  * Хук может сохранять значения в память и использовать потом при рендере шаблонов Helm. Подробнее про эту особенность и цикл работы модуля можно прочитать в [этой документации](https://flant.github.io/addon-operator/OVERVIEW.html#hooks-and-helm-values).
  * Обязательного ознакомления требует [концепция снепшотов](https://flant.github.io/shell-operator/HOOKS.html#snapshots). При помощи снепшотов можно перестать реагировать на отдельные события и реализовать паттерн reconciliation loop, приводя состояние из снепшота к состоянию модуля.
      > Этим способом в Deckhouse реализовано 100% хуков во внутренних модулях.
  * Можно использовать хук как замену `prometheus exporter`. Хуки могут возвращать метрики, которые будет экспортировать Deckhouse. Подробнее о метриках можно [прочитать тут](https://flant.github.io/addon-operator/metrics/METRICS_FROM_HOOKS.html#custom-metrics).
* 🎬 В [видео 2019](https://www.youtube.com/watch?v=1_55KPHjVTU) года @andrey.polovov подробно рассказал о том, что такое shell-operator и addon-operator.
* 🎬 [Более подробное видео](https://www.youtube.com/watch?v=we0s4ETUBLc) про работу хуков на английском языке.
* 💡 Для вдохновения можно посмотреть [модули, сделаные компанией Flant](existing_modules/modules.md).

## Вопросы

На случай если у вас остались вопросы после прочтения документации, основным способом для связи является Slack компании Флант, канал `#tech-deckhouse-modules`. Там вы можете задать свой вопрос команде разработки Deckhouse.
