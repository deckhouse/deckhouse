---
title: "Модуль registry: FAQ"
description: ""
---

## Как подготовить Containerd V1?

{% alert level="danger" %}
При удалении [пользовательских конфигураций Auth](/products/kubernetes-platform/documentation/v1/modules/node-manager/faq.html#как-добавить-авторизацию-в-дополнительный-registry) сервис containerd будет перезапущен.

Новые Mirror Auth конфигурации, добавленные в `/etc/containerd/registry.d`, начнут применяться только после переключения на любой из `Managed` режимов registry (`Direct`, `Local`, `Proxy`).
{% endalert %}

Во время переключения на любой из `Managed` режимов (`Direct`, `Local`, `Proxy`) сервис `Containerd V1` будет перезапущен.

Конфигурация авторизации `Containerd V1` будет изменена на Mirror Auth (`Containerd V2` использует данную конфигурацию по умолчанию).

Перед переключением необходимо убедиться, что на узлах с `Containerd V1` отсутствуют [пользовательские конфигурации авторизации](/products/kubernetes-platform/documentation/v1/modules/node-manager/faq.html#как-добавить-авторизацию-в-дополнительный-registry), расположенные в директории `/etc/containerd/conf.d`.

Если конфигурации присутствуют, их необходимо удалить и создать новые конфигурации авторизации в директории `/etc/containerd/registry.d`. Пример:

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
      [host."${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}"]
        capabilities = ["pull", "resolve"]
        skip_verify = true
        ca = ["/path/to/ca.crt"]
        [host."${REGISTRY_SCHEME}://${REGISTRY_ADDRESS}".auth]
          username = "username"
          password = "password"
          # If auth string:
          auth = "<base64>"
    EOF
    )
    mkdir -p "/etc/containerd/registry.d/${REGISTRY_ADDRESS}"
    echo "$host_toml" > "/etc/containerd/registry.d/${REGISTRY_ADDRESS}/hosts.toml"
  nodeGroups:
    - '*'
  weight: 0
```

Для проверки работоспособности новой конфигурации воспользуйтесь командой:

```bash
# для https:
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ registry.io/registry/path:tag
# для http:
ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http registry.io/registry/path:tag
```

## Как посмотреть статус переключения режима registry?

Статус переключения режима registry можно получить с помощью команды:

<!-- TODO(nabokihms): заменить на подкоманду d8, когда она будет реализована -->
```bash
kubectl -n d8-system -o yaml get secret registry-state | yq -C -P '.data | del .state | map_values(@base64d) | .conditions = (.conditions | from_yaml)'
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

| Условие                           | Описание                                                                                                                                                                                               |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ContainerdConfigPreflightReady`  | Состояние проверки конфигурации `containerd`. Проверяется, что на узлах отсутствуют пользовательские auth конфигурации `containerd`.                                                                   |
| `TransitionContainerdConfigReady` | Состояние подготовки конфигурации `containerd` в новый режим. Проверяется, что конфигурация `containerd` успешно подготовлена и содержит одновременно конфигурации нового и старого режима.            |
| `FinalContainerdConfigReady`      | Состояние завершения переключения `containerd` в новый режим. Проверяется, что конфигурация `containerd` успешно применена и содержит конфигурацию нового режима.                                      |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения Deckhouse и Deckhouse компонентов на использование нового реестра. Значение `True` указывает, что Deckhouse успешно переключился на сконфигурированный реестр и готов к работе. |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает.                                                                                                   |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены.                                     |
| `NodeServicesReady`               | Состояние готовности сервисов на узлах. Проверяется, что все необходимые сервисы на узлах успешно запущены и работают.                                                                                 |
| `CleanupNodeServices`             | Состояние очистки сервисов на узлах, если сервисы не нужны для работы желаемого режима. Проверяется, что все ресурсы, связанные с сервисами на узлах, успешно удалены.                                 |
| `Ready`                           | Общее состояние готовности registry к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе.                                                            |
