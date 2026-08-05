---
title: "Модуль registry: пример использования"
description: "Пошаговые примеры переключения между режимами registry в Deckhouse Kubernets Platform."
---

{% alert level="warning" %}
Если в процессе переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, для устранения проблемы воспользуйтесь [инструкцией](/products/kubernetes-platform/documentation/v1/faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert  %}

## Включение модуля

Чтобы модуль начал управлять тем, как кластер загружает образы, задайте `mode: Managed` и
укажите registry:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: registry
spec:
  version: 1
  enabled: true
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        scheme: HTTPS
        auth:
          license: <LICENSE_KEY>
```

Модуль публикует готовый вариант этой конфигурации для вашего кластера — с адресом, путём,
схемой, удостоверяющим центром и учётными данными того registry, из которого кластер уже
загружает образы:

```bash
d8 k -n d8-system get secret registry-suggested-config -o jsonpath='{.data.registry-mc\.yaml}' | base64 -d
```

Просмотрите его и примените. Это экономит не столько набор текста, сколько перенос: эти значения
разбросаны по секрету и по docker-конфигурации, и именно при переписывании руками появляется
обрезанный путь или неверный удостоверяющий центр — в единственной настройке, которая решает,
сможет ли кластер загружать образы вообще.

Посмотреть, как изменение вступает в силу:

```bash
d8 k get registryconfig registry -o jsonpath='{.status}' | jq
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
```

## Включение внутрикластерного кэша

Добавьте `storage.cache` и размер хранилища:

```yaml
spec:
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        auth:
          license: <LICENSE_KEY>
    storage:
      cache: true
      size: 50Gi
```

На узлах при этом не перенастраивается ничего. Container runtime и так спрашивает агент про
любой registry, а агент начинает предпочитать кэш, оставляя upstream резервным путём, — поэтому
промах кэша с первой же минуты означает более медленную загрузку, а не неудачную.

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Выключение — то же изменение в обратную сторону, и такое же безопасное. Блобы на диске
остаются нетронутыми, поэтому повторное включение дольётся из того, что уже есть, а не начнёт с
нуля — см. [как освободить это место](faq.html#на-узле-остались-данные-кэша-которые-никто-не-использует),
если возвращаться к кэшу вы не собираетесь.

## Переход в air-gap

У изолированного кластера нет upstream'а: кэш — единственный источник образов, а путь внутрь —
`d8 mirror push`. Поскольку полнота кэша должна быть проверяемой, прежде чем ему можно
доверять в одиночку, `storage.source` описывает ожидаемый набор образов.

1. Скачайте образы там, где есть доступ в интернет:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Загрузите их в кластер через endpoint публикации:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --username "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

1. Опишите, что кэш должен держать, и уберите upstream:

   ```yaml
   spec:
     settings:
       mode: Managed
       storage:
         cache: true
         size: 50Gi
         source:
           bundleRef: d8-mirror-bundle
           expectedDigests: 459
   ```

Upstream убирается с узлов не в тот момент, когда вы убрали его из конфигурации, а когда лидер
кэша будет держать весь ожидаемый набор. Это единственный переход, который иначе мог бы оставить
все узлы без источника образов, поэтому он ждёт — и сообщает об этом:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill}'
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

Пока `effectiveUpstream` заполнен, кластер им пользуется. Когда он опустеет, кластер изолирован.

## Добавление ещё одного registry

Registry, который не является источником образов компонентов Deckhouse, объявляется отдельным
ресурсом, а не ещё одним полем в ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: RegistryUpstream
metadata:
  name: virtualization-images
spec:
  match: images.virtualization.example.com
  upstream:
    host: vendor.example.com
    path: /virtualization
    scheme: HTTPS
    auth:
      username: robot
      password: <PASSWORD>
```

После этого загрузки, обращающиеся к `images.virtualization.example.com`, маршрутизируются
агентом на каждом узле в `vendor.example.com/virtualization`, а учётные данные и удостоверяющий
центр держит кластер, а не каждая нагрузка по отдельности. На узлах для этого не
перенастраивается ничего.

Проверьте, что ресурс принят: конфликт с основным registry или с другим ресурсом, претендующим
на то же имя, отвергается, а не объединяется:

```bash
d8 k get registryupstreams -o custom-columns=\
NAME:.metadata.name,MATCH:.spec.match,ACCEPTED:.status.conditions[0].status,REASON:.status.conditions[0].reason
```

## Загрузка из приватного registry без его объявления

