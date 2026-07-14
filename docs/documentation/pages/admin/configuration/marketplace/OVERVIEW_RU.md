---
title: Marketplace
permalink: ru/admin/configuration/marketplace/
description: "Настройка и управление Marketplace в Deckhouse Kubernetes Platform. Подключение репозиториев пакетов, мониторинг операций сканирования и предоставление пользователям доступа к приложениям."
lang: ru
search: marketplace, package repository, packages, пакеты, репозиторий пакетов, приложения
relatedLinks:
  - title: "Использование Marketplace"
    url: ../../../user/marketplace/
---

Marketplace — это система управления единицами поставки Deckhouse Kubernetes Platform (DKP) (Packages). Позволяет администраторам подключать реестры пакетов, обнаруживать доступные пакеты и открывать пользователям проектов возможность их установки.

{% alert level="info" %}
Marketplace доступен начиная с DKP версии 1.76.
{% endalert %}

## Задачи администратора

Администратор кластера:

- Подключает реестры пакетов (создавая объекты  [PackageRepository](package-repository.html)).
- Следит за операциями сканирования, которые обнаруживают пакеты в реестре.
- Обеспечивает пользователям доступ к версиям пакетов для установки в свои неймспейсы.

Пользователи взаимодействуют с пакетами через объект Application (подробнее в разделе [Использование → Marketplace](../../../user/marketplace/)).

## Ключевые ресурсы

| Ресурс | Короткое имя | Область | Описание |
|---|---|---|---|
| [`PackageRepository`](../../../reference/api/cr.html#packagerepository) | — | Cluster | Исходный реестр пакетов |
| [`PackageRepositoryOperation`](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Операция сканирования репозитория |
| [`ApplicationPackageVersion`](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Обнаруженная версия пакета |
| [`ApplicationPackage`](../../../reference/api/cr.html#applicationpackage) | — | Cluster | Агрегированные метаданные пакета |
| [`Application`](../../../reference/api/cr.html#application) | `app` | Namespace | Установленный экземпляр приложения (управляется пользователями) |

В разделе [«Репозитории пакетов»](package-repository.html) описано подключение реестра и проверка его статуса.

В разделе [«Сканирование»](scanning.html) описан мониторинг операций сканирования и запуск сканирования вручную.
