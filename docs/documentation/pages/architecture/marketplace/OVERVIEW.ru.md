---
title: Marketplace
permalink: ru/architecture/marketplace/
description: "Архитектура подсистемы Deckhouse Kubernetes Platform Marketplace. Абстракция Package, типы единиц поставки, модель ресурсов и обзор подсистемы."
lang: ru
search: marketplace architecture, package abstraction, application module, архитектура marketplace, абстракция package
---

Marketplace — это подсистема Deckhouse Kubernetes Platform (DKP), управляющая жизненным циклом единиц поставки, называемых **Packages** (пакетами). Package может быть **Application** (пользовательская нагрузка, развёртываемая в неймспейс) или **Module** (расширение возможностей кластера). В настоящее время поддерживаются только Applications; поддержка Module запланирована на будущую версию.

Marketplace доступен начиная с DKP версии 1.76.

## Разделы

В разделе [«Концепции»](concepts.html) описаны абстракция Package, модель ресурсов, ограничения Application и полный жизненный цикл от сканирования до деплоя.

В разделе [«Разработка приложений»](application-development.html) описано создание пакета Application с нуля: бутстрап, структура проекта, `package.yaml`, CI/CD и организация артефактов в реестре.

В разделе [«Аннотации Nelm»](nelm-annotations.html) рассматривается полный набор Nelm-аннотаций, используемых в шаблонах Application для управления порядком деплоя, жизненным циклом ресурсов, трекингом и логированием.

В разделе [«Хуки»](hooks.html) описано написание Go-хуков для Applications с использованием типа `ApplicationHookInput` из module-sdk.
