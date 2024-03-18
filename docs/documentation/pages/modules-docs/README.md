---
title: "Модули Deckhouse"
permalink: en/modules-docs/
---

<p align="center">
<img src="../images/modules-docs/modules_logo.png" alt="deckhouse modules logo" />
</p>

## Введение

В этом репозитории представлена документация для создания собственного модуля Deckhouse.

## Полезно перед чтением

Чтобы понять принципы, по которым работают модули Deckhouse, ознакомьтесь с [addon-operator](https://github.com/flant/addon-operator) и [shell-operator](https://github.com/flant/addon-operator).

* 📚 Прочитайте документацию операторов о концепции хуков, например, [что такое конфигурация хука и какие функции она предоставляет](https://flant.github.io/shell-operator/HOOKS.html#hook-configuration). При помощи конфигурации настраиваются данные, которые будут доступны из хука.
* Ознакомьтесь с информацией о [биндингах](https://flant.github.io/addon-operator/HOOKS.html#bindings). Биндинги - это события, при которых срабатывает хук. Биндинги указываются в конфигурации хука. Хук может сработать не только из-за событий в Kubernetes, а также, например, по расписанию или перед запуском модуля.
> Хук позволяет сохранять значения в памяти и использовать их позже при рендеринге шаблонов Helm. Об этой особенности и цикле работы модуля, рекомендуем прочитать в [документации Hooks and Helm values](https://flant.github.io/addon-operator/OVERVIEW.html#hooks-and-helm-values).
* Ознакомьтесь с [концепцией снепшотов](https://flant.github.io/shell-operator/HOOKS.html#snapshots). С помощью снепшотов можно игнорировать отдельные события и реализовать шаблон reconciliation loop, при котором состояние из снепшота приводится к состоянию модуля.
 > Этим способом в Deckhouse Kubernetes Platform реализовано 100% поддержка хуков во внутренних модулях.
 * Кроме того, хуки можно использовать вместо `prometheus exporter`. Хуки могут предоставлять метрики, которые Deckhouse будет экспортировать. Ознакомьтесь с информацией [о метриках](https://flant.github.io/addon-operator/metrics/METRICS_FROM_HOOKS.html#custom-metrics).
* 🎬 В [видео 2019](https://www.youtube.com/watch?v=1_55KPHjVTU) года @andrey.polovov подробно рассказал о том, что такое shell-operator и addon-operator.
* 🎬 [Более подробное видео](https://www.youtube.com/watch?v=we0s4ETUBLc) про работу хуков на английском языке.
* 💡 В качестве примера можно посмотреть на [модули, разработанные компанией Flant](existing_modules/modules.md).

## Вопросы по документации

Если после прочтения документации у вас останутся вопросы, то для связи с командой разработки Deckhouse Kubernetes Platform лучше всего использовать Slack компании Флант. В канале `#tech-deckhouse-modules` вы сможете оставить свой вопрос, и команда поможет вам разобраться.
