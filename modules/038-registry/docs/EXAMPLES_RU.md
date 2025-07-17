---
title: "Модуль registry: пример использования"
description: ""
---

## Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` необходимо выполнить следующие шаги:

{% alert level="danger" %}
Во время первого переключения сервис `Containerd V1` будет перезапущен, так как выполнится переключение на [новую конфигурацию авторизации](./faq.html#как-подготовить-containerd-v1).
{% endalert %}

{% alert level="danger" %}
При изменении режима registry или параметров registry, Deckhouse будет перезапущен.
{% endalert %}

1. Если кластер запущен с `Containerd V1`, [необходимо выполнить подготовку пользовательских конфигураций containerd](./faq.html#как-подготовить-containerd-v1).

<!-- markdownlint-disable MD029 -->
2. Убедитесь, что модуль `registry` включен и работает. Для этого выполните следующую команду:

```bash
kubectl get module registry -o wide
# Пример вывода:
# NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
# registry   38     ...  Ready   True                         True
```

<!-- markdownlint-disable MD029 -->
3. Установите настройки Direct режима в `ModuleConfig` модуля `deckhouse`:

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

{% alert level="warning" %}
Если используется реестр, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/products/kubernetes-platform/documentation/v1/modules/deckhouse/) для корректной настройки.
{% endalert %}

<!-- markdownlint-disable MD029 -->
4. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

```yaml
...
  - lastTransitionTime: "..."
    message: ""
    reason: ""
    status: "True"
    type: Ready
hash: ..
mode: Direct
target_mode: Direct
```

## Переключение на режим `Unmanaged`

{% alert level="warning" %}
Переключение в режим `Unmanaged` доступно только из режима `Direct`. Конфигурационные параметры реестра будут взяты из предыдущего активного режима.
{% endalert %}

{% alert level="danger" %}
При изменении режима registry или параметров registry, Deckhouse будет перезапущен.
{% endalert %}

Для переключения кластера на режим `Unmanaged` необходимо выполнить следующие шаги:

1. Убедитесь, что модуль `registry` запущен в режиме `Direct`, и статус переключения в режим `Direct` имеет значение `Ready`. Проверить состояние можно через секрет `registry-state`, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

```yaml
...
  - lastTransitionTime: "..."
    message: ""
    reason: ""
    status: "True"
    type: Ready
hash: ..
mode: Direct
target_mode: Direct
```

<!-- markdownlint-disable MD029 -->
2. Установите настройки `Unmanaged` режима в `ModuleConfig` модуля `deckhouse`:

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

<!-- markdownlint-disable MD029 -->
3. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](./faq.html#как-посмотреть-статус-переключения-режима-registry). Пример вывода:

```yaml
...
  - lastTransitionTime: "..."
    message: ""
    reason: ""
    status: "True"
    type: Ready
hash: ..
mode: Unmanaged
target_mode: Unmanaged
```
