---
title: Модуль deckhouse
permalink: ru/architecture/deckhouse/deckhouse.html
lang: ru
search: deckhouse, deckhouse-controller, modules
description: Архитектура модуля deckhouse в Deckhouse Kubernetes Platform.
---

Модуль [`deckhouse`](/modules/deckhouse/) реализует ядро Deckhouse Kubernetes Platform (DKP), выполняющее следующие операции:
- обновление платформы;
- управление конфигурацией модулей;
- установка и обновление модулей;
- сборка документации модулей;
- валидация кастомных ресурсов, находящихся под управлением модулей DKP.

Модуль управляет следующими кастомными ресурсами API-группы `deckhouse.io`:

- управление модулями:
  - [Module](../../reference/api/cr.html#module) — описание, статус и публикация информации о модуле;
  - [ModuleConfig](../../reference/api/cr.html#moduleconfig) — хранение и применение пользовательских настроек для модулей;
  - [ModulePullOverride](../../reference/api/cr.html#modulepulloverride) — управление исключениями для выбора версий модулей;
  - [ModuleRelease](../../reference/api/cr.html#modulerelease) — описание, публикация и отслеживание релизов модулей;
  - [ModuleSettingsDefinition](../../reference/api/cr.html#modulesettingsdefinition) — схема, версии и правила преобразования настроек модуля;
  - [ModuleSource](../../reference/api/cr.html#modulesource) — определяет источник, репозиторий или хранилище модулей;
  - [ModuleUpdatePolicy](../../reference/api/cr.html#moduleupdatepolicy) — правила обновления и автоматизации переходов версий модулей;

- управление платформой:
  - [DeckhouseRelease](../../reference/api/cr.html#deckhouserelease) — объект, определяющий релиз (версию) Deckhouse и политику обновления платформы;

- управление пакетами ([Marketplace](../marketplace)):
  - [Application](../../../latest/reference/api/cr.html#application) — описание и желаемое состояние прикладного пакета (группы компонентов или приложения);
  - [ApplicationPackage](../../../latest/reference/api/cr.html#applicationpackage) — определяет метаданные, источники и настройки пакета;
  - [ApplicationPackageVersion](../../../latest/reference/api/cr.html#applicationpackageversion) — управление конкретной версией пакета и ее параметрами;
  - [PackageRepository](../../../latest/reference/api/cr.html#packagerepository) — объект, описывающий источник репозиториев пакетов и их параметры;
  - [PackageRepositoryOperation](../../../latest/reference/api/cr.html#packagerepositoryoperation) — операции над репозиториями пакетов, такие как синхронизация или обновление;

- управление утилитами:
  - [CNIMigration](../../../latest/reference/api/cr.html#cnimigration) — процесс миграции сетевого плагина [Container Network Interface (CNI)](https://github.com/containernetworking/cni), содержит параметры и статус миграции;
  - [CNINodeMigration](../../../latest/reference/api/cr.html#cninodemigration) — статус и управление миграцией CNI на уровне отдельных нод;
  - ObjectKeeper — ресурс, обеспечивающий связь между другими ресурсами Kubernetes с использованием `ownerReference`;
  - [ModuleDocumentation](../../reference/api/cr.html#moduledocumentation) — генерация и хранение документации модулей;

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

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Архитектура модуля deckhouse](../../images/architecture/deckhouse/c4-l2-deckhouse-deckhouse.ru.svg)

## Компоненты модуля

Модуль состоит из следующих компонентов:

1. **Deckhouse** (Deployment) — контроллер, реализующий следующие действия по управлению платформой:
   - обновление платформы;
   - управление конфигурацией модулей;
   - установка и обновление модулей;
   - сборка документации модулей;
   - отслеживание и удаление кастомных ресурсов ObjectKeeper.

   Состоит из следующих контейнеров:

   * **init-downloaded-modules** — инит-контейнер, подготавливающий структуру каталогов для работы с модулями;
   * **deckhouse** — основной контейнер;
   * **kube-rbac-proxy** — сайдкар-контейнер с авторизующим прокси на основе Kubernetes RBAC для организации защищенного доступа к интерфейсу debug HTTP основного контейнера.

   Контроллер оркестрирует задачи по управлению платформой с использованием механизма очередей. Подробнее можно ознакомиться [далее](#очередь-deckhouse).

   Контроллер Deckhouse может быть запущен в стандартном режиме, либо же в режиме изоляции хуков. Для этого необходимо создать ConfigMap `chroot-mode` в неймспейсе `d8-system`.

   Если включен режим [высокой доступности (High Availability, HA)](../../admin/configuration/high-reliability-and-availability/), запускается несколько экземпляров контроллера Deckhouse. Для обеспечения корректной работы контроллеры Deckhouse проводят выборы лидера с использованием ресурса Lease `deckhouse-leader-election`. Контроллер, который был избран как лидер, запускает [addon-operator](https://github.com/flant/addon-operator) и управление кастомными ресурсами.
  
1. **Webhook-handler** (Deployment) — состоит из одного контейнера **handler** и реализует универсальный веб-хук для конверсий и валидации кастомных ресурсов, находящихся под управлением DKP.

    Компонент следит за кастомными ресурсами [ConversionWebhook](/modules/deckhouse/latest/cr.html#conversionwebhook) и [ValidationWebhook](/modules/deckhouse/latest/cr.html#validationwebhook) и на их основе создаёт из шаблона Python-файлы хуков для [shell-operator](https://github.com/flant/shell-operator). При получении запросов от `kube-apiserver` на валидацию или конверсию ресурсов shell-operator запускает необходимый хук и возвращает результат обработки.

1. **Cni-migration-manager** (Deployment) — опциональный компонент, запускающийся на control-plane узлах и состоящий из одного контейнера **manager**. Компонент управляет процессом миграции CNI и фиксирует текущее состояние в кастомном ресурсе CNIMigration.

    {% alert level="info" %}
    Компонент создаётся global hook `detect-cni-migration` при наличии CNIMigration, создаваемый при [переключении CNI в кластере](/products/kubernetes-platform/guides/cni-migration.html).
    {% endalert %}

1. **Cni-migration-agent** (DaemonSet) — опциональный компонент, запускающийся на всех узлах кластера и состоящий из одного контейнера **agent**. Компонент следит за кастомным ресурсом CNIMigration и управляет кастомным ресурсом CNINodeMigration, отражающим текущее состояние миграции на конкретном узле.

    {% alert level="info" %}
    Компонент создаётся global hook `detect-cni-migration` при наличии CNIMigration, создаваемый при [переключении CNI в кластере](/products/kubernetes-platform/guides/cni-migration.html).
    {% endalert %}

## Взаимодействия модуля

Модуль взаимодействует со следующими компонентами:

1. **Kube-apiserver**:
   - работа с кастомными ресурсами API-группы `deckhouse.io`;
   - отслеживание Pod и DaemonSet;
   - отслеживание ресурсов, описанных в кастомном ресурсе ObjectKeeper;
   - создание и обновление ресурса Lease;
   - создание, удаление, изменение и отслеживание ресурсов, описанных в модулях DKP;
   - авторизация запросов.

1. [**Documentation**](/modules/documentation/) — обновление документации при добавлении или обновлении модуля DKP.

1. **Хранилище образов** — получение модулей вместе с метаданными.

С модулем взаимодействуют следующие внешние компоненты:

* **Kube-apiserver** — валидация и конверсия кастомных ресурсов DKP.
* **Prometheus-main** — сбор метрик c `deckhouse` и `webhook-handler` компонентов.

## Очередь Deckhouse

Контроллер Deckhouse реализует 2 вида очередей:

1. **Очередь addon-operator** — это основной механизм обработки встроенных и внешних модулей Deckhouse. Очередь реализована в [shell-operator](https://github.com/flant/shell-operator) и расширена типами задач [addon-operator](https://github.com/flant/addon-operator). Контроллер Deckhouse синхронизирует кастомные ресурсы ModuleConfig и обновляет глобальный values для addon-operator.

    Каждая очередь — отдельный pipeline с одним воркером, который реализует очередь со следующими свойствами:

    - задачи могут вставляться как в хвост (`AddLast`), так и в голову (`AddFirst`) очереди;
    - выполнение задач происходит с головы очереди;
    - поддерживается работа с многофазными операциями, которые используют операции:
       - `AddHeadTasks` — вставить подзадачи перед текущей;
       - `AddTailTasks` — вставить подзадачу в конец очереди после того, как текущая завершится успешно;
       - `AddAfterTasks` — вставить подзадачу сразу после текущей;
    - задача выполняется до успеха, если не указано `allowFailure: true` в параметре хука;
    - в случае ошибки выполняется экспоненциальный перезапуск (backoff) начиная с задержки в 5 секунд между попытками.

    {% alert level="warning" %}
    Если задача не может завершиться успешно и при этом в параметрах хука не указано `allowFailure: true`, то такая задача блокирует очередь, в которой она выполняется.
    Задачи в разных очередях обработки не блокируют друг друга.
    {% endalert %}

    Типы очередей:

    | Очередь       | Имя                                   | Назначение                                                                                      |
    |-------------- |-------------------------------------- |-------------------------------------------------------------------------------------------------|
    | Main          | `main`                                  | Старт, converge, ModuleConfig, ModuleRun/Delete, глобальные хуки на старте                      |
    | Parallel      | `parallel_queue_0` … `parallel_queue_19`  | Параллельный ModuleRun с учётом зависимостей модулей (20 штук)                                 |
    | Hook queues   | из конфига хука | Хуки конкретного модуля или global hook                                        |
    
    Типы задач:

    | Задача                           | Что делает                                                                          |
    |----------------------------------|-------------------------------------------------------------------------------------|
    | GlobalHookRun                    | Запуск global hook (onStartup, beforeAll, afterAll, kubernetes, schedule)           |
    | GlobalHookEnableKubernetesBindings | Включение K8s-мониторов global hook                                                 |
    | GlobalHookWaitKubernetesSynchronization | Ожидание sync global hooks                                                 |
    | GlobalHookEnableScheduleBindings | Включение cron global hook                                                          |
    | DiscoverHelmReleases             | Поиск «лишних» Helm-релизов после первого converge                                  |
    | ApplyKubeConfigValues            | Применение изменений из ModuleConfig                                                |
    | ConvergeModules                  | Полный цикл converge всех модулей                                                   |
    | ModuleRun                        | Настройка или обновление модуля, выполняются подзадачи: onStartup → sync → beforeHelm → helm → afterHelm                                   |
    | ParallelModuleRun                | Пакетный параллельный запуск модулей                                                |
    | ModuleDelete                     | Удаление модуля (helm delete, afterDeleteHelm)                                      |
    | ModuleHookRun                    | Запуск module hook по событию                                                       |
    | ModuleEnsureCRDs                 | Установка CRD модуля                                                                |
    | ModulePurge                      | Удаление неизвестного helm release                                                  |

    При старте контроллер Deckhouse создает очереди `main` и `parallel_queue_0..19` и добавляет в `main` задачи в следующем порядке:

     - GlobalHookRun (onStartup) — для каждого global hook;
     - GlobalHookEnableScheduleBindings — для включения планировщика cron;
     - GlobalHookEnableKubernetesBindings — для включения глобальных хуков отслеживающих ресурсы Kubernetes;
     - GlobalHookWaitKubernetesSynchronization;
     - ConvergeModules (OperatorStartup) — первый converge всех модулей.
    
    После ConvergeModules контроллер добавляет задачу DiscoverHelmReleases — очистка неизвестных helm releases.

    Порядок обработки модулей в задаче ConvergeModules определяется несколькими аттрибутами:

     - критичность модуля — заданный параметр `critical` в конфигурации модуля `module.yaml`;    
     - вес модуля — числовое обозначение порядка обработки модуля, чем число выше, тем позже будет обрабатываться модуль. Вес модуля берётся из параметра `weight` конфигурации модуля `module.yaml`, если этого параметра нет или он равен 0, то используется вес по-умолчанию, равный 900. Если файла конфигурации модуля не существует, то используется вес из числового префикса директории с модулем (например: `040-node-manager` — вес 40). Если вес не удалось получить из имени директории, используется вес, равный 100;
     - зависимости модуля — список модулей, которые должны быть установлены до текущего модуля.
    
    На основе этих аттрибутов, планировщик контроллера Deckhouse выстраивает порядок обработки по следующим принципам:

     - для критичных модулей учитывается вес модуля в порядке возрастания — задачи помещаются в очередь `main`;
     - для некритичных модулей вес модуля не учитывается — задачи помещаются в очереди `parallel_queue_0..19`;
     - для всех модулей учитывается зависимости от других модулей.

     Для просмотра очередей addon-operator можно использовать команду `d8 system queue list`. 

1. **Очередь Marketplace** — внутренняя реализация очереди для работы с подсистемой [Marketplace](../marketplace).

    Каждая очередь, которую обслуживает контроллер Deckhouse для подсистемы Marketplace, обладает следующими свойствами:

    - FIFO (First In First Out) — определяет неизменяемый порядок выполнения задач. Задача поступившая в очередь первая будет исполнена первой;
    - строго последовательное выполнение задач, одна задача за раз;
    - запуск задач происходит только при получении событий (event-driven), исключен опрос каких-либо ресурсов;
    - повторный запуск при ошибке с экспоненциальным ростом промежутка между попытками, начиная с 15 секунд, но не более 1 минуты, без лимита на количество попыток;
    - поддерживается каскадная отмена задач при смене версии или удалении пакета.

    Типы очередей:

    | Имя                               | Назначение                                                        |
    |-----------------------------------|-------------------------------------------------------------------|
    | {packageName}                     | Lifecycle: Deploy, Load, Configure, Enable, Run, Disable, Undeploy|
    | {packageName}/{hookQueue}         | Хуки по K8s/schedule-событиям (queue из binding хука)            |
    | {packageName}/{hookQueue}/sync    | Синхронизация хука при старте (WaitForSynchronization)            |

    Типы задач:

    | Задача    | Что делает                                                                                  |
    |-----------|--------------------------------------------------------------------------------------------|
    | Deploy    | Скачивает/монтирует образ пакета                                                           |
    | Load      | Парсит конфиг, создаёт Application/Module, регистрирует в scheduler                        |
    | Configure | Применяет settings из Store                                                                |
    | Enable    | Включает хуки, sync, OnStartup                                                             |
    | Run       | BeforeHelm → helm Upgrade → AfterHelm                                                      |
    | HookRun   | Запуск хука по событию                                                                     |
    | HookSync  | Начальная синхронизация K8s binding                                                        |
    | Disable   | Удаляет Helm, отключает хуки, чистит hook-очереди                                          |
    | Undeploy  | Убирает пакет с диска                                                                      |

    Выполнение задач в одной очереди не блокируют выполнение задач в другой очереди.

    Для просмотра очередей подсистемы Marketplace можно использовать команду `d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages queue dump`.

    {% alert level="warning" %}
    Контроллер Deckhouse при converge модуля в addon-operator ставит на паузу отработку очередей подсистемы Marketplace.
    {% endalert %}