Объявлять ничего не нужно. Агент на узле проксирует незнакомый ему registry без изменений,
вместе с теми учётными данными, которые уже были в запросе, поэтому обычный `imagePullSecret`
ведёт себя точно так же, как на кластере, где этот модуль никогда не включался:

```bash
d8 k create secret docker-registry my-private-registry \
  --docker-server=private.example.com \
  --docker-username=robot \
  --docker-password=<PASSWORD>
```

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
spec:
  imagePullSecrets:
  - name: my-private-registry
  containers:
  - name: app
    image: private.example.com/team/app:v1
```

`RegistryUpstream` вместо этого стоит объявлять, если вы хотите, чтобы учётные данные хранил
кластер, или если registry требует удостоверяющий центр, которого на узлах нет.

## Выключение управления

Верните режим в `Unmanaged`:

```yaml
spec:
  settings:
    mode: Unmanaged
```

После этого модуль не управляет ничем: его компоненты удаляются, написанная им конфигурация
узлов отзывается, и кластер возвращается к загрузке образов из того registry, который записан в
секрете `deckhouse-registry`, — то есть оттуда, откуда загружал до включения модуля.

Данные кэша на master-узлах намеренно остаются, чтобы повторное включение долилось из того, что
уже есть. См.
[как освободить это место](faq.html#на-узле-остались-данные-кэша-которые-никто-не-использует).

## Примеры для предыдущей реализации

Всё, что ниже, относится к кластеру, который всё ещё работает на реализации, настраиваемой
через ModuleConfig `deckhouse`. См.
[как завершить миграцию](faq.html#как-завершить-миграцию).

### Переключение на режим `Direct`

Для переключения уже работающего кластера на режим `Direct` выполните следующие шаги:

{% alert level="danger" %}
При первом переключении с режима `Unmanaged` на режим `Direct` произойдёт полный перезапуск всех компонентов DKP.
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

1. Установите настройки режима `Direct` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-direct). Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

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

### Переключение на режим `Proxy`

Для переключения уже работающего кластера на режим `Proxy` выполните следующие шаги:

{% alert level="danger" %}
- При первом переключении с режима `Unmanaged` на режим `Proxy` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Local` в `Proxy` недоступно. Для переключения из режима `Local` необходимо переключить registry на другой доступный режим (например, `Direct`).
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

1. Установите настройки режима `Proxy` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-proxy). Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

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
   mode: Proxy
   target_mode: Proxy
   ```

### Переключение на режим `Local`

Для переключения уже работающего кластера на режим `Local` выполните следующие шаги:

{% alert level="danger" %}
- При первом переключении с режима `Unmanaged` на режим `Local` произойдёт полный перезапуск всех компонентов DKP.
- Переключение из режима `Proxy` в `Local` недоступно. Для переключения из режима `Proxy` необходимо переключить registry на другой доступный режим (например, `Direct`).
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

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry). В статусе необходимо дождаться запуска проверки `RegistryContainsRequiredImages`. Условие отобразит отсутствие или наличие образов в запущенном локальном registry.

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

1. Загрузите образы в локальный registry с помощью команды `d8 mirror`. Загрузка образов в локальный registry осуществляется через Ingress по адресу `registry.${PUBLIC_DOMAIN}`.

   Получите пароль read-write пользователя локального registry:

   ```bash
   $ d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
   name: rw
   password: KFVxXZGuqKkkumPz
   passwordHash: $2a$10$Phjbr6iinLf00ZZDD2Y7O.p9H3nDOgYzFmpYKW5eydGvIsdaHQY0a
   ```

   Загрузите образы в локальный registry:

   ```bash
   d8 mirror push \
   --registry-login="rw" \
   --registry-password="KFVxXZGuqKkkumPz" \
   /home/user/d8-bundle \
   registry.${PUBLIC_DOMAIN}/system/deckhouse
   ```

1. Проверьте статус переключения registry в секрете `registry-state`, используя [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry). После загрузки образов статус `RegistryContainsRequiredImages` должен быть в состоянии `Ready`

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

1. Дождитесь завершения переключения. Для проверки статуса переключения воспользуйтесь [инструкцией](faq.html#как-посмотреть-статус-переключения-режима-registry).

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

### Переключение на режим `Unmanaged`

Для переключения уже работающего кластера на режим `Unmanaged` выполните следующие шаги:

{% alert level="danger" %}
Изменение registry в `Unmanaged` режиме приведёт к перезапуску всех компонентов DKP.
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

1. Установите настройки режима `Unmanaged` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). Если используется registry, отличный от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

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
