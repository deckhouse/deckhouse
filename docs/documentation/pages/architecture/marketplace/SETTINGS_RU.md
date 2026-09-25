---
title: Настройки приложения
permalink: ru/architecture/marketplace/settings.html
description: "Схемы настроек и values пакета Application: валидация, значения по умолчанию, правила CEL, ресурсы, доступные по гранту, неизменяемые поля и расширения формы веб-интерфейса."
lang: ru
search: application settings, settings.yaml, x-deckhouse-validations, x-deckhouse-immutable, x-deckhouse-ui, настройки приложения, схема настроек, валидация настроек
---

Пользователь настраивает экземпляр приложения в поле `spec.settings` ресурса [Application](../../reference/api/cr.html#application). Пакет описывает допустимые настройки OpenAPI-схемой. Deckhouse Platform (DP) проверяет настройки по этой схеме, подставляет значения по умолчанию и передаёт результат в шаблоны и хуки. Веб-интерфейс DP строит по этой же схеме форму настроек приложения.

## Файлы схем

Каталог `openapi/` пакета содержит две схемы:

- `settings.yaml` — схема `Application.spec.settings`, то есть настроек, которые может менять пользователь. Для обратной совместимости поддерживается прежнее имя файла `config-values.yaml`: DP читает его, только если файла `settings.yaml` нет.
- `values.yaml` — схема полного набора значений, доступных шаблонам как `.Values`, включая внутренние значения, которые задают хуки.
- `doc-ru-settings.yaml` — описания настроек на русском языке. Команда `d8 package verify` требует файл `doc-ru-<NAME>.yaml` для каждого файла схемы, кроме `values.yaml`.

Настройки находятся в корне values, поэтому схема values тоже должна их описывать. Чтобы включить все настройки в схему values без дублирования, добавьте в `openapi/values.yaml` расширение `x-extend`. Без него values не проходят валидацию, когда их меняет хук.

Пример `openapi/values.yaml`:

```yaml
x-extend:
  schema: settings.yaml
type: object
properties:
  internal:
    type: object
    default: {}
    properties:
      adminPassword:
        type: string
```

При сканировании репозитория DP публикует обе схемы в поле `status.packageSchemas` ресурса [ApplicationPackageVersion](../../reference/api/cr.html#applicationpackageversion). Опубликованные схемы используют веб-интерфейс и validating-вебхук Application. Публикуются только стандартные ключевые слова OpenAPI и расширения `x-deckhouse-*`, описанные на этой странице. Остальные расширения `x-*` удаляются из опубликованной схемы.

## Как DP формирует values

DP формирует values из следующих источников в указанном порядке:

1. Значения по умолчанию из `openapi/settings.yaml`.
2. Значения по умолчанию проекта для полей с расширением [`x-deckhouse-grantable-resource`](#подстановка-значения-ресурса-кластера-доступного-по-гранту-x-deckhouse-grantable-resource).
3. Пользовательские настройки из `Application.spec.settings`. Они переопределяют значения из предыдущих источников.
4. Значения по умолчанию из `openapi/values.yaml` для полей, которые всё ещё не заданы.
5. Значения, заданные хуками.

Шаблоны получают результат как `.Values`. Итоговые настройки (источники 1–3) также доступны шаблонам как `.Application.Settings`. После успешного применения настроек DP сохраняет итоговые настройки в поле `status.lastAppliedConfiguration` ресурса Application.

## Валидация

### Незадекларированные поля

DP добавляет `additionalProperties: false` к каждому объекту обеих схем, в котором `additionalProperties` не задано явно. Поэтому настройка или значение, не описанные в схеме, отклоняются.

Чтобы разрешить произвольные ключи в объекте, например, в словаре меток, задайте `additionalProperties` явно:

```yaml
type: object
properties:
  podLabels:
    type: object
    additionalProperties:
      type: string
```

{% alert level="warning" %}
Поля, описанные только в ветках `oneOf`, `anyOf` или `allOf`, тоже отклоняются. Описывайте все поля объекта в его `properties`, а ветки используйте только для ограничения сочетаний значений.
{% endalert %}

### Когда проверяются настройки

| Когда | Что проверяет DP | Результат при ошибке |
|---|---|---|
| Application создаётся или изменяется | Схема настроек, опубликованная в ApplicationPackageVersion, включая правила CEL без `oldSelf`, и поля с `x-deckhouse-immutable` | Запрос отклоняется |
| Изменяется Application установленного приложения | Дополнительно — проверки установленной версии: правила CEL с `oldSelf`, доступность ресурсов по грантам и [хук валидации настроек](hooks.html#хук-валидации-настроек) | Запрос отклоняется |
| DP применяет настройки | То же, что в предыдущей строке | Настройки не применяются, ошибка отражается в условиях Application |
| Хук меняет values | Схема `openapi/values.yaml`, включая правила CEL | Хук завершается с ошибкой |

### Правила CEL (x-deckhouse-validations)

Обе схемы поддерживают расширение `x-deckhouse-validations` с правилами на языке Common Expression Language (CEL). Синтаксис и семантика такие же, как в [схемах модулей](../module-development/structure/#валидации-x-deckhouse-validations-cel): правило состоит из `expression` и `message`, значение текущего уровня доступно как `self`, а предыдущее значение — как `oldSelf`.

Пример `openapi/settings.yaml` с правилом, проверяющим два поля вместе:

```yaml
type: object
properties:
  replicas:
    type: integer
    default: 1
  maxReplicas:
    type: integer
    default: 3
x-deckhouse-validations:
  - expression: "self.replicas <= self.maxReplicas"
    message: "replicas must not exceed maxReplicas"
```

Правила с `oldSelf` сравнивают новые настройки с ранее применёнными. Такие правила проверяются только при изменении настроек установленного приложения, а при установке пропускаются. Чтобы запретить изменение поля после установки, используйте [`x-deckhouse-immutable`](#неизменяемые-поля-x-deckhouse-immutable) — это расширение проверяется при каждом обновлении.

### Подстановка значения ресурса кластера, доступного по гранту (x-deckhouse-grantable-resource)

Поле `settings` типа `string` можно связать с cluster-wide-ресурсом, которым управляет
[`multitenancy-manager`](/modules/multitenancy-manager/) и который доступен проекту по гранту (например, StorageClass).

Когда поле связано и пользователь оставляет его пустым, в `values` подставляется имя ресурса, заданное для проекта как значение по умолчанию. При указании собственного значения оно проверяется по списку ресурсов, доступных проекту. Значение, которого в этом списке нет, отклоняется.

Чтобы связать поле с cluster-wide-ресурсом, добавьте к полю расширение `x-deckhouse-grantable-resource` и укажите имя ресурса, доступного проекту по гранту
(AvailableClusterResource или GrantableClusterResourceDefinition), например `storageclasses`.

{% alert level="info" %}
Группа, версия и тип ресурса (GVK) задаются в гранте. Указывать их в `openapi/settings.yaml` не нужно.
{% endalert %}

Пример `openapi/settings.yaml` с `x-deckhouse-grantable-resource`:

```yaml
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-grantable-resource: storageclasses
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-grantable-resource: postgresclasses
```

Поведение:

- Значение по умолчанию определяется для каждого проекта из AvailableClusterResource в неймспейсе приложения, поэтому разные проекты могут получать разные значения по умолчанию.
- Явно заданное пользователем значение имеет более высокий приоритет, чем значение по умолчанию для проекта.
- Если отсутствует определение кастомного ресурса (CRD), для проекта не создан каталог или в каталоге не задано значение по умолчанию, поле остаётся без изменений. Значение не подставляется и не проверяется.

### Неизменяемые поля (x-deckhouse-immutable)

Некоторые настройки не следует менять после применения конфигурации приложения: например, смена `storageClass` после создания томов либо ни на что не влияет, либо может привести к ошибкам в работе приложения. Чтобы запретить изменение значения такого поля после успешного применения конфигурации приложения, добавьте к нему расширение `x-deckhouse-immutable: true`.

Пример `openapi/settings.yaml` с `x-deckhouse-immutable`:

```yaml
type: object
properties:
  storageClass:
    type: string
    default: default
    x-deckhouse-immutable: true
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      volumeSize:
        type: string
```

Поведение:

- Расширение действует только при значении `true`. Любое другое значение расширения игнорируется.
- Если `x-deckhouse-immutable` добавлено к объекту, неизменяемым становится весь объект. После первого успешного применения конфигурации приложения изменение любого вложенного поля будет отклонено, даже если для него отдельно не указано `x-deckhouse-immutable`. Указывайте расширение на уровне объекта, только если необходимо запретить изменение всего объекта. Чтобы запретить изменение только одного поля, добавьте `x-deckhouse-immutable` непосредственно к нему, как к `postgres.storageClass` в примере выше. В этом случае ограничение не распространяется на `postgres.volumeSize`, и его значение можно изменять.
- Обновление, меняющее зафиксированное значение, отклоняется validating-вебхуком с указанием имени поля.
- В веб-интерфейсе значение поля с расширением можно задать при установке приложения. В форме редактирования установленного приложения поле доступно только для чтения.
- Сравнение выполняется с фактически применённой конфигурацией после подстановки значений по умолчанию из схемы. Поэтому поле с расширением можно удалить из манифеста только в том случае, если его `default` совпадает с уже применённым значением. Если значения по умолчанию нет или оно отличается, изменение отклоняется.

{% alert level="info" %}
Если удалить из манифеста весь объект, значения по умолчанию для его вложенных полей не подставляются. Поэтому удаление объекта с полями, использующими `x-deckhouse-immutable`, может привести к потере зафиксированных значений и будет отклонено.
{% endalert %}

- Метка не распространяется на элементы массивов и записи карт, которые обновление добавляет: у нового элемента нет предыдущего значения, относительно которого его можно фиксировать.

## Форма в веб-интерфейсе

Веб-интерфейс строит форму настроек приложения по `openapi/settings.yaml`. Отображением полей управляют следующие расширения:

| Расширение | Значение | Действие |
|---|---|---|
| `x-deckhouse-ui-order` | Целое число | Порядок отображения поля. Поля с меньшим значением показываются раньше |
| `x-deckhouse-ui-group` | Строка | Поле показывается в группе верхнего уровня с этим именем. Путь поля в настройках не меняется |
| `x-deckhouse-ui-advanced` | `true` | Поле скрыто за переключателем расширенных настроек. Расширение учитывается только для полей верхнего уровня |
| `x-deckhouse-ui-validation-message` | Строка | Сообщение, которое показывается под полем вместо ошибки валидации по схеме, например, вместо регулярного выражения, которому не соответствует значение |
| `x-deckhouse-ui-resource-name` | Объект с полями `apiVersion` и `kind` и необязательным полем `labelSelector` | Для поля типа `string`: выпадающий список с именами ресурсов указанного типа из неймспейса приложения. `labelSelector` — стандартный селектор меток Kubernetes, сужающий список |
| `x-deckhouse-enum-switch-settings` | Список, задаётся в ветке `oneOf` | Диалоги подтверждения перед переключением на другую ветку. См. [следующий раздел](#подтверждение-переключения-режима-x-deckhouse-enum-switch-settings) |

Пример `openapi/settings.yaml` с расширениями формы:

```yaml
type: object
properties:
  replicas:
    type: integer
    default: 1
    x-deckhouse-ui-order: 10
  domain:
    type: string
    pattern: '^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$'
    x-deckhouse-ui-order: 20
    x-deckhouse-ui-validation-message: "Enter a domain name, for example, app.example.com."
  tlsSecretName:
    type: string
    x-deckhouse-ui-group: TLS
    x-deckhouse-ui-resource-name:
      apiVersion: v1
      kind: Secret
      labelSelector:
        matchLabels:
          example.com/tls: "true"
  logLevel:
    type: string
    enum: [Info, Debug]
    default: Info
    x-deckhouse-ui-advanced: true
```

### Подтверждение переключения режима (x-deckhouse-enum-switch-settings)

Некоторые переключения настроек нельзя отменить без потери данных, например, перевод хранилища приложения с внутрикластерного на внешнее. Чтобы веб-интерфейс запрашивал подтверждение перед таким переключением, добавьте расширение `x-deckhouse-enum-switch-settings` в ветку `oneOf`. Подтверждение показывается, когда дискриминатор ветки (поле, значение `enum` которого выбирает ветку) меняется со значения этой ветки на одно из целевых значений.

Каждый элемент списка содержит следующие поля:

- `to` — целевые значения дискриминатора, к которым относится подтверждение. Значения сравниваются как строки, поэтому булевы дискриминаторы указываются как `"true"` и `"false"`.
- `impact` — серьёзность переключения: `info` (по умолчанию), `warning` или `destructive`. Переключение `destructive` веб-интерфейс применяет только после явного подтверждения и записывает его в аннотацию `applications.deckhouse.io/allow-destructive-settings-switch` ресурса Application.
- `messages` — текст диалога в формате Markdown по кодам языков (`en`, `ru`).

Пример `openapi/settings.yaml` с `x-deckhouse-enum-switch-settings`:

```yaml
type: object
properties:
  storage:
    type: object
    default: {}
    properties:
      mode:
        type: string
        enum: [Internal, External]
        default: Internal
      size:
        type: string
      url:
        type: string
    oneOf:
      - properties:
          mode:
            enum: [Internal]
        x-deckhouse-enum-switch-settings:
          - to: [External]
            impact: destructive
            messages:
              en: "Switching to external storage **deletes the data** stored in the cluster."
              ru: "Переключение на внешнее хранилище **удалит данные**, хранящиеся в кластере."
      - properties:
          mode:
            enum: [External]
        required: [url]
```
