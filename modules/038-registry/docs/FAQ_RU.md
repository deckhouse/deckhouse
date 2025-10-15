---
title: "Модуль registry: FAQ"
description: "Часто задаваемые вопросы о модуле registry Deckhouse Kubernets Platform, включая процедуры миграции, переключение режимов, конфигурацию containerd и устранение проблем с registry."
---

## Как подготовить containerd v1?

{% alert level="info" %}

При переключении на режим `Direct` сервис containerd v1 будет перезапущен.  
Конфигурация авторизации будет изменена на Mirror Auth (данная конфигурация используется по умолчанию в `Containerd V2`).  
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

Перед переключением убедитесь, что на узлах с containerd v1 отсутствуют [пользовательские конфигурации реестра](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), расположенные в директории `/etc/containerd/conf.d`.

Если такие конфигурации существуют:

{% alert level="danger" %}
- После удаления [пользовательских конфигураций реестра](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) из директории `/etc/containerd/conf.d` сервис containerd будет перезапущен. Удалённые конфигурации перестанут работать.

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

## Как переключиться на предыдущую конфигурацию авторизации containerd v1?

{% alert level="warning" %}
Этот формат конфигурации containerd устарел (deprecated).
{% endalert %}

{% alert level="danger" %}
- Переключение возможно только в режиме `Unmanaged`.
- При переключении на старую конфигурацию авторизации `Containerd V1` пользовательские конфигурации в `/etc/containerd/registry.d` перестанут работать.
- Добавить [пользовательские конфигурации реестра](/products/kubernetes-platform/documentation/v1.72/modules/node-manager/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для старой схемы авторизации (в каталог `/etc/containerd/conf.d`) можно только после переключения на неё.
{% endalert %}

Чтобы переключиться на предыдущую конфигурацию авторизации containerd v1, выполните следующие шаги:

1. Перейдите в режим `Unmanaged`.

1. Проверьте статус переключения, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

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

1. После удаления дождитесь завершения переключения на старую конфигурацию авторизации в `Containerd V1`.  
   Для отслеживания используйте [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

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

## Как посмотреть статус переключения режима registry?

Статус переключения режима registry можно получить с помощью следующей команды:

<!-- TODO(nabokihms): заменить на подкоманду d8, когда она будет реализована -->
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
| `FinalContainerdConfigReady`      | Состояние завершения переключения containerd в новый режим. Проверяется, что конфигурация containerd успешно применена и содержит конфигурацию нового режима.                                 |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения Deckhouse и его компонентов на использование нового registry. Значение `True` указывает, что Deckhouse успешно переключился на сконфигурированный registry и готов к работе. |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает.                                                                                              |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены.                                |
| `Ready`                           | Общее состояние готовности registry к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе.                                                       |
