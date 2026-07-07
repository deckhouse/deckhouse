---
title: Модуль deckhouse
permalink: ru/architecture/deckhouse/deckhouse.html
lang: ru
search: deckhouse, deckhouse-controller, modules
description: Архитектура модуля deckhouse в Deckhouse Kubernetes Platform.
---

Модуль [`deckhouse`](/modules/deckhouse/) реализует ядро Deckhouse Kubernetes Platform (DKP) и выполняет следующие операции:
- обновление платформы;
- управление конфигурацией модулей;
- установка и обновление модулей;
- запуск сборки документации модулей;
- валидация кастомных ресурсов, находящихся под управлением модулей DKP.

Модуль управляет следующими кастомными ресурсами API-группы `deckhouse.io`:

- управление модулями:
  - [Module](../../reference/api/cr.html#module) — описание, статус и публикация информации о модуле;
  - [ModuleConfig](../../reference/api/cr.html#moduleconfig) — описание пользовательских настроек для модулей;
  - [ModulePullOverride](../../reference/api/cr.html#modulepulloverride) — описание исключений для выбора версий модулей;
  - [ModuleRelease](../../reference/api/cr.html#modulerelease) — описание, публикация и отслеживание релизов модулей;
  - [ModuleSettingsDefinition](../../reference/api/cr.html#modulesettingsdefinition) — схема, версии и правила преобразования настроек модуля;
  - [ModuleSource](../../reference/api/cr.html#modulesource) — описание источника, репозитория или хранилища модулей;
  - [ModuleUpdatePolicy](../../reference/api/cr.html#moduleupdatepolicy) — правила обновления и автоматизации переходов версий модулей;

- управление платформой:
  - [DeckhouseRelease](../../reference/api/cr.html#deckhouserelease) — объект, определяющий релиз (версию) Deckhouse и политику обновления платформы;

- управление пакетами ([Marketplace](../marketplace)):
  - [Application](../../reference/api/cr.html#application) — описание и желаемое состояние прикладного пакета (группы компонентов или приложения);
  - [ApplicationPackage](../../reference/api/cr.html#applicationpackage) — метаданные, источники и настройки пакета;
  - [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion) — описание конкретной версией пакета и ее параметров;
  - [PackageRepository](../../reference/api/cr.html#packagerepository) — объект, описывающий источник репозиториев пакетов и их параметры;
  - [PackageRepositoryOperation](../../reference/api/cr.html#packagerepositoryoperation) — операции над репозиториями пакетов, такие как синхронизация или обновление;

- управление утилитами:
  - [CNIMigration](../../reference/api/cr.html#cnimigration) — процесс миграции сетевого плагина [Container Network Interface (CNI)](https://github.com/containernetworking/cni), содержит параметры и статус миграции;
  - [CNINodeMigration](../../reference/api/cr.html#cninodemigration) — статус и управление миграцией CNI на уровне отдельных узлов;
  - ObjectKeeper — ресурс, обеспечивающий связь между другими ресурсами Kubernetes с использованием `ownerReference`;
  - [ModuleDocumentation](../../reference/api/cr.html#moduledocumentation) — описание параметров для генерации и хранения документации модулей;

- управление кастомными ресурсами под управлением модулей DKP:
  - [ConversionWebhook](/modules/deckhouse/latest/cr.html#conversionwebhook) — настройки и обработчики вебхуков для конверсий ресурсов;
  - [ValidationWebhook](/modules/deckhouse/latest/cr.html#validationwebhook) — настройки и обработчики вебхуков для валидации ресурсов.

## Архитектура модуля

{% alert level="info" %}
Для упрощения схемы приняты следующие допущения:

* На схеме показано, что контейнеры разных подов взаимодействуют друг с другом напрямую. Фактически они взаимодействуют через соответствующие сервисы Kubernetes (внутренние балансировщики). Названия сервисов не указываются, если они очевидны из контекста. В остальных случаях название сервиса указано над стрелкой.
* Поды могут быть запущены в нескольких репликах, однако на схеме все поды изображены в одной реплике.
{% endalert %}

Архитектура модуля [`deckhouse`](/modules/deckhouse/) на уровне 2 модели C4 и его взаимодействие с другими компонентами DKP изображены на следующей диаграмме:

![Архитектура модуля deckhouse](../../images/architecture/deckhouse/c4-l2-deckhouse-deckhouse.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Deckhouse** (Deployment) — контроллер, реализующий операции по управлению платформой.

   Контроллер оркестрирует задачи по управлению платформой с использованием [механизма очередей](./queues.html).

   Контроллер Deckhouse может быть запущен в стандартном режиме или в режиме изоляции [хуков](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md). Для этого необходимо создать ConfigMap `chroot-mode` в неймспейсе `d8-system`. В режиме изоляции shell-хуки и скрипты включения модулей выполняются в chroot-окружении с ограниченным набором смонтированных каталогов, что изолирует их от файловой системы контейнера контроллера.

   Если включен режим [высокой доступности (High Availability, HA)](../../admin/configuration/high-reliability-and-availability/), запускается несколько экземпляров контроллера Deckhouse. Для обеспечения корректной работы контроллеры Deckhouse проводят выборы лидера с использованием ресурса Lease `deckhouse-leader-election`. Контроллер, который был избран как лидер, берёт на себя выполнение всех операций по управлению платформой.

   Кроме того, контроллер Deckhouse настраивает:

   | Описание       | Параметр в конфигурации модуля                |
   |-------------- |-------------------------------------- |
   | Уровень логирования          | [`.spec.settings.logLevel`](/modules/deckhouse/configuration.html#parameters-loglevel)   |
   | Набор модулей, включенных по умолчанию | [`.spec.settings.bundle`](/modules/deckhouse/configuration.html#parameters-bundle)   |
   | Канал обновлений | [`.spec.settings.releaseChannel`](/modules/deckhouse/configuration.html#parameters-releasechannel)   |
   | Режим обновлений | [`.spec.settings.update.mode`](/modules/deckhouse/configuration.html#parameters-update-mode)   |
   | Окна обновлений | [`.spec.settings.update.windows.days`](/modules/deckhouse/configuration.html#parameters-update-windows)   |

   Подробнее с описанием настроек модуля можно ознакомиться [в разделе документации модуля](/modules/deckhouse/).

   Состоит из следующих контейнеров:

   * **init-downloaded-modules** — init-контейнер, подготавливающий структуру каталогов для работы с модулями;
   * **deckhouse** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к интерфейсу отладки компонентов основного контейнера.
  
1. **Webhook-handler** (Deployment) — состоит из одного контейнера **handler** и реализует универсальный вебхук для конверсий и валидации кастомных ресурсов, находящихся под управлением DKP.

    Компонент следит за кастомными ресурсами [ConversionWebhook](/modules/deckhouse/latest/cr.html#conversionwebhook) и [ValidationWebhook](/modules/deckhouse/latest/cr.html#validationwebhook) и на их основе создаёт из шаблона Python-файлы хуков для [shell-operator](https://github.com/flant/shell-operator). При получении запросов от `kube-apiserver` на валидацию или конверсию ресурсов shell-operator запускает необходимый хук и возвращает результат обработки.

1. **Cni-migration-manager** (Deployment) — опциональный компонент, запускающийся на control-plane узлах и состоящий из одного контейнера **manager**. Компонент управляет процессом смены сетевого плагина (CNI) в кластере DKP и фиксирует текущее состояние в кастомном ресурсе CNIMigration. Поддерживается миграция на плагины Flannel, Simple bridge, Cilium. Подробнее с переключением CNI в кластере можно ознакомиться [в соответствующем руководстве](/products/kubernetes-platform/guides/cni-migration.html).

    {% alert level="info" %}
    Компонент создаётся глобальным хуком `detect-cni-migration` при наличии кастомного ресурса CNIMigration. Ресурс CNIMigration создаётся администратором вручную или при использовании команды `d8 network cni-migration switch --to-cni <target cni>`.
    {% endalert %}

1. **Cni-migration-agent** (DaemonSet) — опциональный компонент, запускающийся на всех узлах кластера и состоящий из одного контейнера **agent**. Компонент следит за кастомным ресурсом CNIMigration и управляет кастомным ресурсом CNINodeMigration, отражающим текущее состояние миграции на конкретном узле.

    {% alert level="info" %}
    Компонент создаётся глобальным хуком `detect-cni-migration` при наличии кастомного ресурса CNIMigration. Ресурс CNIMigration создаётся администратором вручную или при использовании команды `d8 network cni-migration switch --to-cni <target cni>`.
    {% endalert %}

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:
   - работа с кастомными ресурсами API-группы `deckhouse.io`;
   - отслеживание ресурсов Pod и DaemonSet, а также перезапуск Pod при смене сетевого плагина;
   - отслеживание ресурсов, описанных в кастомном ресурсе ObjectKeeper;
   - создание и обновление ресурса Lease;
   - создание, удаление, изменение и отслеживание ресурсов, описанных в модулях DKP;
   - авторизация запросов.

1. [**Documentation**](/modules/documentation/) — обновление документации при добавлении или обновлении модуля DKP.

1. **Хранилище образов** — получение образов компонент модулей вместе с метаданными, в случае если модуль [`registry`](/modules/registry/) установлен в режиме Unmanaged.

1. **Модуль `registry`** — получение образов компонент модулей вместе с метаданными, в случае если модуль [`registry`](/modules/registry/) установлен в одном из режимов Direct, Proxy или Local.

С модулем взаимодействуют следующие внешние компоненты:

* **Kube-apiserver** — валидация и конверсия кастомных ресурсов DKP.
* **Prometheus-main** — сбор метрик с контейнеров `deckhouse` и `webhook-handler`.
