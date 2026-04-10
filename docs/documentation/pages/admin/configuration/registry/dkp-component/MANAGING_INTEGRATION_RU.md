---
title: Настройки в кластере, управляемом DKP
permalink: ru/admin/configuration/registry/dkp-component/managing-interaction.html
description: "Как настроить взаимодействие с хранилищем образов компонентов платформы в кластере, полностью управляемом DKP."
lang: ru
search: registry, dkp registry, direct, proxy, local, unmanaged, module registry, хранилище образов, управляемый кластер
extractedLinksMax: 3
---
Управление настройками взаимодействия с хранилищем образов компонентов платформы в кластерах, полностью управляемых DKP, реализуется с помощью модуля [`registry`](/modules/registry/).

## Что настраивается на этой странице

Параметр `settings.registry` задает, как DKP подключается к хранилищу образов контейнеров с образами своих компонентов. В кластерах, полностью управляемых DKP, эти настройки обрабатывает внутренний модуль `registry`.

Режим работы задается в `ModuleConfig` `deckhouse` в секции `spec.settings.registry`. Именно через эту конфигурацию DKP переключается между вариантами доступа к образам компонентов платформы.

## Что такое режимы и зачем их переключать

Режимы определяют, как DKP получает образы своих компонентов:
- напрямую из внешнего хранилища образов контейнеров;
- через внутренний кеширующий прокси;
- из локального внутреннего хранилища образов.

Кластер уже работает в одном из режимов. Текущий режим зависит от того, как кластер был установлен и какая конфигурация `settings.registry` уже применена. Единого исходного режима для всех кластеров нет.

Переключение между режимами нужно, если меняются требования к работе с образами компонентов DKP. Например:
- нужен прямой доступ к внешнему хранилищу образов контейнеров без смены внутреннего адреса;
- нужен кеширующий прокси;
- нужен изолированный сценарий с локальным внутренним хранилищем образов;
- нужно использовать прямое подключение без внутреннего механизма `registry`.

{% alert level="info" %}
Режим `Local` не используется во время bootstrap. Bootstrap кластера поддерживается только в режимах `Direct`, `Unmanaged` и `Proxy`.
{% endalert %}

## Как посмотреть текущий режим

Посмотрите текущую конфигурацию `ModuleConfig` `deckhouse`:

```bash
d8 k get mc deckhouse -o yaml
```

Обратите внимание на секцию:

```yaml
spec:
  settings:
    registry:
```

Если параметр `mode` задан, он показывает текущий режим работы DKP с хранилищем образов компонентов платформы.

## Режимы взаимодействия с хранилищем образов компонентов DKP

В DKP поддерживается несколько режимов взаимодействия с хранилищем образов компонентов платформы.

В режимах `Direct`, `Proxy` и `Local` используется фиксированный виртуальный адрес `registry.d8-system.svc:5001/system/deckhouse`. Это позволяет избежать перезапуска всех компонентов control plane и повторного скачивания образов при изменении параметров хранилища образов.

В режиме `Unmanaged` виртуальный адрес не используется. DKP обращается напрямую к внешнему хранилищу образов контейнеров. Если адрес хранилища или его параметры меняются, все компоненты DKP перезапускаются.

Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически.

Архитектура режимов описана в разделе [«Архитектура режимов взаимодействия с хранилищем образов»](../../../architecture/deckhouse/registry-modes.html).

### Особенности режимов

- `Direct` — прямой доступ к внешнему хранилищу образов контейнеров по фиксированному виртуальному адресу `registry.d8-system.svc:5001/system/deckhouse`. Этот режим помогает избежать повторной загрузки образов и перезапуска компонентов при изменении параметров хранилища.
- `Proxy` — использование внутреннего кеширующего прокси, который обращается к внешнему хранилищу образов контейнеров. Кеширующий прокси запускается на control-plane-узлах и сокращает число запросов во внешнее хранилище.
- `Local` — использование локального внутреннего хранилища образов. Этот режим подходит для изолированных сред. Хранилище запускается на control-plane-узлах.
- `Unmanaged` — работа без внутреннего механизма хранилища образов. DKP обращается напрямую к внешнему хранилищу образов контейнеров.

Для режима `Unmanaged` доступны два варианта:
- **конфигурируемый** — режим, которым управляет модуль `registry`;
- **неконфигурируемый** — устаревший режим без использования модуля `registry`. Параметры задаются при установке кластера или меняются через `helper change-registry`.

{% alert level="info" %}
Для работы в режиме `Direct` необходимо использовать CRI `containerd` или `containerd v2` на всех узлах кластера. Для настройки CRI ознакомьтесь с конфигурацией [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration).
{% endalert %}

## Ограничения

### Ограничения при установке кластера

При установке кластера действуют такие ограничения:
- bootstrap кластера поддерживается только в режимах `Direct`, `Unmanaged` и `Proxy`;
- bootstrap в режиме `Local` не поддерживается;
- для запуска кластера в неконфигурируемом `Unmanaged` режиме параметры хранилища образов нужно указать в `InitConfiguration`.

