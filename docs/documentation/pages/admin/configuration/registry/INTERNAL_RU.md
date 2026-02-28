---
title: Работа с хранилищем образов контейнеров и редакциями в кластере, полностью управляемом DKP
permalink: ru/admin/configuration/registry/internal.html
description: "Настройка хранилища образов в Deckhouse Kubernetes Platform. Кеширование образов, оптимизация хранилища и управление высокодоступным хранилищем образов."
lang: ru
---

Возможность управления хранилищем образов контейнеров реализуется модулем [`registry`](/modules/registry/).

## Режимы работы с хранилищем образов

В DKP реализованы следующие режимы работы хранилищем образов:

- `Direct` — использование прямого доступа к внешнему хранилищу образов по фиксированному адресу `registry.d8-system.svc:5001/system/deckhouse`. Фиксированный адрес, при изменении параметров хранилища, позволяет избежать повторного скачивания образов и перезапуска компонентов при смене параметров хранилища. Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически (подробнее — в примерах переключения ниже). Архитектура режима описана в разделе [«Архитектура режима Direct»](../../../architecture/registry-modes.html#архитектура-режима-direct).
- `Proxy` — использование внутреннего кеширующего прокси-хранилища образов с обращением к внешнему хранилищу, с запуском кеширующего прокси-хранилища на control-plane (master) узлах. Режим позволяет сократить количество запросов к внешнему хранилищу за счёт кеширования образов. Кешируемые данные хранятся на control-plane (master) узлах. Обращение к внутреннему хранилищу образов выполняется по фиксированному адресу `registry.d8-system.svc:5001/system/deckhouse` аналогично режиму `Direct`. Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически (подробнее — в примерах переключения ниже). Архитектура режима описана в разделе [«Архитектура режима Proxy»](../../../architecture/registry-modes.html##архитектура-режима-proxy).
- `Local` — использование локального внутреннего хранилища образов, с запуском хранилища на control-plane (master) узлах. Режим позволяет кластеру работать в изолированной среде. Данные хранятся на control-plane (master) узлах. Обращение к внутреннему хранилищу образов выполняется по фиксированному адресу `registry.d8-system.svc:5001/system/deckhouse` аналогично `Direct` и `Proxy` режимам. Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически (подробнее — в примерах переключения ниже). Архитектура режима описана в разделе [«Архитектура режима Local»](../../../architecture/registry-modes.html#архитектура-режима-local).
- `Unmanaged` — работа без использования внутреннего хранилища образов. Обращение внутри кластера выполняется напрямую к внешнему хранилищу.
  Существует 2 вида режима `Unmanaged`:
  - Конфигурируемый — режим, управляемый с помощью модуля `registry`. Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически (подробнее — в примерах переключения ниже).
  - Неконфигурируемый (deprecated) — режим, используемый по умолчанию. Параметры конфигурации задаются [при установке кластера](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), или при [изменении в развёрнутом кластере](/products/kubernetes-platform/documentation/v1/admin/configuration/registry/third-party.html) с помощью утилиты `helper change registry` (deprecated).

{% alert level="info" %}
Для работы в режиме `Direct` необходимо использовать CRI containerd или containerd v2 на всех узлах кластера. Для настройки CRI ознакомьтесь с конфигурацией [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration)
{% endalert %}

## Ограничения по работе с хранилищем образов

Работа с хранилищем образов имеет ряд ограничений и особенностей, касающихся установки, условий работы и переключения режимов.

### Ограничения при установке кластера

Ограничения при установке кластера следующие:

- Bootstrap кластера DKP поддерживается только в `Direct` и `Unmanaged` режимах (`Local` и `Proxy` режимы не поддерживаются). Хранилище образов контейнеров во время установки кластера настраивается через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry).
- Для запуска кластера в неконфигурируемом `Unmanaged` режиме (Legacy), необходимо указать параметры хранилища образов в [`initConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo).

### Ограничения по условиям работы

Для использования хранилища образов контейнеров в DKP необходимо соблюдение следующих условий:

- Если на узлах кластера используется CRI containerd или containerd v2. Для настройки CRI ознакомьтесь с конфигурацией [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
- Кластер полностью управляется DKP. В Managed Kubernetes кластерах он работать не будет.
- Режимы `Local` и `Proxy` поддерживаются только на статичных кластерах.

### Ограничения по переключению режимов

Ограничения по переключению режимов следующие:

- Изменение параметров хранилища образов и переключение режимов доступны только после полного завершения этапа bootstrap.
- При первом переключении необходимо выполнить миграцию пользовательских конфигураций хранилища образов. Подробнее — в разделе [«Модуль registry: FAQ»](/modules/registry/faq.html).
- Переключение в неконфигурируемый режим `Unmanaged`  доступно только из `Unmanaged` режима. Подробнее — в разделе [«Модуль registry: FAQ»](/modules/registry/faq.html).
- Переключение между режимами `Local` и `Proxy` возможно только через промежуточные режимы `Direct` или `Unmanaged`. Пример последовательности переключения: `Local`/`Proxy` → `Direct` → `Proxy`/`Local`.

## Примеры переключения

{% alert level="warning" %}
Если в процессе переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, для устранения проблемы воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert %}

### Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` выполните следующие шаги:

{% alert level="danger" %}
При первом переключении с режима `Unmanaged` на режим `Direct` произойдёт полный перезапуск всех компонентов DKP.
{% endalert %}

1. Перед переключением выполните [миграцию на формат управления хранилищем образов с использованием-модуля `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включен и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`. Для этого используйте следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 system queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Установите настройки режима `Direct` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-direct). Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Direct
         direct:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

   Пример вывода:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Direct
   target_mode: Direct
   ```

### Переключение на режим `Proxy`

{% alert level="danger" %}

- При первом переключении с режима `Unmanaged` на режим `Proxy` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Local` в `Proxy` недоступно. Для переключения из режима `Local` необходимо переключить хранилище образов на другой доступный режим (например, `Direct`).
{% endalert %}

Для переключения уже работающего кластера на режим `Proxy` выполните следующие шаги:

1. Перед переключением выполните [миграцию на формат управления хранилищем образов с использованием-модуля `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`, используя следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 system queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Установите настройки режима `Proxy` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-proxy). Если используется хранилище образов контейнеров, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Proxy
         proxy:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

   Пример вывода:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Proxy
   target_mode: Proxy
   ```

### Переключение на режим `Local`

{% alert level="danger" %}

- При первом переключении с режима `Unmanaged` на режим `Local` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Proxy` в `Local` недоступно. Для переключения из режима `Proxy` необходимо переключить хранилище образов на другой доступный режим (например, `Direct`).
{% endalert %}

Для переключения уже работающего кластера на режим `Local` выполните следующие шаги:

1. Перед переключением выполните [миграцию на формат управления хранилищем образов с использованием-модуля `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включен и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`, используя следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 system queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Подготовьте архивы с образами DKP текущей версии. Для этого, воспользуйтесь командой `d8 mirror`.

   Пример:

   ```bash
   TAG=$(
    d8 k -n d8-system get deployment/deckhouse -o yaml \
    | yq -r '.spec.template.spec.containers[] | select(.name == "deckhouse").image | split(":")[-1]'
   ) && echo "TAG: $TAG"

   EDITION=$(
    d8 k -n d8-system exec -it svc/deckhouse-leader -- deckhouse-controller global values -o yaml \
    | yq .deckhouseEdition
   ) && echo "EDITION: $EDITION"
   ```

   ```bash
   d8 mirror pull \
   --license="<LICENSE_KEY>" \
   --source="registry.deckhouse.ru/deckhouse/$EDITION" \
   --deckhouse-tag="$TAG" \
   /home/user/d8-bundle
   ```

1. Установите настройки режима `Local` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-mode).

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Local
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов). В статусе необходимо дождаться запуска проверки `RegistryContainsRequiredImages`. Условие отобразит отсутствие или наличие образов в запущенном локальном хранилище образов.

   Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: 0 of 166 items processed, 166 items with errors:
       - source: module/control-plane-manager/control-plane-manager133
         image: 10.128.0.5:5001/system/deckhouse@sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:00202db19b40930f764edab5695f450cf709d50736e012055393447b3379414a: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/cloud-provider-yandex/cloud-metrics-exporter
         image: 10.128.0.5:5001/system/deckhouse@sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0e28263da59edf11f0fad
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:05517a86fcf0ec4a62d14ed7dc4f9ffd91c05716b8b0ed86d6a1f465f4556fb8: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)
       - source: module/control-plane-manager/kube-controller-manager132
         image: 10.128.0.5:5001/system/deckhouse@sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61
         error: HEAD https://10.128.0.5:5001/v2/system/deckhouse/manifests/sha256:13f24cc717698682267ed2b428e7399b145a4d8ffe96ad1b7a0b3269b17c7e61: unexpected status code 404 Not Found (HEAD responses have no body, use GET for details)

         ...and more
     reason: Processing
     status: "False"
     type: RegistryContainsRequiredImages
   ```

1. Загрузите образы в локальное хранилище образов с помощью команды `d8 mirror`. Образы загружаются в локальное хранилище через Ingress по адресу `registry.${PUBLIC_DOMAIN}`.

   Получите пароль read-write пользователя локального хранилища образов:

   ```bash
   $ d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   name: rw
   password: KFVxXZGuqKkkumPz
   passwordHash: $2a$10$Phjbr6iinLf00ZZDD2Y7O.p9H3nDOgYzFmpYKW5eydGvIsdaHQY0a
   ```

   Загрузите образы в локальное хранилище образов:

   ```bash
   d8 mirror push \
   --registry-login="rw" \
   --registry-password="KFVxXZGuqKkkumPz" \
   /home/user/d8-bundle \
   registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов). После загрузки образов статус `RegistryContainsRequiredImages` должен быть в состоянии `Ready`

   Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: |-
       Mode: Default
       master-1: all 166 items are checked
     reason: Ready
     status: "True"
     type: RegistryContainsRequiredImages
   hash: ..
   mode: Direct
   target_mode: Local
   ```

1. Дождитесь завершения переключения. Для проверки статуса переключения воспользуйтесь [инструкцией](#просмотр-статуса-переключения-режима-хранилища-образов).

   Пример вывода:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Local
   target_mode: Local
   ```

