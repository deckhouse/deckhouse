---
title: Очереди deckhouse
permalink: ru/architecture/deckhouse/queues.html
lang: ru
search: deckhouse, deckhouse-controller, modules, queue
description: Описание работы очередей контроллера Deckhouse в Deckhouse Kubernetes Platform.
---

Модуль [`deckhouse`](/modules/deckhouse/) реализует ядро Deckhouse Kubernetes Platform (DKP), выполняющее различные операции по управлению платформой с использованием механизма очередей. Подробнее с архитектурой модуля можно ознакомиться в [соответствующем разделе документации](./deckhouse.html).

Контроллер Deckhouse реализует 2 вида очередей:

- addon-operator;
- marketplace.

### Addon-operator

**Очередь addon-operator** — это основной механизм обработки встроенных и внешних модулей Deckhouse. Очередь реализована в [shell-operator](https://github.com/flant/shell-operator) и расширена типами задач [addon-operator](https://github.com/flant/addon-operator). Контроллер Deckhouse синхронизирует кастомные ресурсы [ModuleConfig](../../reference/api/cr.html#moduleconfig) и обновляет глобальные или модульные values для addon-operator.

Каждая очередь — отдельный pipeline с одним воркером, который реализует очередь со следующими свойствами:

- задачи могут вставляться как в хвост (`AddLast`), так и в голову (`AddFirst`) очереди;
- выполнение задач происходит с головы очереди;
- поддерживается работа с многофазными операциями, которые используют операции:
  - `AddHeadTasks` — вставить подзадачи перед текущей;
  - `AddTailTasks` — вставить подзадачу в конец очереди после того, как текущая завершится успешно;
  - `AddAfterTasks` — вставить подзадачу сразу после текущей;
- задача выполняется до успеха, если не указано `allowFailure: true` в параметре задачи;
- в случае ошибки выполняется экспоненциальный перезапуск (backoff) начиная с задержки в 5 секунд между попытками.

{% alert level="warning" %}
Если задача не может завершиться успешно и при этом в её параметрах не указано `allowFailure: true`, то такая задача блокирует очередь, в которой она выполняется.
Задачи в разных очередях обработки не блокируют друг друга.
{% endalert %}

Типы очередей:

| Очередь       | Имя                                   | Назначение                                                                                      |
|-------------- |-------------------------------------- |-------------------------------------------------------------------------------------------------|
| Main          | `main`                                  | Старт, converge, ModuleConfig, ModuleRun/Delete, глобальные задачи на старте                      |
| Parallel      | `parallel_queue_0` … `parallel_queue_19`  | Параллельный ModuleRun с учётом зависимостей модулей (20 штук)                                 |
| Hook queues   | из конфига задачи ([хука](https://github.com/flant/addon-operator/blob/main/docs/src/HOOKS.md)) | Задачи конкретного модуля или global hook                                        |

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
- GlobalHookEnableKubernetesBindings — для включения глобальных задач, отслеживающих ресурсы Kubernetes;
- GlobalHookWaitKubernetesSynchronization;
- ConvergeModules (OperatorStartup) — первый converge всех модулей.

После ConvergeModules контроллер добавляет задачу DiscoverHelmReleases — очистка неизвестных helm releases.

Порядок обработки модулей в задаче ConvergeModules определяется несколькими атрибутами:

- критичность модуля — заданный параметр `critical` в конфигурации модуля `module.yaml`;
- вес модуля — числовое обозначение порядка обработки модуля, чем число выше, тем позже будет обрабатываться модуль. Вес модуля берётся из параметра `weight` конфигурации модуля `module.yaml`, если этого параметра нет или он равен 0, то используется вес по-умолчанию, равный 900. Если файла конфигурации модуля не существует, то используется вес из числового префикса директории с модулем (например: `040-node-manager` — вес 40). Если вес не удалось получить из имени директории, используется вес, равный 100;
- зависимости модуля — список модулей, которые должны быть установлены до текущего модуля.

На основе этих атрибутов планировщик контроллера Deckhouse выстраивает порядок обработки по следующим принципам:

- для критичных модулей учитывается вес модуля в порядке возрастания — задачи помещаются в очередь `main`;
- для некритичных модулей вес модуля не учитывается — задачи помещаются в очереди `parallel_queue_0..19`;
- для всех модулей учитываются зависимости от других модулей.

При этом, если критичные модули могут быть обработаны параллельно, то планировщик контроллера Deckhouse помещает в очередь `main` задачу `ParallelModuleRun` с указанием списка модулей. Задача `ParallelModuleRun` запускает для каждого модуля задачу `ModuleRun` в очередях `parallel_queue_0..19` и ждёт завершения их работы, тем самым блокируя очередь `main`. Если при обработке задачи `ModuleRun` происходит ошибка, то такую задачу планировщик перемещает в конец очереди и запускает следующую задачу из очереди.

Процесс установки критичных модулей изображен на следующей диаграмме:

![Диаграмма последовательности при установке критичных модулей](../../images/architecture/deckhouse/DECKHOUSE_QUEUE_MODULES_CRITICAL.ru.svg)

Процесс установки некритичных (функциональных) модулей изображен на следующей диаграмме:

![Диаграмма последовательности при установке функциональный модулей](../../images/architecture/deckhouse/DECKHOUSE_QUEUE_MODULES_FUNCTIONAL.ru.svg)

Если в очередь добавлено более одной идентичной задачи, то при старте выполнения такой задачи все остальные задачи удаляются из очереди для дедупликации.

Для просмотра очередей addon-operator можно использовать команду `d8 system queue list`.

### Marketplace

**Очередь Marketplace** — внутренняя реализация очереди для работы с функционала [Marketplace](../marketplace).

Каждая очередь, которую обслуживает контроллер Deckhouse для Marketplace, обладает следующими свойствами:

- FIFO (First In First Out) — определяет неизменяемый порядок выполнения задач. Задача, поступившая в очередь первая, будет исполнена первой;
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

Выполнение задач в одной очереди не блокирует выполнение задач в другой очереди.

Для просмотра очередей  Marketplace можно использовать команду `d8 k -n d8-system exec -ti svc/deckhouse-leader -c deckhouse -- deckhouse-controller packages queue dump`.

{% alert level="warning" %}
Контроллер Deckhouse при converge модуля в addon-operator ставит на паузу отработку очередей Marketplace.
{% endalert %}
