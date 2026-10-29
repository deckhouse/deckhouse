---
title: Доступность модуля
description: Настройки accessibility в module.yaml, которые определяют, в каких редакциях Deckhouse Platform модуль доступен и в каких наборах модулей включён по умолчанию.
lang: ru
---

Настройки задаются для каждой [редакции](glossary.html#edition), блок `_default` действует для редакций без собственных настроек: `available` определяет, можно ли использовать модуль в редакции, `enabledInBundles` перечисляет [наборы модулей](glossary.html#module-bundle) (`Default`, `Managed`, `Minimal`), в которых модуль включён без [ModuleConfig](reference/api/cr.html#moduleconfig).

Подробнее в разделе [«Структура модуля»](architecture/module-development/structure/).
