---
title: Источник модулей
description: Хранилище образов контейнеров, из которого Deckhouse Platform получает сведения о модулях, не входящих в состав платформы. Задаётся с помощью объекта [ModuleSource](reference/api/cr.html#modulesource) с адресом хранилища и параметрами аутентификации.
lang: ru
---

[Модули из источника](glossary.html#module-from-source) публикуются в хранилище как [артефакты модуля](glossary.html#module-artifacts). Deckhouse Platform синхронизируется с источником и создаёт релизы модулей по [политике обновления модуля](glossary.html#module-update-policy). Затем модуль включается с помощью [ModuleConfig](reference/api/cr.html#moduleconfig). В [изолированном кластере](glossary.html#air-gapped-cluster) источник указывает на внутрикластерное или приватное хранилище.

Подробнее в разделе [«Как запустить и проверить модуль в кластере DP»](architecture/module-development/run/).
