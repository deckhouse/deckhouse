---
title: Marketplace
permalink: ru/admin/configuration/marketplace/
description: "Настройка и управление Marketplace в Deckhouse Platform. Подключение репозиториев пакетов, мониторинг операций сканирования и предоставление пользователям доступа к пакетам приложений."
lang: ru
search: marketplace, package repository, packages, пакеты, репозиторий пакетов, приложения
relatedLinks:
  - title: "Использование Marketplace"
    url: ../../../user/marketplace/
---

Marketplace — это система управления единицами поставки Deckhouse Platform (DP) (Packages). Она позволяет администраторам подключать хранилища образов с пакетами, обнаруживать доступные пакеты и открывать пользователям проектов возможность их установки.

{% alert level="info" %}
Marketplace доступен начиная с DP версии 1.76.
{% endalert %}

## Задачи администратора

Администратор кластера:

1. Подключает хранилище образов с пакетами, создавая ресурс [PackageRepository](package-repository.html).
2. Следит за операциями сканирования, которые обнаруживают пакеты в хранилище образов.
3. Выдаёт пользователям права на просмотр версий пакетов и на создание объектов Application в их неймспейсах. `ApplicationPackage` и `ApplicationPackageVersion` — ресурсы уровня кластера.

Пользователи работают с пакетами через объект Application (подробнее — в разделе [«Использование → Marketplace»](../../../user/marketplace/)).

## Ключевые ресурсы

| Ресурс | Короткое имя | Область | Описание |
|---|---|---|---|
| [`PackageRepository`](../../../reference/api/cr.html#packagerepository) | — | Cluster | Хранилище образов с пакетами и параметры его сканирования |
| [`PackageRepositoryOperation`](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Cluster | Операция сканирования репозитория |
| [`ApplicationPackageVersion`](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Cluster | Обнаруженная версия пакета |
| [`ApplicationPackage`](../../../reference/api/cr.html#applicationpackage) | `ap` | Cluster | Сводная информация о пакете: репозитории, в которых он доступен, версии, на которые указывают каналы обновлений, и приложения, которые его используют |
| [`Application`](../../../reference/api/cr.html#application) | — | Namespace | Установленный экземпляр приложения (управляется пользователями) |

В разделе [«Репозитории пакетов»](package-repository.html) описаны подключение хранилища образов и проверка состояния репозитория.

В разделе [«Сканирование»](scanning.html) описаны мониторинг операций сканирования и запуск сканирования вручную.