### Ограничения по условиям работы

Для управления настройками взаимодействия с хранилищем образов должны выполняться такие условия:
- на узлах кластера используется CRI `containerd` или `containerd v2`;
- кластер полностью управляется DKP;
- в Managed Kubernetes-кластерах модуль `registry` не используется;
- режимы `Local` и `Proxy` поддерживаются только в статичных кластерах.

### Ограничения по переключению режимов

Учитывайте следующие ограничения:
- менять параметры хранилища и переключать режимы можно только после завершения bootstrap;
- при первом переключении нужно выполнить миграцию пользовательских конфигураций хранилища образов;
- переключение в неконфигурируемый `Unmanaged` доступно только из `Unmanaged`;
- переключение между `Local` и `Proxy` напрямую не поддерживается. Используйте промежуточный режим `Direct` или `Unmanaged`.

## Примеры переключения режимов

В этом разделе показано, как перевести уже работающий кластер на другой режим. Перед переключением:
- проверьте текущий режим;
- убедитесь, что понимаете, зачем меняете его;
- убедитесь, что кластер соответствует ограничениям для нужного режима.

{% alert level="warning" %}
Если в процессе переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert %}

### Переключение на режим `Direct`

{% alert level="danger" %}
При первом переключении с режима `Unmanaged` на режим `Direct` произойдёт полный перезапуск всех компонентов DKP.
{% endalert %}

