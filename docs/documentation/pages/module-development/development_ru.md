---
title: "Разработка и отладка"
permalink: ru/module-development/troubleshooting/
lang: ru
---

{% raw %}

Команда Deckhouse Kubernetes Platform (DKP) всегда готова проконсультировать. Вы можете обратиться к нам в канале `#tech-deckhouse-modules` внутреннего слака Flant.

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

Требования к параметрам ресурса:
* Имя модуля **metadata.name** должно соответствовать имени модуля в *ModuleSource* (параметр `.status.modules.[].name`).

* Тег образа контейнера **spec.imageTag** может быть любым. Например, ~pr333~, ~my-branch~.

* Параметр *ModuleSource* **spec.source** выдает данные для авторизации в registry.

Необязательный интервал времени **spec.scanInterval** устанавливает интервал для проверки образов в registry. По умолчанию, задан интервал в 15 секунд.

Для принудительного обновления можно задать больший интервал, а также использовать аннотацию `renew=""`.

Пример команды:

```sh
kubectl annotate mop <name> renew=""
```

## Принцип действия

При разработке этого ресурса, указанный модуль не будет учитывать *ModuleUpdatePolicy*, а также не будет загружать и создавать объекты *ModuleRelease*.

Вместо этого, модуль будет загружаться при каждом изменении параметра `imageDigest` и будет применяться в кластере.
При этом, в статусе объекта *ModuleSource* этот модуль получит признак `overridden: true`, который укажет на то, что используется ресурс *ModulePullOverride*.

После удаления *ModulePullOverride*, модуль продолжит функционировать, но если для него применена политика *ModuleUpdatePolicy*, то при наличии загрузятся новые релизы, которые заменят текущую "версию разработчика".

### Пример

1. В *ModuleSource* присутствуют два модуля `echo` и `hello-world`. Для них определена политика обновления, они загружаются и устанавливаются в DKP:

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

1. Создайте *ModulePullOverride* для модуля `echo`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     source: test
   ```

    Этот ресурс будет проверять тег образа `registry.example.com/deckhouse/modules/echo:main-patch-03354` (ms:spec.registry.repo/mpo:metadata.name:mpo:spec.imageTag).

1. При каждом обновлении статус этого ресурса будет меняться:

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
    - **imageDigest** – уникальный идентификатор образа контейнера, который был загружен.
    - **lastUpdated** – время последней загрузки образа.

1. При этом *ModuleSource* приобретет вид:

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

{% endraw %}