### Переключение на режим Unmanaged

Для переключения уже работающего кластера на режим `Unmanaged` выполните следующие шаги:

{% alert level="danger" %}
Изменение хранилища образов в `Unmanaged` режиме приведёт к перезапуску всех компонентов DKP.
{% endalert %}

1. Перед переключением выполните [миграцию на формат управления хранилищем образов с использованием-модуля `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включен и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Проверьте, чтобы очередь Deckhouse была пустой и без ошибок:

   ```shell
   d8 system queue list
   ```

   Пример вывода:

   ```console
   Summary:
   - 'main' queue: empty.
   - 107 other queues (0 active, 107 empty): 0 tasks.
   - no tasks to handle.
   ```

1. Установите настройки режима `Unmanaged` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

   Пример вывода:

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. При необходимости переключения на старый метод управления хранилищем образов, ознакомьтесь с [инструкцией](#миграция-на-устаревший-формат-управления-хранилищем-образов-без-модуля-registry).

{% alert level="warning" %}
Это устаревший (deprecated) формат управления хранилищем образов.
{% endalert %}

## Миграция на формат управления хранилищем образов с использованием модуля registry

Во время миграции для containerd v1 будет выполнен переход на новую схему конфигурации хранилища образов.
containerd v2 использует новую схему по умолчанию. Подробнее можно ознакомиться в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry)

### Для containerd v2

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в `moduleConfig` `deckhouse` параметры `Unmanaged` режима. Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Посмотреть текущие настройки хранилища образов можно с помощью команды:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Данные настройки укажите при конфигурации `Unmanaged` режима:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#просмотр-статуса-переключения-режима-хранилища-образов):

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

### Для containerd v1

{% alert level="danger" %}

- Во время переключения containerd v1 сервис будет перезапущен.
- Во время переключения containerd v1 будет переведен на новую схему конфигурации хранилища образов.
- Во время переключения, [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Убедитесь, что на узлах с containerd v1 отсутствуют [пользовательские конфигурации хранилища образов](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), расположенные в директории `/etc/containerd/conf.d`.

1. Если конфигурации присутствуют, необходимо выполнить миграцию на новый формат конфигурации хранилища образов в containerd. Для этого добавьте новые конфигурации в директорию `/etc/containerd/registry.d`. Эти конфигурации вступят в силу после переключения на модуль `registry`. Для добавления конфигураций подготовьте `NodeGroupConfiguration`, подробнее — в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry). Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Шаг может быть любой, т.к. не требуется перезапуск сервиса containerd
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.
       
       REGISTRY_URL=private.registry.example

       mkdir -p "/etc/containerd/registry.d/${REGISTRY_URL}"
       bb-sync-file "/etc/containerd/registry.d/${REGISTRY_URL}/hosts.toml" - << EOF
       [host]
         [host."https://${REGISTRY_URL}"]
           capabilities = ["pull", "resolve"]
           [host."https://${REGISTRY_URL}".auth]
             username = "username"
             password = "password"
       EOF
   ```

1. Примените [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Дождитесь появления конфигурационных файлов в директории `/etc/containerd/registry.d` на всех узлах.

1. Проверьте корректность работы конфигураций. Для этого воспользуйтесь командой:

   ```bash
   # Для https:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # Для http:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в `moduleConfig` `deckhouse` параметры `Unmanaged` режима. Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Посмотреть текущие настройки хранилища образов можно с помощью команды:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

   Данные настройки укажите при конфигурации `Unmanaged` режима:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. После применения, дождитесь в [статусе переключения](#просмотр-статуса-переключения-режима-registry) сообщения:

   Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T15:22:34Z"
     message: |
       Check current nodes configuration
       2/2 node(s) Unready:
       - master-0: has custom toml merge containerd configuration
       - worker-5e389be0-578df-s5sm5: has custom toml merge containerd configuration
     reason: Processing
     status: "False"
     type: ContainerdConfigPreflightReady
   ```

   Это сообщение означает, что на узлах имеются старые конфигурации хранилища образов, расположенные в директории `/etc/containerd/conf.d`. И в данный момент переключение на новую конфигурацию containerd заблокировано. Для того чтобы разрешить переключение, необходимо удалить старые конфигурационные файлы.

1. Удалите старые конфигурационные файлы, чтобы разрешить переключение на модуль `registry`. Для этого создайте [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Пример манифеста NodeGroupConfiguration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     # Шаг должен выполниться до '032_configure_containerd.sh'
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       file="/etc/containerd/conf.d/old-config.toml"

       [ -f "$file" ] && rm -f "$file"
   ```
  
1. После удаления старых конфигураций убедитесь, что переключение продолжается. Пример [статуса переключения](#просмотр-статуса-переключения-режима-registry):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#просмотр-статуса-переключения-режима-registry):

   ```yaml
   conditions:
   # ...
     - lastTransitionTime: "..."
       message: ""
       reason: ""
       status: "True"
       type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Удалите [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration), созданный на шаге удаления старых конфигурационных файлов:

   ```shell
   d8 k delete nodegroupconfiguration containerd-additional-config-auth-delete.sh
   ```

   Чтобы убедиться, что NodeGroupConfiguration удалён, используйте команду:

   ```shell
   d8 k get nodegroupconfiguration
   ```

   В списке не должно быть NodeGroupConfiguration, подлежащего удалению (в этом примере — `containerd-additional-config-auth-delete.sh`).

## Миграция на устаревший формат управления хранилищем образов (без модуля registry)

{% alert level="danger" %}

- Это устаревший (deprecated) формат управления хранилищем образов.
- Во время переключения containerd v1 будет перезапущен.
- Во время переключения containerd v1 будет переведен на старую схему конфигурации хранилища образов.
- Во время переключения, [пользовательские конфигурации хранилища образов](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Переведите registry в режим `Unmanaged`. Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [deckhouse](/modules/deckhouse/configuration.html) для корректной настройки.

   Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
         unmanaged:
           imagesRepo: registry.deckhouse.ru/deckhouse/ee
           scheme: HTTPS
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ
   ```

1. Проверьте статус переключения, используя [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Переведите registry в неконфигурируемый режим `Unmanaged`. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     enabled: true
     settings:
       registry:
         mode: Unmanaged
   ```

1. Проверьте статус переключения, используя [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Если используется containerd v1, и в кластере применены [пользовательские конфигурации реестра](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), их необходимо заменить на старый формат. Для этого подготовьте конфигурации хранилища образов старого формата. Данные конфигурации на данном этапе применять не нужно. Пример конфигурации:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Для добавления файла перед шагом '032_configure_containerd.sh'
     weight: 31
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       # Copyright 2023 Flant JSC
       #
       # Licensed under the Apache License, Version 2.0 (the "License");
       # you may not use this file except in compliance with the License.
       # You may obtain a copy of the License at
       #
       #     http://www.apache.org/licenses/LICENSE-2.0
       #
       # Unless required by applicable law or agreed to in writing, software
       # distributed under the License is distributed on an "AS IS" BASIS,
       # WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
       # See the License for the specific language governing permissions and
       # limitations under the License.

       REGISTRY_URL=private.registry.example

       mkdir -p /etc/containerd/conf.d
       bb-sync-file /etc/containerd/conf.d/additional_registry.toml - << EOF
       [plugins]
         [plugins."io.containerd.grpc.v1.cri"]
           [plugins."io.containerd.grpc.v1.cri".registry]
             [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
               [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_URL}"]
                 endpoint = ["https://${REGISTRY_URL}"]
             [plugins."io.containerd.grpc.v1.cri".registry.configs]
               [plugins."io.containerd.grpc.v1.cri".registry.configs."${REGISTRY_URL}".auth]
                 username = "username"
                 password = "password"
                 # OR
                 auth = "dXNlcm5hbWU6cGFzc3dvcmQ="
       EOF
   ```

1. Удалите секрет `registry-bashible-config`. Во время удаления containerd v1 переключится на старый формат конфигурации containerd:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. После удаления дождитесь завершения переключения. Для отслеживания используйте [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "..."
     message: ""
     reason: ""
     status: "True"
     type: Ready
   hash: ..
   mode: Unmanaged
   target_mode: Unmanaged
   ```

1. Если используется containerd v1, примените заготовленные на этапе ранее `NodeGroupConfiguration` с пользовательскими конфигурациями хранилища образов.

1. Отключите модуль `registry`. Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: false
     settings: {}
     version: 1
   ```

## Просмотр статуса переключения режима хранилища образов

Статус переключения режима хранилища образов можно получить с помощью следующей команды:

```bash
d8 k -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
```

Пример вывода:

```yaml
conditions:
  - lastTransitionTime: "2025-07-15T12:52:46Z"
    message: 'registry.deckhouse.ru: all 157 items are checked'
    reason: Ready
    status: "True"
    type: RegistryContainsRequiredImages
  - lastTransitionTime: "2025-07-11T11:59:03Z"
    message: ""
    reason: ""
    status: "True"
    type: ContainerdConfigPreflightReady
  - lastTransitionTime: "2025-07-15T12:47:47Z"
    message: ""
    reason: ""
    status: "True"
    type: TransitionContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:52:48Z"
    message: ""
    reason: ""
    status: "True"
    type: InClusterProxyReady
  - lastTransitionTime: "2025-07-15T12:54:53Z"
    message: ""
    reason: ""
    status: "True"
    type: DeckhouseRegistrySwitchReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: FinalContainerdConfigReady
  - lastTransitionTime: "2025-07-15T12:55:48Z"
    message: ""
    reason: ""
    status: "True"
    type: Ready
mode: Direct
target_mode: Direct
```

Вывод отображает состояние процесса переключения. Каждое условие может находиться в статусе `True` или `False`, а также содержать поле `message` с пояснением.

Описание условий:

| Условие                           | Описание                                                                                                                                                                                                                     |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `ContainerdConfigPreflightReady`  | Состояние проверки конфигурации containerd. Проверяется, что на узлах отсутствуют пользовательские auth конфигурации containerd.                                                                                             |
| `TransitionContainerdConfigReady` | Состояние подготовки конфигурации containerd в новый режим. Проверяется, что конфигурация containerd успешно подготовлена и содержит одновременно конфигурации нового и старого режима.                                      |
| `FinalContainerdConfigReady`      | Состояние завершения переключения containerd в новый режим. Проверяется, что конфигурация containerd успешно применена и содержит конфигурацию нового режима.                                                                |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения Deckhouse и его компонентов на использование нового хранилища образов контейнеров. Значение `True` указывает, что Deckhouse успешно переключился на сконфигурированное хранилище образов и готов к работе.                          |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает.                                                                                                                         |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены.                                                           |
| `NodeServicesReady`               | Состояние готовности Node Services Manager и Static-Pod хранилища образов. Проверяется, что Node Services Manager успешно запущен и работает, и что Static-Pod хранилища образов был успешно развёрнут с помощью Node Services Manager.        |
| `CleanupNodeServices`             | Состояние очистки Node Services Manager и Static-Pod хранилища образов, если компоненты не нужны для работы желаемого режима. Проверяется, что все ресурсы, связанные с Node Services Manager и Static-Pod хранилища образов, успешно удалены. |
| `RegistryContainsRequiredImages`  | Состояние проверки хранилища образов на наличие необходимых образов.                                                                                                                                                                   |
| `Ready`                           | Общее состояние готовности хранилища образов к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе.                                                                                  |
