---
title: "Разработка и отладка модуля"
permalink: ru/module-development/development/
lang: ru
---

{% raw %}

При разработке модулей может возникнуть необходимость загрузить и развернуть модуль в обход каналов обновления. Для этого используется ресурс [ModulePullOverride](../../cr.html#modulepulloverride).

Пример ModulePullOverride:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ModulePullOverride
metadata:
  name: <module-name>
spec:
  imageTag: <tag of the module image>
  scanInterval: <image digest check interval. Default: 15s>
```

Требования к параметрам ресурса:
* Имя модуля (`metadata.name`) должно соответствовать имени модуля в ModuleSource (`.status.modules.[].name`).

* Тег образа контейнера (`spec.imageTag`) может быть любым. Например, `pr333`, `my-branch`.

Необязательный интервал времени `spec.scanInterval` устанавливает интервал для проверки образов в registry. По умолчанию задан интервал в 15 секунд. Для принудительного обновления можно изменить интервал, либо установить на ModulePullOverride аннотацию `renew=""`.

Результат применения ModulePullOverride можно увидеть в сообщении (колонка `MESSAGE`) при получении информации об ModulePullOverride. Значение `Ready` означает применение параметров ModulePullOverride. Любое другое значение означает конфликт.

Пример отсутствия конфликтов при применении ModulePullOverride:

```console
$ kubectl get modulepulloverrides.deckhouse.io 
NAME      UPDATED   MESSAGE
example1  10s       Ready
```

Требования к модулю:
* Модуль должен существовать, иначе сообщение у ModulePullOverride будет *The module not found*.

  Пример:

  ```console
  $ kubectl get modulepulloverrides.deckhouse.io 
  NAME      UPDATED   MESSAGE
  example1  10s       The module not found
  ```

* Модуль не должен быть встроенным модулем Deckhouse, иначе сообщение у ModulePullOverride будет *The module is embedded*.

  Пример:

  ```console
  $ kubectl get modulepulloverrides.deckhouse.io 
  NAME           UPDATED  MESSAGE
  ingress-nginx  10s      The module is embedded
  ```

* Модуль должен быть включен, иначе сообщение у ModulePullOverride будет *The module disabled*.

  Пример:

  ```console
  $ kubectl get modulepulloverrides.deckhouse.io 
  NAME     UPDATED   MESSAGE
  example  7s        The module disabled
  ```

* Модуль должен иметь источник, иначе сообщение у ModulePullOverride будет *The module does not have an active source*.
  
  Пример:

  ```console
  $ kubectl get modulepulloverrides.deckhouse.io 
  NAME       UPDATED   MESSAGE
  example    12s       The module does not have an active source
  ```

* Источник модуля должен существовать, иначе сообщение у ModulePullOverride будет *The source not found*.

  Пример:

  ```console
  $ kubectl get modulepulloverrides.deckhouse.io 
  NAME       UPDATED   MESSAGE
  example    12s       The source not found
  ```

Чтобы обновить модуль не дожидаясь начала следующего цикла обновления, можно выполнить следующую команду:

```sh
kubectl annotate mpo <name> renew=""
```

## Принцип действия

После создания ModulePullOverride, соответствующий модуль не будет учитывать ModuleUpdatePolicy, а также не будет загружать и создавать объекты ModuleRelease. Модуль будет загружаться при каждом изменении параметра `imageDigest`, после чего будет применяться в кластере. В статусе ModuleSource модуль получит признак `overridden: true`, который указывает на то, что используется ModulePullOverride, а не ModuleUpdatePolicy. Также, соответствующий объект Module будет иметь в своем статусе поле `IsOverridden` и версию модуля из `imageTag`.

Пример:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  creationTimestamp: "2024-11-18T15:34:15Z"
  generation: 16
  labels:
    deckhouse.io/epoch: "1326105356"
  name: example
  resourceVersion: "230347744"
  uid: 7111cee7-50cd-4ecf-ba20-d691b13b0f59
properties:
  availableSources:
  - example
  releaseChannel: Stable
  requirements:
    deckhouse: '> v1.63.0'
    kubernets: '> v1.30.0'
  source: example
  version: mpo-tag
  weight: 910
status:
  conditions:
  - lastProbeTime: "2024-12-03T15:57:20Z"
    lastTransitionTime: "2024-12-03T15:57:20Z"
    status: "True"
    type: EnabledByModuleConfig
  - lastProbeTime: "2024-12-03T15:59:58Z"
    lastTransitionTime: "2024-12-03T15:57:26Z"
    status: "True"
    type: EnabledByModuleManager
  - lastProbeTime: "2024-12-03T15:59:58Z"
    lastTransitionTime: "2024-12-03T15:56:23Z"
    status: "True"
    type: IsReady
  - lastProbeTime: "2024-12-03T15:59:48Z"
    lastTransitionTime: "2024-12-03T15:56:47Z"
    status: "True"
    type: IsOverridden
  phase: Ready
```

После удаления ModulePullOverride модуль продолжит работать. Но, если для модуля существует [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy), то загрузятся новые релизы модуля (ModuleRelease), которые заменят текущую "версию разработчика".

### Пример

1. В [ModuleSource](../../cr.html#modulesource) присутствуют два модуля `echo` и `hello-world`. Для них определена политика обновления, они загружаются и устанавливаются в DKP:

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

1. Включите модуль и создайте [ModulePullOverride](../../cr.html#modulepulloverride) для модуля `echo`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
   ```

   После создания ModulePullOverride, для модуля будет использоваться тег образа `registry.example.com/deckhouse/modules/echo:main-patch-03354` (`ms:spec.registry.repo/mpo:metadata.name:mpo:spec.imageTag`).

1. Данные ModulePullOverride будут меняться при каждом обновлении модуля:

   ```yaml
   apiVersion: deckhouse.io/v1alpha2
   kind: ModulePullOverride
   metadata:
     name: echo
   spec:
     imageTag: main-patch-03354
     scanInterval: 15s
   status:
     imageDigest: sha256:ed958cc2156e3cc363f1932ca6ca2c7f8ae1b09ffc1ce1eb4f12478aed1befbc
     message: "Ready"
     updatedAt: "2023-12-07T08:41:21Z"
   ```

   где:
   - `imageDigest` — уникальный идентификатор образа контейнера, который был загружен.
   - `lastUpdated` — время последней загрузки образа.

1. При этом ModuleSource приобретет вид:

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

## Артефакты модуля в container registry

После сборки модуля его артефакты должны быть загружены в container registry по пути, который является *источником* для загрузки и запуска модулей в DKP. Путь, по которому загружаются артефакты модулей в registry, указывается в ресурсе [ModuleSource](../../cr.html#modulesource).

Пример иерархии образов контейнеров после загрузки артефактов модулей `module-1` и `modules-2` в registry:

```tree
registry.example.io
📁 modules-source
├─ 📁 module-1
│  ├─ 📦 v1.23.1
│  ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
│  ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
│  ├─ 📦 v1.23.2
│  └─ 📁 release
│     ├─ 📝 v1.23.1
│     ├─ 📝 v1.23.2
│     ├─ 📝 alpha
│     └─ 📝 beta
└─ 📁 module-2
   ├─ 📦 v0.30.147
   ├─ 📦 d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
   ├─ 📦 e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
   ├─ 📦 v0.31.1
   └─ 📁 release
      ├─ 📝 v0.30.147
      ├─ 📝 v0.31.1
      ├─ 📝 alpha
      └─ 📝 beta
```

{% alert level="warning" %}
Container registry должен поддерживать вложенную структуру репозиториев. Подробнее об этом [в разделе требований](../#требования).  
{% endalert %}

Далее приведен список команд для работы с источником модулей. В примерах используется утилита [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane#crane). Установите ее [по инструкции](https://github.com/google/go-containerregistry/tree/main/cmd/crane#installation). Для macOS воспользуйтесь `brew`.

### Вывод списка модулей в источнике модулей

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>
```

Пример:

```shell
$ crane ls registry.example.io/modules-source
module-1
module-2
```

### Вывод списка образов модуля

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>
```

Пример:

```shell
$ crane ls registry.example.io/modules-source/module-1
v1.23.1
d4bf3e71015d1e757a8481536eeabda98f51f1891d68b539cc50753a-1589714365467
e6073b8f03231e122fa3b7d3294ff69a5060c332c4395e7d0b3231e3-1589714362300
v1.23.2
```

В примере в модуле `module-1` присутствуют два образа модуля и два образа контейнеров приложений.

### Вывод файлов в образе модуля

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>:<MODULE_TAG> - | tar -tf -
```

Пример:

```shell
crane export registry.example.io/modules-source/module-1:v1.23.1 - | tar -tf -
```

Ответ будет достаточно большим.

### Вывод списка образов контейнеров приложений модуля

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>:<MODULE_TAG> - | tar -Oxf - images_digests.json
```

Пример:

```shell
$ crane export registry.example.io/modules-source/module-1:v1.23.1 -  | tar -Oxf - images_digests.json
{
  "backend": "sha256:fcb04a7fed2c2f8def941e34c0094f4f6973ea6012ccfe2deadb9a1032c1e4fb",
  "frontend": "sha256:f31f4b7da5faa5e320d3aad809563c6f5fcaa97b571fffa5c9cab103327cc0e8"
}
```

### Просмотр списка релизов

```shell
crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release
```

Пример:

```shell
$ crane ls <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release
v1.23.1
v1.23.2
alpha
beta
```

В примере в container registry два релиза и используются два канала обновлений: `alpha` и `beta`.

### Вывод версии, используемой на канале обновлений `alpha`

```shell
crane export <REGISTRY_URL>/<MODULE_SOURCE>/<MODULE_NAME>/release:alpha - | tar -Oxf - version.json
```

Пример:

```shell
$ crane export registry.example.io/modules-source/module-1/release:alpha - | tar -Oxf - version.json
{"version":"v1.23.2"}
```
