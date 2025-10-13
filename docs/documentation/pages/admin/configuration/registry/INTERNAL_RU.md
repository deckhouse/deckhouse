---
title: Управление внутренним container registry
permalink: ru/admin/configuration/registry/internal.html
description: "Настройка внутреннего container registry в Deckhouse Kubernetes Platform. Кэширование образов, оптимизация хранилища и управление высокодоступным registry."
lang: ru
---

Возможность использования внутреннего container registry (registry) реализуется модулем [`registry`](/modules/registry/).

Внутренний registry позволяет оптимизировать загрузку и хранение образов, а также обеспечить высокую доступность и отказоустойчивость Deckhouse Kubernetes Platform.

## Режимы работы с внутренним registry

[Модуль `registry`](/modules/registry/), реализующий внутреннее хранилище, работает в следующих режимах:

- `Direct` — работа с использованием внутреннего registry. Обращение к внутреннему registry выполняется по фиксированному адресу `registry.d8-system.svc:5001/system/deckhouse`. Фиксированный адрес, при изменении параметров registry, позволяет избежать повторного скачивания образов и перезапуска компонентов. Переключение между режимами и registry выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html). Переключение выполняется автоматически (подробнее — в примерах переключения ниже). Архитектура режима описана в разделе [«Архитектура режима Direct»](../../../architecture/registry-direct-mode.html).
- `Unmanaged` — работа без использования внутреннего registry. Обращение внутри кластера выполняется по адресу, который можно [задать при установке кластера](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo), или [изменить в развернутом кластере](../registry/third-party.html).

{% alert level="info" %}
Для работы в режиме `Direct` необходимо использовать CRI containerd или containerd v2 на всех узлах кластера. Для настройки CRI ознакомьтесь с конфигурацией [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration)
{% endalert %}

## Ограничения по работе с внутренним registry

Работа с внутренним registry с помощью [модуля `registry`](/modules/registry/) имеет ряд ограничений и особенностей, касающихся установки, условий работы и переключения режимов.

### Ограничения при установке кластера

Bootstrap кластера Deckhouse Kubernetes Platform с включенным режимом `Direct` не поддерживается. Кластер разворачивается с настройками для режима `Unmanaged`.

### Ограничения по условиям работы

[Модуль `registry`](/modules/registry/), реализующий возможность использования внутреннего container registry, работает при соблюдении следующих условий:

