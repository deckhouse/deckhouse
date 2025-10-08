---
title: "Модуль registry: пример использования"
description: "Пошаговые примеры переключения между режимами Direct и Unmanaged registry в Deckhouse Kubernets Platform, включая примеры конфигурации и мониторинг статуса."
---

## Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` выполните следующие шаги:

{% alert level="danger" %}
При изменении режима registry или параметров registry, Deckhouse будет перезапущен.
{% endalert %}

1. Перед переключением выполните [миграцию на использование модуля `registry`](faq.html#как-мигрировать-на-модуль-registry).

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

1. Проверьте, чтобы [очередь Deckhouse](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse) была пустой и без ошибок.

1. Установите настройки режима `Direct` в ModuleConfig `deckhouse`. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](../deckhouse/) для корректной настройки.

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

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry).

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

## Переключение на режим `Unmanaged`

Для переключения уже работающего кластера на режим `Unmanaged` выполните следующие шаги:

{% alert level="danger" %}
При изменении режима registry или параметров registry, Deckhouse будет перезапущен.
{% endalert %}

1. Перед переключением выполните [миграцию на использование модуля `registry`](faq.html#как-мигрировать-на-модуль-registry).

1. Убедитесь, что модуль `registry` включен и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```

1. Проверьте, чтобы [очередь Deckhouse](../../deckhouse-faq.html#как-проверить-очередь-заданий-в-deckhouse) была пустой и без ошибок.

1. Установите настройки режима `Unmanaged` в ModuleConfig `deckhouse`. Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](../deckhouse/) для корректной настройки.

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

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry).

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

1. При необходимости переключения на старый метод управления registry, ознакомьтесь с [инструкцией](faq.html#как-мигрировать-обратно-с-модуля-registry).

{% alert level="warning" %}
Это устаревший (deprecated) формат управления registry.
{% endalert %}
