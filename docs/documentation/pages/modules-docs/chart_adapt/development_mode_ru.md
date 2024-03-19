---
title: "Режим разработчика"
permalink: ru/modules-docs/chart-adapt/development-mode/
lang: ru
---

## Режим разработчика

При разработке модулей может возникнуть необходимость загрузить и развернуть модуль в обход каналов обновления. Для этого используется ресурс *ModulePullOverride*.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModulePullOverride
metadata:
  name: <module-name>
spec:
  imageTag: <tag of the module image>
  scanInterval: <image digest check interval. Default: 15s>
  source: <ModuleSource ref>
```

Параметры ресурса:
* Имя модуля **metadata.name**. Должно соответствовать имени модуля в *ModuleSource* (параметр `.status.modules.[].name`).

* Тэг образа контейнера **spec.imageTag**. Может быть любым. Например, ~pr333~, ~my-branch~.

* Имя *ModuleSource* **spec.source** . Выдает данные для авторизации в registry.

Не обязательный интервал **spec.scanInterval**. Проверяет образы в registry. По-умолчанию задан интервал в 15 секунд.

Для принудительного обновления можно задать больший интервал, а также использовать аннотацию `renew=""`.

Пример:

```sh
kubectl annotate mop <name> renew=""
```

## Принцип действия

При разработке этого ресурса, указанный модуль не будет учитывать *ModuleUpdatePolicy* и не будет загружать и создавать объекты *ModuleRelease*.
Вместо этого, модуль будет загружаться при каждом изменении image digest и будет применяться в кластере.
При этом, в статусе объекта *ModuleSource* этот модуль получит признак `overridden: true`, который указывает на то, что используется ресурс *ModulePullOverride*.
После удаления *ModulePullOverride*, модуль продолжит функционировать, но если для него применена политика *ModuleUpdatePolicy*, то загрузятся новые релизы (при наличии), которые заменят текущую "версию разработчика".

### Пример

В примере представлен *ModuleSource*, содержащий два модуля:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: test
   spec:
     registry:
       ca: ""
       dockerCfg: someBase64String==
       repo: registry.example.com/deckhouse/modules
       scheme: HTTPS
   status:
     modules:
     - name: echo
       policy: test-alpha
     - name: hello-world
       policy: test-alpha
     modulesCount: 2
   ```

В *ModuleSource* присутствуют два модуля `echo` и `hello-world`. Для них определена политика обновления, они загружаются и устанавливаются в Deckhouse Kubernetes Platform.

1. Создайте *ModulePullOverride* для модуля `echo`.

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     source: test
   ```

 Этот ресурс будет проверять tag образа `registry.example.com/deckhouse/modules/echo:main-patch-03354` (<ms:spec.registry.repo>/<mpo:metadata.name>:<mpo:spec.imageTag>).

 При каждом обновлении статус этого ресурса будет меняться:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     scanInterval: 15s
     source: test
   status:
     imageDigest: sha256:ed958cc2156e3cc363f1932ca6ca2c7f8ae1b09ffc1ce1eb4f12478aed1befbc
     message: ""
     updatedAt: "2023-12-07T08:41:21Z"
   ```

   где:

- **imageDigest** - уникальный идентификатор образа контейнера, который был загружен.
- **lastUpdated** - время последней загрузки образа.

При этом *ModuleSource* приобретет вид:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleSource
   metadata:
     name: test
   spec:
     registry:
       ca: ""
       dockerCfg: someBase64String==
       repo: registry.example.com/deckhouse/modules
       scheme: HTTPS
   status:
     modules:
     - name: echo
       overridden: true
     - name: hello-world
       policy: test-alpha
     modulesCount: 2
   ```