- Если на узлах кластера используется CRI containerd или containerd v2. Для настройки CRI ознакомьтесь с конфигурацией [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
- Кластер полностью управляется DKP. В Managed Kubernetes кластерах он работать не будет.

### Ограничения по переключению режимов

Ограничения по переключению режимов следующие:

- Переключение на режим `Direct` возможно, если на узлах отсутствуют [пользовательские конфигурации registry](/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).
- Переключение в режим `Unmanaged` доступно только из режима `Direct`.
- В режиме `Unmanaged` изменение параметров registry не поддерживается. Для изменения параметров нужно переключиться на режим `Direct`, внести необходимые изменения и снова включить режим `Unmanaged`.

## Примеры переключения

### Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` (включает использование внутреннего registry) выполните следующие шаги:

{% alert level="danger" %}

- Во время первого переключения сервис `Containerd V1` будет перезапущен, так как выполнится переключение на [новую конфигурацию авторизации](#подготовка-containerd-v1).
- При изменении режима или параметров registry Deckhouse Kubernetes Platform будет перезапущена.

{% endalert %}

1. Если кластер запущен с containerd v1, [подготовьте пользовательские конфигурации containerd](#подготовка-containerd-v1).

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

1. Убедитесь, что [модуль `registry`](/modules/registry/) включен и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Установите настройки режима `Direct` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html). Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

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

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-registry).

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

### Переключение на режим Unmanaged

{% alert level="danger" %}
При изменении режима или параметров registry Deckhouse Kubernetes Platform будет перезапущена.
{% endalert %}

{% alert level="warning" %}
Переключение в режим `Unmanaged` доступно только из режима `Direct`. Конфигурационные параметры registry будут взяты из предыдущего активного режима.
{% endalert %}

Для переключения кластера на режим `Unmanaged` выполните следующие шаги:

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

1. Убедитесь, что [модуль `registry`](/modules/registry/) запущен в режиме `Direct`, и статус переключения в режим `Direct` имеет значение `Ready`. Проверить состояние можно через секрет `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

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

1. Установите настройки режима `Unmanaged` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html):

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

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

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

1. При необходимости переключения на предыдущую auth-конфигурацию containerd v1 ознакомьтесь с [инструкцией](#переключение-на-предыдущую-конфигурацию-авторизации-containerd-v1)

## Подготовка containerd v1

{% alert level="info" %}

При переключении на режим `Direct` сервис containerd v1 будет перезапущен.  
Конфигурация авторизации будет изменена на Mirror Auth (данная конфигурация используется по умолчанию в containerd v2).  
После возврата в режим `Unmanaged` обновлённая конфигурация авторизации останется без изменений.

{% endalert %}

Пример структуры Mirror Auth-конфигурации:

```bash
tree /etc/containerd/registry.d
.
├── registry.d8-system.svc:5001
│   ├── ca.crt
│   └── hosts.toml
└── registry.deckhouse.ru
    ├── ca.crt
    └── hosts.toml
```

Пример конфигурации файла `hosts.toml`:

```toml
[host]
  [host."https://registry.deckhouse.ru"]
    capabilities = ["pull", "resolve"]
    skip_verify = true
    ca = ["/path/to/ca.crt"]
    [host."https://registry.deckhouse.ru".auth]
      username = "username"
      password = "password"
      # If providing auth string:
      auth = "<base64>"
```

Перед переключением убедитесь, что на узлах с containerd v1 отсутствуют [пользовательские конфигурации авторизации](/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), расположенные в директории `/etc/containerd/conf.d`.

Если такие конфигурации существуют:

{% alert level="danger" %}
- После удаления [пользовательских конфигураций авторизации](/modules/node-manager/faq.html#как-добавить-авторизацию-в-дополнительный-registry) из директории `/etc/containerd/conf.d` сервис containerd будет перезапущен. Удалённые конфигурации перестанут работать.

- Новые Mirror Auth-конфигурации, добавленные в `/etc/containerd/registry.d`, вступят в силу только после перехода в режим `Direct`.
{% endalert %}

1. Создайте новые Mirror Auth-конфигурации в директории `/etc/containerd/registry.d`. Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: custom-registry
   spec:
     bundles:
       - '*'
     content: |
       #!/bin/bash
       REGISTRY_ADDRESS="registry.io"
       REGISTRY_SCHEME="https"
       host_toml=$(cat <<EOF
       [host]
         [host."https://registry.deckhouse.ru"]
           capabilities = ["pull", "resolve"]
           skip_verify = true
           ca = ["/path/to/ca.crt"]
           [host."https://registry.deckhouse.ru".auth]
             username = "username"
             password = "password"
             # If providing auth string:
             auth = "<base64>"
       EOF
       )
       mkdir -p "/etc/containerd/registry.d/${REGISTRY_ADDRESS}"
       echo "$host_toml" > "/etc/containerd/registry.d/${REGISTRY_ADDRESS}/hosts.toml"
     nodeGroups:
       - '*'
     weight: 0
   ```

   Для проверки новой конфигурации выполните:

   ```bash
   # HTTPS:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ registry.io/registry/path:tag

   # HTTP:
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http registry.io/registry/path:tag
   ```

1. Удалите auth-конфигурации из директории `/etc/containerd/conf.d`.

## Переключение на предыдущую конфигурацию авторизации containerd v1

{% alert level="danger" %}
- Переключение возможно только в режиме `Unmanaged`.
- При переключении на старую конфигурацию авторизации containerd v1 пользовательские конфигурации в `/etc/containerd/registry.d` перестанут работать.
- Добавить [пользовательские auth-конфигурации](/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для старой схемы авторизации (в каталог `/etc/containerd/conf.d`) можно только после переключения на неё.
{% endalert %}

Чтобы переключиться на предыдущую конфигурацию авторизации containerd v1, выполните следующие шаги:

1. Перейдите в режим `Unmanaged`.

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

1. Удалите секрет `registry-bashible-config`:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. После удаления дождитесь завершения переключения на старую конфигурацию авторизации в containerd v1.  
   Для отслеживания используйте [инструкцию](#просмотр-статуса-переключения-режима-registry). Пример вывода:

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

## Просмотр статуса переключения режима registry

Статус переключения режима registry можно получить с помощью следующей команды:

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

| Условие                           | Описание                                                                                                                                                                                          |
| --------------------------------- |---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `ContainerdConfigPreflightReady`  | Состояние проверки конфигурации containerd. Проверяется, что на узлах отсутствуют пользовательские auth конфигурации containerd.                                                              |
| `TransitionContainerdConfigReady` | Состояние подготовки конфигурации containerd в новый режим. Проверяется, что конфигурация containerd успешно подготовлена и содержит одновременно конфигурации нового и старого режима.       |
| `FinalContainerdConfigReady`      | Состояние завершения переключения containerd в новый режим. Проверяется, что конфигурация `containerd` успешно применена и содержит конфигурацию нового режима.                                 |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения Deckhouse и его компонентов на использование нового registry. Значение `True` указывает, что Deckhouse успешно переключился на сконфигурированный registry и готов к работе. |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает.                                                                                              |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены.                                |
| `Ready`                           | Общее состояние готовности registry к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе.                                                       |
