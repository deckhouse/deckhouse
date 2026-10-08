---
title: Управление хранилищем образов в кластерах, полностью управляемых DP
permalink: ru/admin/configuration/registry/managing-interaction.html
description: "Управление настройками хранилища образов компонентов платформы в Deckhouse Platform. Режимы взаимодействия с хранилищем компонентов DP"
lang: ru
---

В кластерах, полностью управляемых Deckhouse Platform (DP), вы можете настраивать пути загрузки:

- образов контейнеров компонентов DP;
- образов контейнеров из дополнительных хранилищ (вендоров или ваших собственных).

Управление путями загрузки образов контейнеров реализуется с помощью модуля [`registry`](/modules/registry/).

## Реализации управления путями загрузки образов контейнеров

В DP поддерживаются две реализации модуля `registry` для управления путями загрузки образов контейнеров:

- **Текущая реализация**. Используется, начиная с версии DP 1.78. Управление путями загрузки образов контейнеров настраивается через [ModuleConfig `registry`](/modules/registry/configuration.html). Реализация описана в разделе [«Текущая реализация»](#текущая-реализация).
- **Предыдущая реализация**. Хранилище образов компонентов DP настраивается через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Реализация описана в разделе [«Предыдущая реализация»](#предыдущая-реализация).

{% alert level="warning" %}
В будущих релизах DP планируется отказ от предыдущей реализации модуля.
{% endalert %}

Обе реализации никогда не управляют кластером одновременно: в определенный момент времени активна только одна из них.
Кластер, в котором предыдущая реализация никогда не работала, сразу использует текущую.
Кластер, в котором работает предыдущая реализация, продолжает использовать её до тех пор, пока не будут выполнены условия перехода на текущую — тогда управление автоматически передаётся текущей реализации, без отдельной команды. Условия перехода и пошаговые инструкции для каждого режима предыдущей реализации приведены в разделе [«Миграция с предыдущей реализации на текущую»](#миграция-с-предыдущей-реализации-на-текущую) ниже.

{% alert level="warning" %}
В будущих релизах DP планируется отказ от предыдущей реализации модуля. Рекомендуется заранее перейти на текущую реализацию. Переход возможен, начиная с релиза 1.78.
{% endalert %}

{% alert level="danger" %}
Заранее подготовьте кластер к переходу на текущую реализацию. После удаления предыдущей реализации обновление DP будет блокироваться, пока кластер работает в режиме `Proxy` или `Local` предыдущей реализации, либо в режиме `Direct` без настроенного ModuleConfig `registry`.
{% endalert %}

Узнать, какая реализация модуля используется в кластере, можно по наличию секрета `registry-v2-switch`. Для этого воспользуйтесь [инструкцией](#проверка-реализации-используемой-в-кластере).

## Текущая реализация

В текущей реализации управление путями загрузки образов контейнеров настраивается через [ModuleConfig `registry`](/modules/registry/configuration.html).

### Режимы работы

В текущей реализации в DP поддерживаются следующие режимы управления путями загрузки образов контейнеров (режим задаётся параметром [`mode`](/modules/registry/cr.html#registryconfig-v1alpha1-spec-mode) в ModuleConfig `registry`):

- `Unmanaged` (режим по умолчанию). Модуль `registry` не управляет путями загрузки образов компонентов DP: кластер загружает образы из того хранилища, с которым был установлен (хранилище указывается при бутстрапе кластера в параметре [`deckhouse`](../../../reference/api/cr.html#initconfiguration-deckhouse) в InitConfiguration).
- `Managed`. В этом режиме модуль `registry` управляет путями загрузки образов контейнеров. При работе в этом режиме используются следующие независящие друг от друга настройки:

  - [`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream) — хранилище образов контейнеров, откуда загружаются образы компонентов DP. Если параметр не задан, кластер считается изолированным (air-gapped). В изолированном кластере единственным источником образов контейнеров становится внутрикластерный кеш, который наполняется командой [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push).
  - [`storage.cache`](/modules/registry/configuration.html#parameters-storage-cache) — управление внутрикластерным кешем на master-узлах. Если параметр включён, на master-узлах разворачивается хранилище образов контейнеров, и все узлы получают образы из него, а upstream-хранилище остаётся резервным путём, пока кеш наполняется.

Параметры [`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream) и [`storage.cache`](/modules/registry/configuration.html#parameters-storage-cache) вместе покрывают все поддерживаемые конфигурации:

| `primary.upstream` | `storage.cache` | Что делает кластер |
|---|---|---|
| Задан | `false` | Узлы загружают образы напрямую из upstream-хранилища. На master-узлах ничего не разворачивается |
| Задан | `true` | Кеш работает на проход: наполняется из upstream-хранилища по запросу и заранее |
| Не задан | `true` | Кластер изолирован (air-gapped). Кеш — единственный источник. Для наполнения используется команда [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push) |
| Не задан | `false` | Конфигурация отвергается: узлам было бы неоткуда загружать образы |

В любой момент можно перенастроить включение и выключение кеша, смену хранилища образов и смену учётных данных. Однако при удалении upstream-хранилища для перехода в изолированное состояние модуль ожидает, пока во внутрикластерном кеше появится весь ожидаемый набор образов — только после этого прекращается использование upstream-хранилища. Если бы использование upstream-хранилища прекратилось до того момента, пока во внутрикластерном кеше появился весь ожидаемый набор образов, узлы могли бы остаться без источника образов на время наполнения кеша.

Подробнее об архитектуре текущей реализации (об агенте на узле, внутрикластерном кеше, выборе главной реплики, работе изолированных кластеров) — в [документации модуля `registry`](/modules/registry/#текущая-реализация).

### Сборка мусора

Реплики хранилища по расписанию выполняют сборку мусора.

Сборка мусора включена по умолчанию и запускается раз в сутки в ночное время (в 03:17 МСК).
Если нужно отключить или настроить сборку мусора, используйте параметр [`storage.garbageCollection`](/modules/registry/configuration.html#parameters-storage-garbagecollection) ModuleConfig `registry`.

{% alert level="warning" %}
Отключение сборки мусора имеет смысл только в случае, если для хранилища используется диск, размера которого хватит при неограниченном росте объёма хранилища.
{% endalert %}

Чтобы посмотреть состояние и расписание сборки мусора, используйте команды ниже.

- Просмотр времени последней сборки мусора на каждой реплике и ошибки, если они были:

  ```bash
  d8 k get registrystorage registry -o jsonpath='{.status.replicas}' | jq \
    'map({node, collectedAt, collectionError})'
  ```

- Просмотр текущего расписания сборки мусора:

  ```bash
  d8 k get registrystorage registry -o jsonpath='{.spec.garbageCollection}' | jq
  ```

Сборка мусора — единственный механизм, выполняющий удаление данных из хранилища образов. Каждый релиз DP добавляет новые образы, поэтому без сборки мусора хранилище кластера, работающего продолжительное время, рано или поздно заполнится, и новые образы в него перестанут добавляться. При этом, например, изолированный кластер с заполненным хранилищем нельзя обновить.

Сборка мусора удаляет образы релизов, которые кластер уже прошёл. Сохраняются:

- развёрнутый релиз и предыдущий — чтобы откат не скачивал заново то, к чему откатывается;
- всё, что новее развёрнутого релиза, — это обновление в процессе или, в изолированном
  кластере, релиз, загруженный намеренно;
- все теги, которые не являются версиями: имена каналов обновлений вроде `stable`, плавающие
  теги, всё загруженное вручную. Сборщик не может знать, что они означают, поэтому не удаляет их.

Сборка мусора выполняется с осторожностью. В изолированном кластере удаление ещё нужного блоба невосстановимо
без повторного выполнения команды `d8 mirror push`, а хранение ненужного стоит только места на диске. Поэтому
если сборщик мусора ничего не делает с объектом, если не может явно определить необходимость его удаления (например, когда не найден развёрнутый релиз).

Подробнее о работе сборщика мусора — [в документации модуля `registry`](/modules/registry/faq.html#кеш-растёт-что-его-чистит).

### Дополнительные хранилища образов контейнеров

[`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream) — единственное хранилище образов, настраиваемое в самом ModuleConfig `registry`. Оно используется для получения образов компонентов DP. Если нужны дополнительные хранилища (от вендора, нужные какому-то модулю, или ваши собственные), используйте для их добавления кастомный ресурс [RegistryUpstream](/modules/registry/cr.html#registryupstream).

Пример добавления дополнительного хранилища описан в разделе [«Добавление дополнительного хранилища образов контейнеров»](#добавление-дополнительного-хранилища-образов-контейнеров).

### Требования к использованию текущей реализации

Перед включением управления путями загрузки образов контейнеров с помощью текущей реализации модуля `registry` убедитесь, что кластер соответствует следующим требованиям:

- На узлах используется containerd или containerd v2 — задаётся параметром [`defaultCRI`](../../../reference/api/cr.html#clusterconfiguration-defaultcri) в ClusterConfiguration.
- Кластер полностью управляется DP. В Managed Kubernetes-кластерах текущая реализация модуля не работает — используйте [переключение на стороннее хранилище](third-party.html).

### Примеры настройки текущей реализации

{% alert level="warning" %}
Если в процессе переключения на текущую реализацию образ какого-либо модуля не загрузился заново и модуль не переустановился, для устранения проблемы воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert %}

#### Настройка управления путями загрузки образов компонентов DP

Чтобы включить управление путями загрузки образов контейнеров компонентов DP с помощью модуля `registry`, включите модуль в режиме `Managed` и укажите данные хранилища образов в [`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream). Пример ModuleConfig `registry`:

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
          license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
```

Вы можете не создавать ModuleConfig с нуля, а использовать готовый.
Модуль публикует готовый вариант конфигурации для вашего кластера — с адресом, путём, схемой, удостоверяющим центром и учётными данными того хранилища, из которого кластер уже загружает образы.

Чтобы получить готовую конфигурацию ModuleConfig, выполните команду:

```bash
d8 k -n d8-system get secret registry-suggested-config -o jsonpath='{.data.registry-mc\.yaml}' | base64 -d
```

Просмотрите его и примените — это исключает ошибки, которые легко допустить, переписывая эти значения вручную (обрезанный путь, неверный удостоверяющий центр и т. д.).

Чтобы посмотреть, как изменение вступает в силу, используйте команды ниже.

- Просмотр состояния конфигурации хранилища образов в DP:

  ```bash
  d8 k get registryconfig registry -o jsonpath='{.status}' | jq
  ```

- Проверка применения конфигурации хранилища образов на узлах, всё ли успешно согласовано и какие бэкенды сейчас активны:

  ```bash
  d8 k get registrynodes -o custom-columns=\
  NODE:.metadata.name,APPLIED:.status.observedGeneration,OK:.status.reconciled,BACKENDS:.status.activeBackends
  ```

#### Включение внутрикластерного кеша

{% alert level="info" %}
Внутрикластерный кеш использует диски master-узлов и поддерживается только на статических кластерах.
{% endalert %}

Чтобы включить внутрикластерный кеш, добавьте в ModuleConfig `registry` настройку [`storage.cache`](/modules/registry/configuration.html#parameters-storage-cache) и укажите размер хранилища:

```yaml
spec:
  settings:
    mode: Managed
    primary:
      upstream:
        host: registry.deckhouse.io
        path: /deckhouse/ee
        auth:
          license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
    storage:
      cache: true
      size: 50Gi
```

На узлах при этом ничто не перенастраивается: container runtime и так обращается за любым хранилищем к агенту, а агент начинает в первую очередь использовать кеш. Upstream-хранилище используется как резервный путь. Поэтому отсутствие данных в кеше с первой же минуты после включения кеша означает более медленную загрузку, а не неудачную.

Чтобы проверить наполнение внутрикластерного кеша, используйте команду:

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{phase,fill,leader,allReplicasFull}'
```

Выключение кеша — то же изменение в обратную сторону, и такое же безопасное. Блобы на диске остаются нетронутыми, и при повторном включении кеш пополнится уже накопленными данными. Если вы отключаете внутрикластерный кеш и не планируете его включение в дальнейшем, удалите оставшиеся данные кеша с узлов кластера. Подробная инструкция — в [FAQ модуля `registry`](/modules/registry/faq.html#как-удалить-с-узла-оставшиеся-данные-кеша).

#### Перевод кластера в изолированное состояние

У изолированного кластера нет upstream-хранилища (не указан в [`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream)). Кеш для такого кластера — единственный источник образов. Для наполнения хранилища используется команда [`d8 mirror push`](../../../cli/d8/reference/#d8-mirror-push). Чтобы можно было проверить наличие всех необходимых образов в кеше, опишите ожидаемый набор образов [в параметре `storage.source`](/modules/registry/configuration.html#parameters-storage-source).

Для перевода кластера в изолированное состояние выполните следующие действия:

1. Скачайте образы на машину, у которой есть доступ в интернет:

   ```bash
   d8 mirror pull --license <LICENSE_KEY> ./d8-bundle
   ```

1. Загрузите их в кластер через эндпоинт публикации:

   ```bash
   PUSH_SECRET=$(d8 k -n d8-system get secret registry-storage-push -o json)
   d8 mirror push ./d8-bundle registry.example.com/system/deckhouse \
     --username "$(echo "$PUSH_SECRET" | jq -r .data.username | base64 -d)" \
     --password "$(echo "$PUSH_SECRET" | jq -r .data.password | base64 -d)"
   ```

1. Опишите ожидаемый набор образов в кеше и уберите `upstream` из ModuleConfig `registry`:

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

Upstream-хранилище убирается с узлов не в момент правки конфигурации, а когда лидер кеша соберёт весь ожидаемый набор образов — иначе все узлы могли бы остаться без источника образов. Для проверки статуса перехода используйте команды:

Проверка, содержит ли лидер кеша весь ожидаемый набор образов.

```bash
d8 k get registrystorage registry -o jsonpath='{.status}' | jq '{safeToDropUpstream,fill}'
```

Проверка, изолирован ли кластер (пока заполнено значение `effectiveUpstream`, кластер им пользуется, если оно пустое — кластер изолирован):

```bash
d8 k get registryconfig registry -o jsonpath='{.status.effectiveUpstream}' | jq
```

#### Добавление дополнительного хранилища образов контейнеров

{% alert level="info" %}
Запросы к дополнительным хранилищам всегда маршрутизируются через агент на узле (учётные данные и удостоверяющий центр хранятся в одном месте) и образы из них не кешируются.
{% endalert %}

Чтобы добавить дополнительное хранилище образов (не для системных компонентов DP), используйте ресурс [RegistryUpstream](/modules/registry/cr.html#registryupstream). Пример:

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
    auth:
      username: robot
      password: <PASSWORD>
```

После создания ресурса проверьте, что он принят — конфликт с основным хранилищем или с другим ресурсом, претендующим на то же имя, отвергается, а не объединяется:

```bash
d8 k get registryupstreams -o custom-columns=\
NAME:.metadata.name,MATCH:.spec.match,ACCEPTED:.status.conditions[0].status,REASON:.status.conditions[0].reason
```

В примере выше после добавления дополнительного хранилища запросы на загрузку образов, поступающие на `images.virtualization.example.com`, маршрутизируются агентом на каждом узле в `vendor.example.com/virtualization`, а учётные данные и удостоверяющий центр держит кластер, а не каждая нагрузка по отдельности. На узлах для этого не перенастраивается ничего.

#### Загрузка из приватного хранилища образов контейнеров без его объявления

Чтобы загружать образы из неизвестного модулю хранилища, не требуется объявлять что-либо дополнительно. Агент на узле проксирует такие запросы без изменений, вместе с теми учётными данными, которые уже были в запросе. Поэтому обычный `imagePullSecret` работает так же, как в кластере, где модуль никогда не включался.

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

Создавать для хранилища [RegistryUpstream](/modules/registry/cr.html#registryupstream) стоит только если нужно, чтобы учётные данные хранил кластер, а не каждая нагрузка по отдельности, либо если хранилищу нужен удостоверяющий центр, которого нет на узлах.

#### Выключение управления путями загрузки образов

Чтобы выключить управление путями загрузки образов, установите [`mode: Unmanaged`](/modules/registry/configuration.html#parameters-mode) в ModuleConfig `registry`:

```yaml
spec:
  settings:
    mode: Unmanaged
```

После установки`mode: Unmanaged` управление путями загрузки образов через модуль `registry` выключается. Кластер возвращается к загрузке образов из того хранилища образов контейнеров, которое записано в секрете `deckhouse-registry`, — то есть оттуда, откуда образы загружались до включения модуля. Данные кеша на master-узлах при этом намеренно сохраняются: при повторном включении управления путями загрузки кеш дополнится только недостающими образами из хранилища образов контейнеров, а не начнёт наполняться заново. Если кеш больше не понадобится, освободите занимаемое им место по [инструкции](/modules/registry/faq.html#как-удалить-с-узла-оставшиеся-данные-кеша) из документации модуля `registry`.

## Предыдущая реализация

{% alert level="warning" %}
Описанная ниже предыдущая реализация модуля `registry` постепенно выводится из эксплуатации. Для новых кластеров и при первой возможности для существующих используйте [текущую реализацию](#текущая-реализация).
{% endalert %}

В предыдущей реализации DP поддерживается несколько режимов управления настройками хранилища образов компонентов платформы: `Unmanaged`, `Direct`, `Proxy` и `Local`. В режимах `Direct`, `Proxy` и `Local` для обращения к хранилищу образов компонентов DP используется фиксированный виртуальный адрес. Это позволяет избежать перезапуска всех компонентов control plane и повторного скачивания образов в случае каких-либо изменений в хранилище образов компонентов платформы.

В режиме `Unmanaged` не используется виртуальный адрес. Обращение выполняется напрямую к внешнему хранилищу: в случае изменения хранилища происходит перезапуск всех компонентов DP.

Переключение между режимами и хранилищами образов выполняется через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry). Переключение выполняется автоматически (подробнее — в примерах переключения ниже).

Архитектура режимов предыдущей реализации описана в разделе [«Модуль registry»](../../../architecture/deckhouse/registry.html).

Особенности режимов управления настройками хранилища образов компонентов DP:

- `Direct` — использование прямого доступа к **внешнему** хранилищу по фиксированному виртуальному адресу `registry.d8-system.svc:5001/system/deckhouse`. Фиксированный адрес позволяет избежать повторного скачивания образов и перезапуска компонентов при изменении параметров хранилища.
- `Proxy` — использование **внутреннего кеширующего прокси-хранилища** с обращением к **внешнему** хранилищу. Кеширующее прокси-хранилище запускается на control-plane (master) узлах. Режим позволяет сократить количество запросов к внешнему хранилищу за счёт кеширования образов. Обращение к внутреннему хранилищу образов выполняется по фиксированному виртуальному адресу `registry.d8-system.svc:5001/system/deckhouse` аналогично режиму `Direct`.
- `Local` — использование локального **внутреннего** хранилища образов, с запуском хранилища на control-plane (master) узлах. Режим позволяет кластеру работать в изолированной среде. Обращение к внутреннему хранилищу образов выполняется по фиксированному виртуальному адресу `registry.d8-system.svc:5001/system/deckhouse` аналогично `Direct` и `Proxy` режимам.
- `Unmanaged` (конфигурируемый режим) — работа без использования внутреннего хранилища образов. Виртуальный адрес не используется. Обращение выполняется напрямую к **внешнему** хранилищу.

{% alert level="warning" %}
В этом документе упоминаются конфигурируемый режим `Unmanaged` (предыдущей реализации модуля `registry`) и неконфигурируемый режим `Unmanaged` (полностью устаревший формат, без модуля `registry`).

Неконфигурируемый режим `Unmanaged` не использует модуль `registry` вообще (например, в Managed Kubernetes-кластерах). Параметры конфигурации хранилища образов компонентов DP задаются при установке кластера, или при изменении в развёрнутом кластере с помощью утилиты `helper change registry` (deprecated).
{% endalert %}

### Ограничения по настройкам хранилища образов компонентов DP

Существует ряд ограничений и особенностей, связанных с установкой кластера, условиями использования предыдущей реализации и переключением между её режимами.

#### Ограничения при установке кластера

Ограничения по настройке хранилища образов компонентов DP при установке кластера следующие:

- Бутстрап кластера DP поддерживается в режимах `Direct`, `Unmanaged`, `Proxy` и `Local`. Параметры доступа к хранилищу образов компонентов DP во время установки кластера настраивается через [ModuleConfig `deckhouse`](/modules/deckhouse/configuration.html#parameters-registry).
- Бутстрап кластера в режимах `Local` и `Proxy` поддерживаются только на статичных кластерах.
- Для запуска кластера в неконфигурируемом `Unmanaged` режиме (Legacy, без использования модуля `registry`) необходимо указать параметры хранилища образов в [InitConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo).

#### Ограничения по условиям работы

Для управления настройками хранилища образов компонентов DP в предыдущей реализации необходимо соблюдение следующих условий:

- Использование CRI containerd или containerd v2 на узлах кластера. Для настройки CRI ознакомьтесь с конфигурацией [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
- Кластер полностью управляется DP. В кластерах Managed Kubernetes настройка хранилища образов компонентов DP с помощью модуля `registry` недоступна.
Режимы `Local` и `Proxy` поддерживаются только в статичных кластерах.

#### Ограничения по переключению режимов

Ограничения по переключению режимов следующие:

- Изменение параметров хранилища образов компонентов DP и переключение режимов доступны только после полного завершения этапа бутстрапа.
- При первом переключении необходимо выполнить миграцию пользовательских конфигураций хранилища. Подробнее — в разделе [«Модуль registry: FAQ»](/modules/registry/faq.html).
- Переключение в неконфигурируемый режим `Unmanaged` (без использования модуля `registry`) доступно только из конфигурируемого `Unmanaged` режима. Подробнее — в разделе [«Модуль registry: FAQ»](/modules/registry/faq.html).
- Прямое переключение между режимами `Local` и `Proxy` возможно только через промежуточные режимы `Direct` или `Unmanaged`. Пример последовательности переключения: `Local`/`Proxy` → `Direct` → `Proxy`/`Local`. Также не поддерживается прямое переключение с режима `Local` на неконфигурируемый `Unmanaged` (без использования модуля `registry`): при необходимости выполняйте переключение через промежуточный режим `Direct` или `Unmanaged`.

### Примеры переключения режимов предыдущей реализации

{% alert level="warning" %}
Если в процессе переключения образ какого-либо модуля не загрузился заново и модуль не переустановился, для устранения проблемы воспользуйтесь [инструкцией](../../../faq.html#что-делать-если-образ-модуля-не-скачался-и-модуль-не-переустанов).
{% endalert %}

#### Переключение на режим Direct

Для переключения уже работающего кластера на режим `Direct` выполните следующие шаги:

{% alert level="danger" %}
При первом переключении с режима `Unmanaged` на режим `Direct` произойдёт полный перезапуск всех компонентов DP.
{% endalert %}

1. Если в кластере используется неконфигурируемый `Unmanaged` управления настройками хранилища образов компонентов DP (без модуля `registry`), перед переключением выполните [миграцию на формат управления настройками хранилища образов с использованием модуля `registry`](#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`. Для этого используйте следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте, чтобы очередь DP была пустой и без ошибок:

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации).

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

1. Если необходимо отключить автоматическое обновление DP до новых версий, загружая их из настроенного хранилища образов, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
   После этого автоматическое обновление платформы будет отключено, и управлять версией DP потребуется вручную.

#### Переключение на режим Proxy

{% alert level="danger" %}

- При первом переключении с режима `Unmanaged` на режим `Proxy` произойдёт полный перезапуск всех компонентов DP.
- Переключение из режима `Local` в `Proxy` недоступно. Для переключения из режима `Local` необходимо переключить хранилище образов на другой доступный режим (например, `Direct`).
{% endalert %}

Для переключения уже работающего кластера на режим `Proxy` выполните следующие шаги:

1. Если в кластере используется неконфигурируемый `Unmanaged` режим управления настройками хранилища образов компонентов DP (без модуля `registry`), перед переключением выполните [миграцию на формат управления настройками хранилища образов с использованием модуля `registry`](#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`, используя следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте, чтобы очередь DP была пустой и без ошибок:

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации).

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

1. Если необходимо отключить автоматическое обновление DP до новых версий, загружая их из настроенного хранилища образов, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
   После этого автоматическое обновление платформы будет отключено, и управлять версией DP потребуется вручную.

#### Переключение на режим Local

{% alert level="danger" %}

- При первом переключении с режима `Unmanaged` на режим `Local` произойдёт полный перезапуск всех компонентов DP.
- Переключение из режима `Proxy` в `Local` недоступно. Для переключения из режима `Proxy` необходимо переключить хранилище образов на другой доступный режим (например, `Direct`).
{% endalert %}

Для переключения уже работающего кластера на режим `Local` выполните следующие шаги:

1. Если в кластере используется неконфигурируемый `Unmanaged` режим управления настройками хранилища образов компонентов DP (без модуля `registry`), перед переключением выполните [миграцию на формат управления настройками хранилища образов с использованием модуля `registry`](#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Убедитесь, что все master-узлы находятся в состоянии `Ready` и не имеют статуса `SchedulingDisabled`, используя следующую команду:

   ```bash
   d8 k get nodes
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS   ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   Пример вывода, когда master-узел (`master-2` в примере) находится в статусе `SchedulingDisabled`:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       STATUS                      ROLES                 ...
   master-0   Ready    control-plane,master  ...
   master-1   Ready    control-plane,master  ...
   master-2   Ready,SchedulingDisabled    control-plane,master  ...
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте, чтобы очередь DP была пустой и без ошибок:

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

1. Подготовьте архивы с образами DP текущей версии. Для этого воспользуйтесь командой `d8 mirror`.

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

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации). В статусе необходимо дождаться появления проверки `RegistryContainsRequiredImages`. Условие отобразит отсутствие или наличие образов в запущенном локальном хранилище образов.

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
   d8 k -n d8-system get secret/registry-user-rw -o json | jq -r '.data | to_entries[] | "\(.key): \(.value | @base64d)"'
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

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации). После загрузки образов статус `RegistryContainsRequiredImages` должен быть в состоянии `Ready`

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

1. Дождитесь завершения переключения. Для проверки статуса переключения воспользуйтесь [инструкцией](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации).

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

#### Переключение на режим Unmanaged

Для переключения уже работающего кластера на режим `Unmanaged` выполните следующие шаги:

{% alert level="danger" %}
Изменение хранилища образов в `Unmanaged` режиме приведёт к перезапуску всех компонентов DP.
{% endalert %}

1. Если в кластере используется неконфигурируемый `Unmanaged` режим управления хранилищем образов компонентов DP (без модуля `registry`), перед переключением выполните [миграцию на формат управления хранилищем образов с использованием модуля `registry`](#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

1. Убедитесь, что модуль `registry` включён и работает. Для этого выполните следующую команду:

   ```bash
   d8 k get module registry -o wide
   ```

   Пример вывода:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       WEIGHT ...  PHASE   ENABLED   DISABLED MESSAGE   READY
   registry   38     ...  Ready   True                         True
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Проверьте, чтобы очередь DP была пустой и без ошибок:

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. Проверьте статус переключения хранилища образов в секрете `registry-state`, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации).

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

1. Если необходимо отключить автоматическое обновление DP до новых версий, загружая их из настроенного хранилища образов, удалите параметр `releaseChannel` из конфигурации модуля `deckhouse`.
   После этого автоматическое обновление платформы будет отключено, и управлять версией DP потребуется вручную.

При необходимости переключения на старый метод управления хранилищем образов (без модуля `registry`), ознакомьтесь с [инструкцией](#миграция-на-устаревший-формат-управления-настройками-хранилища-образов-компонентов-dp-без-модуля-registry).

{% alert level="warning" %}
Управление хранилищем образов компонентов DP без модуля `registry` — устаревший (deprecated) формат.
{% endalert %}

## Миграция с предыдущей реализации на текущую

Ниже описан переход с предыдущей реализации модуля `registry` на текущую. Если ваш кластер вообще не использует модуль `registry` (полностью устаревший формат), сначала обратитесь к разделу [«Миграция на формат управления настройками хранилища образов с использованием модуля `registry`»](#миграция-на-формат-управления-настройками-хранилища-образов-с-использованием-модуля-registry).

Миграция — это передача пути загрузки текущей реализации от предыдущей.
Её не нужно запускать вручную, и отдельной команды для неё нет: модуль `registry` принимает управление автоматически после обновления кластера на релиз, в котором появилась текущая реализация. Единственное условие — к этому моменту предыдущая реализация должна освободить путь загрузки. Обе реализации настраивают на каждом узле одно и то же — из какого хранилища образов контейнеров container runtime загружает образы и с какими учётными данными, — поэтому они никогда не управляют кластером одновременно.

### Определение способа и шагов миграции

Что нужно сделать для передачи текущей реализации модуля `registry` управления путями загрузки образов и когда — зависит от режима, в котором работает предыдущая реализация. Режим указан в параметре `settings.registry.mode` ModuleConfig `deckhouse`. Чтобы посмотреть режим, используйте команду:

```bash
d8 k get mc deckhouse -o jsonpath='{.spec.settings.registry.mode}'
```

Пустой вывод означает режим `Unmanaged`: настройки хранилища образов в этом кластере никогда не задавали. В таком кластере для миграции ничего предварительно делать не нужно — передача управления произойдёт автоматически.

Действия, необходимые для миграции с предыдущей реализации на текущую, описаны в таблице:

| Режим предыдущей реализации | Что сделать | Когда |
|---|---|---|
| `Unmanaged` | [Ничего](#миграция-из-режима-unmanaged) — передача произойдёт сама | — |
| `Direct` | [Настроить ModuleConfig `registry`](#миграция-из-режима-direct): задать `mode: Managed` и `primary.upstream` | До обновления на релиз DP с текущей реализацией|
| `Proxy` | [Перевести кластер в `Unmanaged`](#миграция-из-режима-proxy) | До обновления на релиз DP с текущей реализацией |
| `Local` | Выполнить отдельную [процедуру для изолированных кластеров](#миграция-изолированного-кластера-из-режима-local) | До обновления на релиз DP с текущей реализацией |

{% alert level="warning" %}
В будущих релизах DP планируется отказ от предыдущей реализации модуля. Рекомендуется заранее перейти на текущую реализацию. Переход возможен, начиная с релиза 1.78.
{% endalert %}

{% alert level="danger" %}
Заранее подготовьте кластер к переходу на текущую реализацию. После удаления предыдущей реализации обновление DP будет блокироваться, пока кластер работает в режиме `Proxy` или `Local` предыдущей реализации, либо в режиме `Direct` без настроенного ModuleConfig `registry`.
{% endalert %}

При использовании предыдущей реализации в режиме `Direct` переключение режима не требуется. Настройки ModuleConfig `registry` намеренно принимаются на релиз раньше: записанные до обновления, они хранятся без эффекта и срабатывают на первой итерации согласования модуля после обновления.

### Проверка реализации, используемой в кластере

Когда модуль `registry` принимает на себя управление путями загрузки, он записывает это в секрет `registry-v2-switch`. Проверьте, существует ли секрет:

```bash
d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 \
  && echo "текущая реализация" || echo "предыдущая реализация"
```

Если кластер всё ещё на предыдущей реализации, модуль сообщает причину на каждой итерации согласования и в кластере срабатывает [алерт `D8RegistryMigrationPending`](../../../reference/alerts.html#registry-d8registrymigrationpending). Чтобы посмотреть, какие действия требуются, используйте команду:

```bash
d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
```

### Миграция из режима Unmanaged

В режиме `Unmanaged` предыдущая реализация не управляет путём загрузки, поэтому предварительная подготовка к переключению на текущую реализацию не требуется — передача управления произойдёт автоматически.

Порядок миграции на новую реализацию в таком случае следующий:

1. Если кластер в предыдущей реализации переводится в `Unmanaged` из другого режима, дождитесь завершения перехода. В статусе должно быть `mode: Unmanaged` без ожидающего целевого режима.

   Для проверки используйте команду:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер на релиз с текущей реализацией модуля (или, если он уже обновлён, дождитесь следующей итерации согласования). Модуль примет управление путями загрузки образов автоматически, и поведение не изменится. Режим по умолчанию для модуля `registry` в текущей реализации — тоже `Unmanaged`, поэтому кластер продолжит загружать образы из того же хранилища, что и раньше.

1. Чтобы модуль начал управлять путями загрузки, задайте [`mode: Managed`](/modules/registry/configuration.html#parameters-mode) в ModuleConfig `registry` и укажите хранилище, из которого загружать образы. Готовая конфигурация для вашего кластера публикуется в секрете `registry-suggested-config`. Пример конфигурации — в разделе [«Настройка управления путями загрузки образов компонентов DP»](#настройка-управления-путями-загрузки-образов-компонентов-dp).

### Миграция из режима Direct

В режиме `Direct` в предыдущей реализации узлы загружают образы через внутрикластерный адрес, который обслуживает прокси предыдущей реализации. Текущая реализация обслуживает тот же адрес, поэтому миграция — это прямая передача адреса: переход через `Unmanaged` не нужен, и компоненты не перезапускаются.

Порядок миграции на новую реализацию в таком случае следующий:

1. Настройте ModuleConfig `registry` до обновления — без этой конфигурации обновление заблокировано. Для заполнения ModuleConfig `registry` возьмите значения [из секции `registry.direct`](/modules/deckhouse/configuration.html#parameters-registry-direct) ModuleConfig `deckhouse`:
   - значение `imagesRepo` из ModuleConfig `deckhouse` разделите на `host` и `path` и укажите их в соответствующих полях ModuleConfig `registry`:
   - используйте учётные данные, указанные в ModuleConfig `deckhouse`: тот же `license` (или `username`/`password`) и `ca`, если хранилище того требует

   Пример ModuleConfig `registry` для переключения:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: registry
   spec:
     enabled: true
     version: 1
     settings:
       mode: Managed
       primary:
         upstream:
           scheme: HTTPS
           host: registry.deckhouse.io
           path: /deckhouse/ee
           auth:
             license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

   Предыдущая реализация сохраняет эти настройки, но не действует по ним, поэтому до обновления в кластере ничего не меняется.

1. Обновите кластер до релиза, в котором поддерживается текущая реализация. Передача управления путями загрузки образов контейнеров произойдёт на следующей итерации согласования, и загрузка образов всё это время будет работать: Service и прокси предыдущей реализации продолжают обслуживать внутрикластерный адрес, пока агент текущей реализации не примет его на каждом узле, и только после этого удаляются.

1. Следите за ходом передачи с помощью команд ниже.

   - Проверка передачи управления путями текущей реализации:

     ```bash
     d8 k -n d8-system get secret registry-v2-switch >/dev/null 2>&1 && echo "управление передано"
     ```

   - Проверка состояния узлов хранилища образов:

     ```bash
     d8 k get registrynode -o custom-columns='NODE:.metadata.name,READY:.status.reconciled,SERVING:.status.proxyListening'
     ```

   Передача фиксируется через несколько минут после старта новой версии, агент появляется на узлах ещё через несколько минут, а объекты предыдущей реализации удаляются вскоре после этого. Загрузка образов продолжает работать на каждом из этих этапов.

### Миграция из режима Proxy

При использовании предыдущей реализации в режиме `Proxy` на каждом узле разворачивается собственный прокси со своими сертификатами. Это состояние текущая реализация перенять не может. Поэтому кластер сначала нужно перевести в `Unmanaged`, и сделать это можно только до обновления.

Порядок миграции на новую реализацию в таком случае следующий:

1. В ModuleConfig `deckhouse` задайте `registry.mode: Unmanaged`, сохранив тот же адрес хранилища и учётные данные (пример ModuleConfig `deckhouse` — в разделе [«Переключение на режим Unmanaged»](#переключение-на-режим-unmanaged)). Все узлы будут перенастроены на загрузку напрямую из внешнего хранилища, поэтому кеширование, которое работало в режиме `Proxy`, пропадёт до последнего шага.

1. Дождитесь завершения перехода — `mode: Unmanaged` без ожидающего целевого режима. Кластер, застигнутый посреди перехода, мигрировать нельзя; в статусе видно, в какой режим он ещё переключается:

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер до релиза с поддержкой текущей реализации. Передача произойдёт на следующей итерации согласования и не изменит поведения: в режиме `Unmanaged` текущая реализация тоже не управляет путём загрузки.

1. Чтобы вернуть кеширование внутри кластера, задайте [`mode: Managed`](/modules/registry/configuration.html#parameters-mode) со [`storage.cache: true`](/modules/registry/configuration.html#parameters-storage-cache) и тем же внешним хранилищем. Учтите, что кеш текущей реализации устроен иначе, чем в режиме `Proxy` предыдущей реализации: одно хранилище с репликами на master-узлах вместо прокси на каждом узле. Прежде чем включать его на кластере, где на master-узлах мало свободного места, ознакомьтесь с тем, [как кеш наполняется и очищается](/modules/registry/faq.html#кеш-растёт-что-его-чистит).

### Миграция изолированного кластера из режима Local

В режиме `Local` в предыдущей реализации у кластера нет внешнего хранилища образов — его роль выполняет хранилище внутри самого кластера. Поэтому стандартная процедура миграции здесь не работает, так как она проходит через режим `Unmanaged`, в котором каждый узел загружает образы напрямую из внешнего хранилища, а такому кластеру загружать их неоткуда.

К кластеру необходимо подключить внешнее хранилище на время миграции. Оно запускается в том же кластере, но вне неймспейсов DP. Кластер переключается на него и обновляется, а когда хранилище текущей реализации наполнится образами, временное хранилище удаляется.

{% alert level="warning" %}
Перед началом убедитесь, что на master-узлах свободно место под **четыре набора образов**: на пике миграции одновременно существуют три копии набора — хранилище `Local`, временное хранилище и наполняющееся хранилище текущей реализации, — а место под четвёртый набор нужно как запас, который узел не должен исчерпать. Подробный расчёт места и процедура проверки описаны в разделе [«Требования к диску»](/modules/registry/faq.html#требования-к-диску) документации модуля `registry`.
{% endalert %}

Порядок действий при миграции:

1. Запустите временное внешнее хранилище в собственном неймспейсе, вне управления DP. Реализация хранилища образов контейнеров может быть любой, но оно должно отдавать TLS с сертификатом, который кластер сможет проверить, и DP не должна им управлять: что бы ни происходило с объектами модуля, временное хранилище образов контейнеров должно продолжать работать.

   Кроме того, хранилище образов контейнеров должно быть доступно **по одному и тому же адресу из двух мест**: на шаге 3 из него загружают образы узлы, а на шаге 6 его читает syncer модуля `registry`, работающий в поде. Сертификат должен покрывать выбранный адрес. Варианты, проверенные на одном кластере:

   | Адрес | С узла | Из пода |
   |---|---|---|
   | Порт `hostNetwork` на IP самого узла (`<NODE_IP>:5000`) | Работает | Работает |
   | Имя Service (`<SERVICE>.<NAMESPACE>.svc:<PORT>`) | Не разрешается | Работает |
   | NodePort на IP узла | Работает | Не работает: `operation not permitted` |

   Используйте первый вариант: запустите хранилище образов контейнеров с `hostNetwork: true` на одном узле, обращайтесь к нему по IP этого узла и добавьте этот IP в SAN сертификата — тогда вся миграция пройдёт на одном адресе. Имя Service выглядит аккуратнее, но перестаёт работать на шаге 3, потому что container runtime узла не разрешает имена через кластерный DNS.
1. Загрузите набор образов текущей версии во временное хранилище: `d8 mirror pull` на машине с доступом в интернет, затем `d8 mirror push` во временное хранилище.
1. В ModuleConfig `deckhouse` задайте `registry.mode: Unmanaged` с адресом, сертификатом CA и учётными данными временного хранилища. Данные хранилища `Local` при этом остаются на месте, в каталоге `/opt/deckhouse/registry` на master-узлах.
1. Убедитесь, что переход завершён и образы действительно загружаются из временного хранилища (`mode: Unmanaged` без ожидающего целевого режима в секрете `registry-state`):

   ```bash
   d8 k -n d8-system get secret registry-state -o jsonpath='{.data.state}' | base64 -d | head
   ```

1. Обновите кластер на релиз с текущей реализацией. Передача управления произойдёт на следующей итерации согласования, и всё это время кластер продолжит загружать образы из временного хранилища.
1. Включите текущую реализацию: задайте [`mode: Managed`](/modules/registry/configuration.html#parameters-mode) с [`storage.cache: true`](/modules/registry/configuration.html#parameters-storage-cache), укажите временное хранилище в качестве [`primary.upstream`](/modules/registry/configuration.html#parameters-primary-upstream) — **и добавьте [`storage.source`](/modules/registry/latest/configuration.html#parameters-storage-source) в том же изменении** (без него следующий шаг будет отклонён). Хранилище текущей реализации будет доступно по тому же пути на хосте, который использовал `Local`, поэтому уже имеющиеся на дисках образы не будут скачиваться заново.
1. Дождитесь, пока хранилище сообщит, что содержит весь набор образов (`phase: Ready` и `safeToDropUpstream: true`), и уберите `primary.upstream` из ModuleConfig `registry`. После этого кластер снова изолирован — теперь уже на текущей реализации:

   ```bash
   d8 k get registrystorage registry -o jsonpath='{.status.phase} {.status.safeToDropUpstream}{"\n"}'
   ```

1. Удалите временное хранилище и освободите занятый им диск.

## Миграция на формат управления настройками хранилища образов с использованием модуля registry

{% alert level="info" %}
Эта процедура переводит кластер с устаревшего формата (без модуля `registry`, настроенного только через `InitConfiguration`) на **предыдущую реализацию** модуля. Если ваш кластер уже использует предыдущую реализацию и вы хотите перейти сразу на текущую, используйте раздел [«Миграция с предыдущей реализации на текущую»](#миграция-с-предыдущей-реализации-на-текущую).
{% endalert %}

Во время миграции для containerd v1 будет выполнен переход на новую схему конфигурации хранилища образов.
containerd v2 использует новую схему по умолчанию. Подробнее можно ознакомиться в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry).

Чтобы посмотреть тип container runtime, используемый на узлах кластера (в NodeGroup) по умолчанию, используйте команду:

```shell
d8 system edit cluster-configuration
```

Тип container runtime указывается в параметре `defaultCRI`.

Если в качестве container runtime используется containerd v2, используйте для миграции инструкцию [для containerd v2](#для-containerd-v2).

Если в качестве container runtime используется containerd v1, можете выбрать один из способов миграции на формат управления настройками хранилища образов с использованием модуля `registry`:

- Без переключения на containerd v2. В этом случае используйте инструкцию [для containerd v1](#для-containerd-v1).
- С переключением на containerd v2. В этом случае:
  
  1. Убедитесь в [возможности переключения на containerd v2](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/migrating.html) и при наличии такой возможности выполните переключение.
  1. После переключения используйте инструкцию [для containerd v2](#для-containerd-v2).

### Для containerd v2

1. Выполните переключение на использование модуля `registry`. Для этого укажите в ModuleConfig `deckhouse` параметры `Unmanaged` режима. Если используется хранилище образов компонентов DP, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Для просмотра текущих настроек хранилища образов используйте команду:

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации):

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

Если требуется добавить конфигурации для дополнительного хранилища образов, воспользуйтесь разделом [«Новый способ добавления конфигурации для дополнительного registry»](/products/kubernetes-platform/documentation/v1/admin/configuration/platform-scaling/node/node-customization.html#новый-способ-добавления-конфигурации-для-дополнительного-registry).

### Для containerd v1

{% alert level="danger" %}

- Во время переключения containerd v1 сервис будет перезапущен.
- Во время переключения containerd v1 будет переведен на новую схему конфигурации хранилища образов компонентов DP.
- Во время переключения [пользовательские конфигурации registry](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Убедитесь, что на узлах с containerd v1 отсутствуют [пользовательские конфигурации хранилища образов](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry), расположенные в директории `/etc/containerd/conf.d`.

1. Если конфигурации присутствуют, необходимо выполнить миграцию на новый формат конфигурации хранилища образов в containerd. Для этого добавьте новые конфигурации в директорию `/etc/containerd/registry.d`. Эти конфигурации вступят в силу после переключения на модуль `registry`. Для добавления конфигураций подготовьте NodeGroupConfiguration, подробнее — в разделе [с описанием способов конфигурации](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry). Пример:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth.sh
   spec:
     # Шаг может быть любой, поскольку перезапуск сервиса containerd не требуется.
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
   # Для HTTPS.
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ private.registry.example/registry/path:tag

   # Для HTTP.
   ctr -n k8s.io images pull --hosts-dir=/etc/containerd/registry.d/ --plain-http private.registry.example/registry/path:tag
   ```

1. Выполните переключение на использование модуля `registry`. Для этого, укажите в ModuleConfig `deckhouse` параметры `Unmanaged` режима. Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/latest/configuration.html) для корректной настройки.

   Для просмотра текущих настроек хранилища образов используйте команду:

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
           license: <LICENSE_KEY> # Замените на ваш лицензионный ключ.
   ```

1. После применения дождитесь в [статусе переключения](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации) сообщения:

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

   Это сообщение означает, что на узлах имеются старые конфигурации хранилища образов, расположенные в директории `/etc/containerd/conf.d`, и в данный момент переключение на новую конфигурацию containerd заблокировано. Для того чтобы разрешить переключение, необходимо удалить старые конфигурационные файлы.

1. Удалите старые конфигурационные файлы, чтобы разрешить переключение на модуль `registry`. Для этого создайте [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration). Пример манифеста NodeGroupConfiguration:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: NodeGroupConfiguration
   metadata:
     name: containerd-additional-config-auth-delete.sh
   spec:
     # Шаг должен выполниться до '032_configure_containerd.sh'.
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
  
1. После удаления старых конфигураций убедитесь, что переключение продолжается. Пример [статуса переключения](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации):

   ```yaml
   conditions:
   # ...
   - lastTransitionTime: "2025-08-13T16:42:09Z"
     message: ""
     reason: ""
     status: "True"
     type: ContainerdConfigPreflightReady
   ```

1. Дождитесь завершения переключения. Пример [статуса переключения](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации):

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

## Миграция на устаревший формат управления настройками хранилища образов компонентов DP (без модуля registry)

{% alert level="danger" %}

- Это устаревший (deprecated) формат управления хранилищем образов компонентов DP.
- Во время переключения containerd v1 будет перезапущен.
- Во время переключения containerd v1 будет переведен на старую схему конфигурации хранилища образов.
- Во время переключения, [пользовательские конфигурации хранилища образов](/modules/node-manager/latest/faq.html#как-добавить-конфигурацию-для-дополнительного-registry) для containerd v1 будут временно недоступны.
{% endalert %}

1. Переведите модуль `registry` в конфигурируемый режим `Unmanaged`. Если используется хранилище образов, отличное от `registry.deckhouse.ru`, ознакомьтесь с конфигурацией модуля [`deckhouse`](/modules/deckhouse/configuration.html) для корректной настройки.

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

1. Проверьте статус переключения, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации). Пример вывода:

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

1. Переведите хранилище образов компонентов DP в неконфигурируемый режим `Unmanaged` (без модуля `registry`). Пример конфигурации:

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

1. Проверьте статус переключения, используя [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации). Пример вывода:

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
     # Для добавления файла перед шагом '032_configure_containerd.sh'.
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
                 auth = "<BASE64_AUTH_STRING>"
       EOF
   ```

1. Удалите секрет `registry-bashible-config`. Во время удаления containerd v1 переключится на старый формат конфигурации containerd:

   ```bash
   d8 k -n d8-system delete secret registry-bashible-config
   ```

1. После удаления дождитесь завершения переключения. Для отслеживания используйте [инструкцию](#просмотр-статуса-переключения-режима-хранилища-образов-предыдущей-реализации). Пример вывода:

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

1. Если используется containerd v1, примените заготовленные на этапе ранее NodeGroupConfiguration с пользовательскими конфигурациями хранилища образов.

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

## Просмотр статуса переключения режима хранилища образов предыдущей реализации

Статус переключения режима хранилища образов компонентов DP в предыдущей реализации можно получить с помощью следующей команды:

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
| `ContainerdConfigPreflightReady`  | Состояние проверки конфигурации containerd. Проверяется, что на узлах отсутствуют пользовательские auth-конфигурации containerd                                                                                             |
| `TransitionContainerdConfigReady` | Состояние подготовки конфигурации containerd в новый режим. Проверяется, что конфигурация containerd успешно подготовлена и содержит одновременно конфигурации нового и старого режима                                      |
| `FinalContainerdConfigReady`      | Состояние завершения переключения containerd в новый режим. Проверяется, что конфигурация containerd успешно применена и содержит конфигурацию нового режима                                                                |
| `DeckhouseRegistrySwitchReady`    | Состояние переключения DP и его компонентов на использование нового хранилища образов контейнеров. Значение `True` указывает, что DP успешно переключился на сконфигурированное хранилище образов и готов к работе                          |
| `InClusterProxyReady`             | Состояние готовности In-Cluster Proxy. Проверяется, что In-Cluster Proxy успешно запущен и работает                                                                                                                         |
| `CleanupInClusterProxy`           | Состояние очистки In-Cluster Proxy, если прокси не нужен для работы желаемого режима. Проверяется, что все ресурсы, связанные с In-Cluster Proxy, успешно удалены                                                           |
| `NodeServicesReady`               | Состояние готовности Node Services Manager и Static-Pod хранилища образов. Проверяется, что Node Services Manager успешно запущен и работает, и что Static-Pod хранилища образов был успешно развёрнут с помощью Node Services Manager        |
| `CleanupNodeServices`             | Состояние очистки Node Services Manager и Static-Pod хранилища образов, если компоненты не нужны для работы желаемого режима. Проверяется, что все ресурсы, связанные с Node Services Manager и Static-Pod хранилища образов, успешно удалены |
| `RegistryContainsRequiredImages`  | Состояние проверки хранилища образов на наличие необходимых образов                                                                                                                                                                   |
| `Ready`                           | Общее состояние готовности хранилища образов к работе в указанном режиме. Проверяется, что все предыдущие условия выполнены и модуль готов к работе                                                                                  |