1. Если кластер использует неконфигурируемый `Unmanaged`, сначала выполните [миграцию на управление через модуль `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает:

   ```bash
   d8 k get module registry -o wide
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`:

   ```bash
   d8 k get nodes
   ```

1. Проверьте, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените настройки режима `Direct` в `ModuleConfig` `deckhouse`:

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
           license: <LICENSE_KEY>
   ```

1. Проверьте статус переключения в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

### Переключение на режим `Proxy`

{% alert level="danger" %}
- При первом переключении с режима `Unmanaged` на режим `Proxy` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Local` в `Proxy` напрямую недоступно. Сначала переключитесь на `Direct` или `Unmanaged`.
{% endalert %}

1. Если кластер использует неконфигурируемый `Unmanaged`, сначала выполните [миграцию на управление через модуль `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает:

   ```bash
   d8 k get module registry -o wide
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`:

   ```bash
   d8 k get nodes
   ```

1. Проверьте, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените настройки режима `Proxy` в `ModuleConfig` `deckhouse`:

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
           license: <LICENSE_KEY>
   ```

1. Проверьте статус переключения в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

### Переключение на режим `Local`

{% alert level="danger" %}
- При первом переключении с режима `Unmanaged` на режим `Local` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Proxy` в `Local` напрямую недоступно. Сначала переключитесь на `Direct` или `Unmanaged`.
{% endalert %}

1. Если кластер использует неконфигурируемый `Unmanaged`, сначала выполните [миграцию на управление через модуль `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает:

   ```bash
   d8 k get module registry -o wide
   ```

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`:

   ```bash
   d8 k get nodes
   ```

1. Проверьте, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Подготовьте bundle с образами DKP текущей версии:

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

1. Включите режим `Local`:

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

1. Проверьте статус `RegistryContainsRequiredImages` в `registry-state`. До загрузки образов условие будет показывать, что образы ещё отсутствуют в локальном хранилище образов.

1. Получите данные пользователя `rw` для загрузки образов:

   ```bash
   d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   ```

1. Загрузите образы в локальное хранилище образов:

   ```bash
   d8 mirror push \
     --registry-login="rw" \
     --registry-password="<PASSWORD>" \
     /home/user/d8-bundle \
     registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Дождитесь состояния `Ready` в `registry-state`.

### Переключение на режим `Unmanaged`

{% alert level="danger" %}
Изменение хранилища образов в режиме `Unmanaged` приведёт к перезапуску всех компонентов DKP.
{% endalert %}

1. Если кластер использует неконфигурируемый `Unmanaged`, сначала выполните [миграцию на управление через модуль `registry`](#миграция-на-формат-управления-хранилищем-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает:

   ```bash
   d8 k get module registry -o wide
   ```

1. Проверьте, что очередь Deckhouse пуста:

   ```bash
   d8 system queue list
   ```

1. Примените настройки режима `Unmanaged` в `ModuleConfig` `deckhouse`:

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
           license: <LICENSE_KEY>
   ```

1. Проверьте статус переключения в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов).

1. Если нужно вернуться к устаревшему способу управления хранилищем образов, используйте [инструкцию](#миграция-на-устаревший-формат-управления-хранилищем-образов-без-модуля-registry).

{% alert level="warning" %}
Это устаревший формат управления хранилищем образов.
{% endalert %}

## Миграция на формат управления хранилищем образов с использованием модуля `registry`

Во время миграции для `containerd v1` выполняется переход на новую схему конфигурации хранилища образов.
`containerd v2` использует новую схему по умолчанию.

Подробнее о способах настройки можно прочитать в [FAQ модуля `node-manager`](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).

### Для `containerd v2`

1. Посмотрите текущие настройки хранилища образов:

   ```bash
   d8 k -n d8-system exec -it svc/deckhouse-leader -c deckhouse -- \
     deckhouse-controller global values | yq e '.modulesImages.registry' -
   ```

1. Укажите эти значения в `ModuleConfig` `deckhouse` и включите конфигурируемый `Unmanaged`:

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
           license: <LICENSE_KEY>
   ```

1. Дождитесь завершения переключения и проверьте `registry-state`.

### Для `containerd v1`

{% alert level="danger" %}
- Во время переключения сервис `containerd v1` будет перезапущен.
- `containerd v1` будет переведён на новую схему конфигурации хранилища образов.
- Во время переключения пользовательские конфигурации registry для `containerd v1` будут временно недоступны.
{% endalert %}

1. Убедитесь, что на узлах с `containerd v1` нет пользовательских конфигураций registry в директории `/etc/containerd/conf.d`.

1. Если такие конфигурации есть, перенесите их в новый формат в директорию `/etc/containerd/registry.d`.

1. Подготовьте `NodeGroupConfiguration` для добавления новых конфигураций. Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
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

1. Примените `NodeGroupConfiguration` и дождитесь появления файлов в `/etc/containerd/registry.d` на всех узлах.

1. Проверьте, что конфигурация работает:

   ```bash
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag
   ```

   Для `http`:

   ```bash
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Включите управление через модуль `registry`, указав параметры режима `Unmanaged` в `ModuleConfig` `deckhouse`.

1. Дождитесь в `registry-state` условия `ContainerdConfigPreflightReady`. Если статус `False`, это значит, что на узлах остались старые конфигурации в `/etc/containerd/conf.d`.

1. Удалите старые конфигурационные файлы. Для этого можно временно создать `NodeGroupConfiguration`, который удаляет старый файл. Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     weight: 0
     bundles:
       - '*'
     nodeGroups:
       - "*"
     content: |
       file="/etc/containerd/conf.d/old-config.toml"
       [ -f "$file" ] && rm -f "$file"
   ```

1. После удаления старых конфигураций дождитесь, пока `ContainerdConfigPreflightReady` станет `True`.

1. Дождитесь завершения переключения.

1. Удалите временный `NodeGroupConfiguration`, который использовали для удаления старой конфигурации:

   ```bash
   d8 k delete nodegroupconfiguration containerd-additional-config-auth-delete.sh
   ```

## Миграция на устаревший формат управления хранилищем образов без модуля `registry`

{% alert level="danger" %}
- Это устаревший формат управления хранилищем образов.
- Во время переключения `containerd v1` будет перезапущен.
- `containerd v1` будет переведён на старую схему конфигурации хранилища образов.
- Во время переключения пользовательские конфигурации registry для `containerd v1` будут временно недоступны.
{% endalert %}

1. Переведите registry в режим `Unmanaged` с конфигурируемыми параметрами:

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
           license: <LICENSE_KEY>
   ```

1. Проверьте статус переключения.

1. Переведите registry в неконфигурируемый `Unmanaged`:

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

1. Снова проверьте статус переключения.

1. Если используется `containerd v1` и в кластере есть пользовательские конфигурации registry, подготовьте их в старом формате.

1. Удалите секрет `registry-bashible-config`:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. Дождитесь завершения переключения.

1. Если используется `containerd v1`, примените подготовленные `NodeGroupConfiguration` со старым форматом конфигурации.

1. Отключите модуль `registry`:

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

Статус переключения можно получить так:

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

Вывод показывает состояние процесса переключения. Каждое условие может иметь статус `True` или `False` и поле `message` с пояснением.

### Что означают условия

| Условие | Описание |
| --- | --- |
| `ContainerdConfigPreflightReady` | Проверка конфигурации `containerd` перед переключением. |
| `TransitionContainerdConfigReady` | Подготовка конфигурации `containerd` к новому режиму. |
| `FinalContainerdConfigReady` | Завершение переключения `containerd` на новый режим. |
| `DeckhouseRegistrySwitchReady` | Переключение Deckhouse и его компонентов на новое хранилище образов контейнеров. |
| `InClusterProxyReady` | Готовность In-Cluster Proxy. |
| `CleanupInClusterProxy` | Удаление ресурсов In-Cluster Proxy, если они больше не нужны. |
| `NodeServicesReady` | Готовность Node Services Manager и static pod хранилища образов. |
| `CleanupNodeServices` | Удаление ресурсов Node Services Manager и static pod хранилища образов, если они больше не нужны. |
| `RegistryContainsRequiredImages` | Проверка, что хранилище образов содержит все необходимые образы. |
| `Ready` | Общее состояние готовности хранилища образов в выбранном режиме. |
