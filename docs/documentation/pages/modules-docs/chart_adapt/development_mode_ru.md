---
title: "Режим разработчика"
permalink: ru/modules-docs/chart-adapt/development-mode/
lang: ru
---

## Режим "разработчика"

При разработке модулей бывает необходимо скачивать и деплоить модуль минуя каналы обновлений.
Для этих целей нужно использовать ресурс ModulePullOverride.

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
**metadata.name** - Имя модуля. Должно соответствовать имени модуля в ModuleSource (поле `.status.modules.[].name`).

**spec.imageTag** - тэг образа контейнера. Может быть любым. Например, ~pr333~, ~my-branch~.

**spec.source** - имя ModuleSource. Из данного ModuleSource берутся данные для авторизации в registry.

**spec.scanInterval** (не обязательно) - интервал проверки образа в registry. По-умолчанию: 15 секунд.

Для принудительного обновления можно задать большой интервал и использовать аннотацию `renew=""`.

Пример:

```sh
kubectl annotate mop <name> renew=""
```

## Принцип действия

При создании данного ресурса, указанный модуль будет игнорировать ModuleUpdatePolicy и не будет скачивать и создавать объекты ModuleRelease.
Вместо этого, модуль будет скачиваться при каждом изменении image digest и сразу применяться в кластере.
При этом, в статусе объекта ModuleSource данный модуль получит признак `overridden: true`, который указывает на то, что у данного модуля был создан и используется ресурс ModulePullOverride.
При удалении ModulePullOverride модуль продолжит функционировать дальше, но если для него существует политика ModuleUpdatePolicy, то будут скачаны новые релизы (если есть), которые заменят текущую "версию разработчика".

### Пример

1. Существует ModuleSource, в котором есть два модуля:

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

   В данном ModuleSource существуют два модуля `echo` и `hello-world` для обоих определена политика обновления, они скачиваются и устанавливаются в Deckhouse.

2. Создадим ModulePullOverride для модуля echo

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     source: test
   ```

   Данный ресурс будет проверять image tag `registry.example.com/deckhouse/modules/echo:main-patch-03354` (<ms:spec.registry.repo>/<mpo:metadata.name>:<mpo:spec.imageTag>).

3. При каждом обновлении статус данного ресурса будет меняться:

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

   - **imageDigest** — digest образа контейнеров, который был скачан.
   - **updatedAt** — когда образ был скачан в последний раз.

4. При этом ModuleSource приобретет вид:

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
