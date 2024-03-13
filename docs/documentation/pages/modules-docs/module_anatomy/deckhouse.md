---
title: "В кластере Deckhouse"
permalink: en/modules-docs/module-anatomy/deckhouse/
---

В этой главе мы разберем как выложить собранный модуль в кластер Deckhouse и где посмотреть результат.

## ModuleSource

Для того чтобы выложить модули в кластер, необходимо создать ресурс `ModuleSource`. Ресурс `ModuleSource` определяет источник (container registry), откуда Deckhouse будет загружать модули.

Пример `ModuleSource`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: example
spec:
  registry:
    repo: registry.example.io/modules-source
    dockerCfg: <base64 encoded credentials>
```

После создания ресурса можно посмотреть, какие модули находятся в подключенном ModuleSource и есть ли ошибки.

```sh
kubectl get ms
```

Пример вывода:

```
NAME        COUNT   SYNC   MSG
example     2       16s
```

```sh
kubectl get ms example -o yaml
```

Пример вывода:

```yaml
...
status:
  modules:
  - module-1
  - module-2
  message: ""
  moduleErrors: []
  modulesCount: 2
  syncTime: "2023-08-13T22:12:00.033854109Z"
```

> Deckhouse обновляет список модулей и версий для них раз в 3 минуты.

На данном этапе модули еще не установлены - не хватает ModuleUpdatePolicy, создаем!

## ModuleUpdatePolicy

Ресурс `ModuleUpdatePolicy` используется для определения списка модулей, которые необходимо установить, политики их обновления (Manual/Auto/Auto with maintenance windows), и релизного канала. Если не указать `ModuleUpdatePolicy` для `ModuleSource`, то будут использоваться настройки обновлений и релизный канал, установленные для Deckhouse.

Также, можно установить `mode: Ignore` для того, чтобы не скачивать модули.

Пример `ModuleUpdatePolicy`, стоит обратить внимание на labelSelector, который сужает действие политики до модуля `module-1`, поставляемого из `example` `ModuleSource`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleUpdatePolicy
metadata:
  name: example-update-policy
spec:
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: example
        module: module-1
  releaseChannel: alpha
  update:
    mode: Manual    
```

## ModuleRelease

По аналогии с `DeckhouseRelease`, у модулей тоже есть релизы.

> Важный момент! Модули Deckhouse из источника модулей имеют свой цикл обновлений в отличии от самого Deckhouse. Чтобы исправить баг в модуле нет необходимости ждать нового релиза Deckhouse.

Ресурсы `ModuleRelease` создает сам Deckhouse на основании того что лежит в container registry.

```sh
kubectl get mr
```

Пример вывода:

```
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for manual approval
```

Т.к. в `ModuleSource` был указан канал обновления `alpha`, то были скачаны самые свежие версии модулей. По скольку режим обновления нашей политики выставлен в Manual, нам необходимо в ручную подтвердить необходимость установки новой версии. Для этого необходимо добавить аннотацию на указанный релиз:

```sh
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```

В случае использования режима обновления Auto - релиз будет установлен автоматический в ближайшее релизное окно (или по факту скачивания, если окна не указаны).

## Module

После успешного скачивания и установки, можно проверить, появились ли модули в доступе для работы. Для этого получим список всех модулей Deckhouse:

```sh
kubectl get modules | grep example
```

```
NAME                                  WEIGHT   STATE      SOURCE
module-1                              900      Disabled   example
module-2                              900      Disabled   example
```

Отлично, модули доступны.

## ModuleConfig

Теперь можно работать с модулями так же, как-будто это обычные модули Deckhouse. Создадим `ModuleConfig`, чтобы включить `module-1`.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-1
spec:
  enabled: true
  settings: {}
  version: 1
```

Если с модулем возникнут проблемы, то Deckhouse запишет ошибку в `ModuleConfig`. Проверим, что все в порядке:

```sh
kubectl get moduleconfig module-1
```

Пример вывода:

```
NAME              STATE     VERSION   AGE   TYPE                  STATUS
module-1          Enabled   1         3m    example
```
