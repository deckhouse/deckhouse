---
title: Хук модуля
description: Исполняемый файл модуля или пакета Marketplace, который Deckhouse Platform запускает в ответ на событие. Хуки подготавливают values для шаблонов, создают и изменяют объекты в кластере и проверяют настройки.
lang: ru
---

Хук запускается, когда срабатывает один из его [биндингов](glossary.html#hook-binding): при старте, до или после применения Helm-шаблонов, по расписанию или при изменении отслеживаемых объектов Kubernetes. Хуки модуля находятся в директории `/hooks/batch`. Хуки [пакета Marketplace](glossary.html#marketplace-package) пишутся на Go с module-sdk и собираются в один бинарный файл. Существуют хуки модулей и глобальные хуки. [Контроллер Deckhouse](glossary.html#deckhouse-controller) выполняет их как задачи [очередей addon-operator](glossary.html#addon-operator-queue).

Подробнее в разделе [«Структура модуля»](architecture/module-development/structure/).
