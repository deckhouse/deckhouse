---
title: Работа с container registries и редакциями
permalink: ru/admin/configuration/registry/
description: "Настройка и управление container registry в Deckhouse Kubernetes Platform. Внутренний registry, интеграция с внешними registry и переключение редакций."
lang: ru
search: container registry, registry configuration, edition management, registry management, container images, управление редакциями
---

В этом разделе описывается работа с container registries (registry) и редакциями в Deckhouse Kubernetes Platform.

## Работа с registry

В разделе рассматривается работа с registry в функционирующем кластере. Если вас интересует информация про работу с registry при установке кластера, перейдите в раздел [«Установка платформы»](../../../installing/).

В разделе [«Сторонний registry»](../registry/third-party.html) описан процесс переключения работающего кластера DKP на использование стороннего registry.
В разделе [«Внутренний registry»](../registry/internal.html) рассматривается подготовка к переключению между режимами работы кластера: с использованием внутреннего container registry или без использования, а также процессы переключения.
В разделе [«Восстановление подключения к registry»](../registry/restore-token.html) описан процесс восстановления загрузки образов Deckhouse Kubernetes Platform при истекшем лицензионном токене.

## Работа с редакциями

В разделе [«Переключение редакций»](../registry/switching-editions.html) описаны возможные варианты переключения редакций в работающем кластере Deckhouse Kubernetes Platform.
