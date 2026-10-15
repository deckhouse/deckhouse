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

Marketplace — это система управления пакетами (packages) в Deckhouse Platform (DP). Она позволяет администраторам подключать репозитории пакетов, обнаруживать доступные пакеты и предоставлять пользователям проектов возможность установки приложений из этих пакетов.

{% alert level="info" %}
Marketplace доступен в DP, начиная с версии 1.76.
{% endalert %}

Архитектура Marketplace, включая основные концепции, разработку и настройку приложений, шаблоны, аннотации Nelm, хуки и жизненный цикл приложений, описана [в разделе «Архитектура»](../../../architecture/marketplace/).

Установка и удаление приложений из Marketplace, а также диагностика проблем описаны [в разделе «Использование»](../../../user/marketplace/).

## Задачи администратора

Администратор кластера:

1. Подключает репозиторий пакетов с помощью [ресурса PackageRepository](../../../reference/api/cr.html#packagerepository).
2. Следит за операциями сканирования репозиториев, в ходе которых Marketplace обнаруживает доступные пакеты.
3. Выдаёт пользователям права на просмотр версий пакетов и создание объектов Application в их неймспейсах.

Пользователи работают с пакетами с помощью [объекта Application](../../../reference/api/cr.html#application).

## Ресурсы Marketplace

Следующие ресурсы используются для работы с Marketplace:

| Ресурс | Короткое имя | Область | Описание |
|---|---|---|---|
| [PackageRepository](../../../reference/api/cr.html#packagerepository) | — | Кластер | Репозиторий пакетов и параметры его сканирования |
| [PackageRepositoryOperation](../../../reference/api/cr.html#packagerepositoryoperation) | `pro` | Кластер | Операция сканирования репозитория |
| [ApplicationPackageVersion](../../../reference/api/cr.html#applicationpackageversion) | `apv` | Кластер | Обнаруженная версия пакета |
| [ApplicationPackage](../../../reference/api/cr.html#applicationpackage) | `ap` | Кластер | Сводная информация о пакете: репозитории, в которых он доступен, версии, на которые указывают каналы обновлений, и приложения, которые его используют |
| [Application](../../../reference/api/cr.html#application) | — | Неймспейс | Установленный экземпляр приложения, которым управляет пользователь |

В разделе [«Репозитории пакетов»](package-repository.html) описан порядок подключения репозитория и проверки его состояния.

В разделе [«Сканирование»](scanning.html) описан мониторинг операций сканирования и порядок запуска сканирования вручную.
