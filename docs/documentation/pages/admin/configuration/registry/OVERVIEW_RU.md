---
title: Хранилище образов компонентов DKP
permalink: ru/admin/configuration/registry/
description: "Хранилище образов компонентов DKP: настройка взаимодействия и использование."
lang: ru
search: container registry, registry configuration, edition management, registry management, container images, хранилище образов контейнеров
---

В этом разделе описываются настройки для взаимодействия с хранилищем образов компонентов DKP.

В разделе описаны процессы настройки DKP в уже работающем кластере. Информация о настройках при установке кластера приведена в разделе [«Установка платформы»](../../../installing/).

Возможности и процессы настройки хранилища образов компонентов DKP зависят от способа управления кластером. В кластерах, полностью управляемых DKP, управление настройками реализуется с помощью модуля [`registry`](/modules/registry/) (подробнее — в разделе [«Настройки в кластере, управляемом DKP»](managing-interaction.html)). В Managed Kubernetes кластерах применяется `helper change-registry`, модуль `registry` не используется (подробнее — в разделе [Настройки в Managed Kubernetes кластере»](third-party.html)).
