---
title: "Модуль registry: примеры использования"
description: "Пошаговые примеры переключения между режимами registry в Deckhouse Platform."
---

{% alert level="warning" %}
Если в процессе переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, для устранения проблемы воспользуйтесь [инструкцией](/products/kubernetes-platform/documentation/v1/faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert  %}

## Включение модуля

Чтобы модуль начал управлять тем, как кластер загружает образы, задайте [`mode: Managed`](configuration.html#parameters-mode) и укажите данные хранилища образов контейнеров:

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
схемой, удостоверяющим центром и учётными данными того хранилища образов контейнеров, из которого кластер уже
загружает образы:

```bash
d8 k -n d8-system get secret registry-suggested-config -o jsonpath='{.data.registry-mc\.yaml}' | base64 -d
```

Просмотрите его и примените. Это экономит не столько набор текста, сколько перенос: эти значения
разбросаны по секрету и по docker-конфигурации, и именно при переписывании руками появляется
обрезанный путь или неверный удостоверяющий центр — в единственной настройке, которая решает,
сможет ли кластер загружать образы вообще.

Чтобы посмотреть, как изменение вступает в силу, используйте команды:

```bash
d8 k get registryconfig registry -o jsonpath='{.status}' | jq
d8 k get registrynodes -o custom-columns=\
NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
```

## Включение внутрикластерного кеша

Чтобы включить внутрикластерный кеш, добавьте ModuleConfig модуля настройку [`storage.cache`](configuration.html#parameters-storage-cache) и укажите размер хранилища:

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

На узлах при этом не перенастраивается ничего. Container runtime и так запрашивает у агента информацию про любое хранилище образов контейнеров, а агент начинает в первую очередь обращаться к кешу, оставляя upstream резервным путём. Поэтому отсутствие данных в кеше с первой же минуты означает более медленную загрузку, а не неудачную.

Чтобы проверить наполнение внутрикластерного кеша, используйте команду:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Выключение внутрикластерного кеша — то же изменение в обратную сторону, и такое же безопасное. Блобы на диске остаются нетронутыми, поэтому при повторном включении кеш пополнится уже накопленными данными, а не начнёт наполняться с нуля. Если возвращаться к кешу вы не собираетесь, [освободите занимаемое им место](faq.html#как-удалить-с-узла-оставшиеся-данные-кеша).

## Переход в air-gap

У изолированного кластера нет upstream'а (не указан в параметре [`primary.upstream`](configuration.html#parameters-primary-upstream)). Кеш для такого кластера — единственный источник образов, а путь внутрь — `d8 mirror push`. Поскольку полнота кеша должна быть проверяемой, прежде чем можно будет доверять только ему, в [`storage.source`](configuration.html#parameters-storage-source) необходимо описать ожидаемый набор образов.

Для перевода кластера air-gap выполните следующие действия:

1. Скачайте образы на машину, у которой есть доступ в интернет:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Загрузите их в кластер через эндпоинт публикации:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --registry-login "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --registry-password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

   Эндпоинт публикации существует, только пока включён кеш, задан `publicDomainTemplate` и включён
   модуль `cert-manager`; см. [«Изолированные кластеры»](/modules/registry/#изолированные-кластеры).

1. Опишите ожидаемый набор образов в кеше и уберите upstream:

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

Upstream убирается с узлов не в тот момент, когда его убрали из конфигурации, а когда лидер
кеша будет держать весь ожидаемый набор образов и в неймспейсах платформы (`kube-system` и `d8-*`)
не останется ссылок на upstream. Это единственный переход, который иначе мог бы оставить
все узлы без источника образов, поэтому он ждёт — и сообщает об этом.
Когда upstream убирается, хранилище образов перезапускается, и ресурс RegistryStorage может
ненадолго перейти в `Failed`: это ожидаемо, дождитесь возврата в `Ready`.

{% alert level="warning" %}
Нагрузки в других неймспейсах не учитываются. Если образы ваших нагрузок ссылаются на адрес
upstream, переведите их на адрес внутри кластера до того, как убрать upstream: после его снятия
им неоткуда будет скачать образы.
{% endalert %}

Для проверки статуса перехода используйте команды:

Проверьте, содержит ли лидер кеша весь ожидаемый набор образов и сколько нагрузок платформы ещё
ссылаются на upstream (`upstreamReferences`):

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill,upstreamReferences}'
```

Ненулевое `upstreamReferences` означает, что обновление нагрузок платформы ещё не завершилось
и они не перешли на адрес внутри кластера. Лог `registry-controller` называет до пяти таких
объектов при каждом изменении их количества. Чтобы посмотреть последнюю такую строку, выполните:

```bash
d8 k -n d8-system logs -l app=registry-controller -c registry-controller --prefix --tail=-1 | grep 'still names the upstream' | tail -1
```

Получите значение `effectiveUpstream`:

```bash
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

Пока `effectiveUpstream` заполнен, кластер им пользуется. Когда он опустеет, кластер изолирован.

Если переход не завершается, найдите, что его держит:

- Кеш не полон (`safeToDropUpstream: false`, а у лидера нет `full`): дождитесь наполнения или
  проверьте `error` лидера в `.status.replicas`.
- `upstreamReferences` не опускается до нуля: завершите или исправьте обновление объектов,
  которые называет лог `registry-controller`.
- У лидера `full`, а `upstreamReferences` равно `0`: upstream удерживается, пока `syncer`,
  работающий на узле лидера, не прочитает своё хранилище и не отметит свой отчёт полем
  `syncerStartedAt`, обычно не позже чем через две минуты после запуска. Удержание не закончится,
  пока под хранилища на узле лидера не Ready или пока в нём работает `syncer`, выпущенный до
  появления этого поля (во время обновления модуля). Пока upstream удерживается, под хранилища
  лидера можно безопасно удалить: новый `syncer` прочитает своё хранилище и отметит отчёт, обычно
  не позже чем через две минуты.
  Посмотреть отчёт лидера и причину, которую называет контроллер:

  ```bash
  d8 k get registrystorage registry -o json | jq '.status as $s | $s.replicas[] | select(.node == $s.leader) | {node,full,error,syncerStartedAt}'
  d8 k -n d8-system logs -l app=registry-controller -c registry-controller --prefix --tail=-1 | grep 'the upstream is held' | tail -1
  ```

Чтобы остаться на upstream, верните тот же `primary.upstream` в ModuleConfig: кластер всё это
время им пользовался и продолжит пользоваться.

## Добавление ещё одного хранилища образов контейнеров

Хранилище образов контейнеров, которое не является источником образов компонентов DP, объявляется отдельным ресурсом [RegistryUpstream](cr.html#registryupstream), а не ещё одним полем в ModuleConfig. Пример:

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

После создания RegistryUpstream проверьте, что ресурс принят: конфликт с основным хранилищем или с другим ресурсом, претендующим
на то же имя, отвергается, а не объединяется. Так же отвергается некорректное поле, например `ca`, не являющийся PEM-сертификатом,
и ресурс, который не помещается в ограничения на все принятые ресурсы вместе (256 ресурсов, 256 КиБ `spec`):

```bash
d8 k get registryupstreams -o custom-columns=\
NAME:.metadata.name,MATCH:.spec.match,ACCEPTED:.status.conditions[0].status,REASON:.status.conditions[0].reason
```

## Загрузка из приватного хранилища образов контейнеров без его объявления

Чтобы загружать образы из хранилища, о котором модуль не знает, ничего объявлять не нужно. Когда модуль управляет путём загрузки образов, агент на узле — это то, к чему container runtime обращается за любым хранилищем образов, включая те, о которых у модуля нет информации. Ненастроенное хранилище образов контейнеров проксируется без изменений, вместе с теми учётными данными, которые уже были в запросе.

Поэтому обычный `imagePullSecret` работает точно так же, как в кластере, где этот модуль никогда не включался.

Для создания секрета с учётными данными приватного хранилища образов контейнеров выполните команду:

```bash
d8 k create secret docker-registry my-private-registry \
  --docker-server=private.example.com \
  --docker-username=robot \
  --docker-password=<PASSWORD>
```

Для использования этого секрета при загрузке образов укажите его в `imagePullSecrets` пода:

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

Создавать RegistryUpstream стоит только если вы хотите, чтобы учётные данные хранил кластер, а не каждая нагрузка по отдельности, или если хранилище образов контейнеров требует удостоверяющий центр, которого на узлах нет.

## Выключение управления

Чтобы модуль перестал управлять путём загрузки образов контейнеров, установите параметр [`mode`](configuration.html#parameters-mode) в `Unmanaged`:

```yaml
spec:
  settings:
    mode: Unmanaged
```

После этого модуль не управляет ничем: его компоненты удаляются, написанная им конфигурация узлов отзывается, и кластер возвращается к загрузке образов из того хранилища образов контейнеров, которое записано в секрете `deckhouse-registry`, — то есть оттуда, откуда загружал до включения модуля.

Данные кеша на master-узлах намеренно сохраняются: при повторном включении кеш дополнится только недостающими образами из хранилища образов контейнеров, а не начнёт наполняться заново. Если кеш больше не понадобится, [освободите занимаемое им место](faq.html#как-удалить-с-узла-оставшиеся-данные-кеша).

## Примеры для реализации v1

Всё, что ниже, относится к кластеру, который всё ещё работает на реализации v1
(DP до версии 1.77 включительно), настраиваемой через ModuleConfig `deckhouse`.
Процесс перехода описан в разделе
[«Как устроена миграция на модуль registry»](faq.html#как-устроена-миграция-на-модуль-registry).

### Переключение на режим `Unmanaged`

Для переключения уже работающего кластера на режим `Unmanaged` выполните следующие шаги:

{% alert level="danger" %}
Изменение хранилища образов контейнеров в `Unmanaged` режиме приведёт к перезапуску всех компонентов DP.
{% endalert %}

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

1. Установите настройки режима `Unmanaged` в [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry-unmanaged). Если используется хранилище образов контейнеров, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/) для корректной настройки.

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. Проверьте статус переключения хранилища образов контейнеров в секрете `registry-state`, используя [инструкцию](faq.html#как-посмотреть-статус-переключения-режима-registry).

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

1. При необходимости переключения на старый метод управления хранилищем образов контейнеров, ознакомьтесь с [инструкцией](faq.html#как-мигрировать-обратно-с-модуля-registry).

{% alert level="warning" %}
Это устаревший (deprecated) формат управления хранилищем образов контейнеров.
{% endalert %}
