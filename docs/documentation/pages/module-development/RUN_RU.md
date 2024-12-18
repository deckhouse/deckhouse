---
title: "Запуск модуля в кластере"
permalink: ru/module-development/run/
lang: ru
---

В этом разделе рассмотрен процесс запуска настроенного модуля в кластере Deckhouse Kubernetes Platform (DKP).

Чтобы запустить модуль в кластере, необходимо выполнить следующие шаги:

- [Определить источник модулей](#источник-модулей) (ресурс [ModuleSource](../../cr.html#modulesource)).
- _(не обязательно)_ Определить [политику обновления модуля](#политика-обновления-модуля) (ресурс [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy)).
- [Включить модуль в кластере](#включение-модуля-в-кластере) (ресурс [ModuleConfig](../../cr.html#moduleconfig)).

## Источник модулей

Чтобы указать в кластере источник, откуда нужно загружать информацию о модулях, необходимо создать ресурс [ModuleSource](../../cr.html#modulesource). В этом ресурсе указывается адрес container registry, откуда DKP будет загружать модули, параметры аутентификации и другие настройки.

Пример ресурса `ModuleSource`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleSource
metadata:
  name: example
spec:
  registry:
    repo: registry.example.com/deckhouse/modules
    dockerCfg: <base64 encoded credentials>
```

После создания ресурса `ModuleSource` DKP начнет выполнять периодическую (раз в три минуты) синхронизацию данных с источником модулей (загружать информацию о модулях, доступны в источнике).

Проверить состояние синхронизации можно с помощью следующей команды:

```shell
kubectl get ms
```

Пример вывода в случае успешной синхронизации:

```shell
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Ready
```

В случае ошибок синхронизации в столбце `MSG` будет указано общее описание ошибки. Пример:

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Some errors occurred. Inspect status for details
```

Подробную информацию об ошибках можно получить в поле `pullError` в статусе ресурса _ModuleSource_.

Пример получения подробной информации об ошибках из источника модулей `example`:

```console
$ kubectl get ms example -o jsonpath='{range .status.modules[*]}{.name}{" module error:\n\t"}{.pullError}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

В случае успешной синхронизации, поле `.status.modules` ресурса _ModuleSource_ будет содержать список модулей, доступных для включения в кластере.

Пример получения списка модулей, доступных из источника модулей `example`:

```console
$ kubectl get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

Полный список модулей, доступных из всех созданных в кластере источников модулей, можно получить с помощью следующей команды:

```shell
kubectl get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

После создания ресурса `ModuleSource` и успешной синхронизации, в кластере должны начать появляться _модули_ — ресурсы [Module](../../cr.html#module) (DKP создает их автоматически, создавать их не нужно). 
Посмотреть список модулей можно с помощью следующей команды:

```shell
kubectl get module
```

Пример получения списка модулей:

```сonsole
$ kubectl get module
NAME       WEIGHT   SOURCE   PHASE       ENABLED   READY
module-one                   Available   False     False                      
module-two                   Available   False     False                      
```

Чтобы посмотреть на модуль подробнее можно выполнить следующую команду:
```shell
$ kubectl get module module-one -oyaml
```

Пример вывода:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  creationTimestamp: "2024-12-12T10:49:40Z"
  generation: 1
  name: module-one
  resourceVersion: "241504954"
  uid: 3ae75474-8e96-4105-a939-6df71cba82d8
properties:
  availableSources:
  - example
status:
  conditions:
  - lastProbeTime: "2024-12-12T10:49:41Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    message: disabled
    reason: Disabled
    status: "False"
    type: EnabledByModuleConfig
  - lastProbeTime: "2024-12-12T10:49:41Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    status: "False"
    type: EnabledByModuleManager
  - lastProbeTime: "2024-12-16T15:46:26Z"
    lastTransitionTime: "2024-12-12T10:49:41Z"
    message: not installed
    reason: NotInstalled
    status: "False"
    type: IsReady
  phase: Available
```

В ресурсе модуля указаны доступные источники из которых его можно скачать(в нашем случае он лишь один).

Следующим шагом нужно включить модуль, для этого нужно создать одноименый модуль конфиг.

В нем потребуется указать что модуль требуется включить, указать источник(если доступен всего один источник то указывать опционально) и политику обновления(опционально, если не указывать будет унаследована политика обновления самого deckhouse):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  enabled: true
  source: example
```

После этого модуль должен перейти в фазу скачивания:

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE         ENABLED   READY
module-one           example  Downloading   False     False
```

{% alert level="warning" %}

Если модуль не перешел в данную фазу стоит проверить источник, возможно он не может скачать модуль.

{% endalert %}

После успешного скачивания модуль перейдет в фазу установки:

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE         ENABLED   READY
module-one  900      example  Installing    False     False
```

Если модуль успешно установился, то он перейдет в фазу готов:

```shell
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE  ENABLED  READY
module-one  900      example  Ready  True     True
```

Посмотрим на ресурс модуля в данный момент:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: Module
metadata:
  creationTimestamp: "2024-11-18T15:34:15Z"
  generation: 1
  name: module-one
  resourceVersion: "242153004"
  uid: 7111cee7-50cd-4ecf-ba20-d691b13b0f59
properties:
  availableSources:
  - example
  releaseChannel: Stable
  requirements:
    deckhouse: '> v1.63.0'
    kubernetes: '> v1.25.0'
  source: example
  version: v0.7.24
  weight: 910
status:
  conditions:
  - lastProbeTime: "2024-12-12T15:49:35Z"
    lastTransitionTime: "2024-12-12T15:49:35Z"
    status: "True"
    type: EnabledByModuleConfig
  - lastProbeTime: "2024-12-17T09:35:27Z"
    lastTransitionTime: "2024-12-12T15:49:39Z"
    status: "True"
    type: EnabledByModuleManager
  - lastProbeTime: "2024-12-17T09:35:27Z"
    lastTransitionTime: "2024-12-17T09:35:25Z"
    status: "True"
    type: IsReady
  - lastProbeTime: "2024-12-17T09:32:50Z"
    lastTransitionTime: "2024-12-17T09:32:50Z"
    status: "False"
    type: IsOverridden
  hooksState: 'v0.7.24/hooks/moduleVersion.py: ok'
  phase: Ready
```

Мы можем увидеть в нем текущую установленную версию модуля, его вес, источник откуда он скачался, его зависимости и релизный канал.

При возникновении каких либо ошибок модуль перейдет в фазу ошибки:

```console
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE  ENABLED  READY
module-one  910      example  Error  True     Error
```

В случае если модуль включен, и имеет несколько доступных источников, а в модуль конфиге явно не выбран источник, модуль перейдет в фазу конфликта:

```console
$ kubectl get module module-one
NAME        WEIGHT   SOURCE   PHASE     ENABLED  READY
module-one                    Conflict  Fasle    False
```

Чтобы решить конфликт, укажите источник явно в конфиге модуля.

После скачивания модуля в кластере появится модуль релиз модуля.

Посмотреть список релизов можно с помощью следующей команды:

```shell
kubectl get mr
```

Пример получения списка релизов модулей:
```console
$ kubectl get mr
NAME                       PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
module-one-v0.7.23         Superseded   deckhouse       33h              
module-one-v0.7.24         Deployed     deckhouse       33h              
module-two-v1.2.0          Superseded   deckhouse       48d              
module-two-v1.2.1          Superseded   deckhouse       48d              
module-two-v1.2.3          Deployed     deckhouse       48d              
module-two-v1.2.4          Superseded   deckhouse       44d              
module-two-v1.2.5          Pending      deckhouse       44d              Waiting for the 'release.deckhouse.io/approved: \"true\"' annotation
```

Если релиз модуля находится в статусе Superseded, это значит что релиз модуля устарел, и есть более новый релиз, который его заменил.

{% alert level="warning" %}

Если релиз модуля находится в статусе Pending, то это значит что он требует ручного подтверждения для установки (смотри далее [про политику обновления модуля](#политика-обновления-модуля)). Подтвердить релиз модуля можно следующей командой (укажите имя _moduleRelease_):

```shell
kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Переключение модуля на другой источник модулей

Если необходимо развернуть модуль из другого источника модулей, выполните следующие шаги:

1. Создайте новый [ресурс ModuleSource](#источник-модулей).

2. Укажите его в поле `source` у модуль конфига.

3. Проверьте, что новые _ModuleRelease_ для модуля создаются из нового _ModuleSource_ в соответствии с политикой обновления.

   ```shell
   kubectl get mr
   ```

## Политика обновления модуля

Политика обновления модуля — это правила, по которым DKP обновляет модули в кластере. Она определяется ресурсом [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy), в котором можно настроить:
- режим обновления модуля (автоматический, ручной, обновления отключены);
- канал стабильности, используемый при обновлении;
- окна автоматического обновления, в пределах которых разрешено обновление модуля.

Создавать ресурс `ModuleUpdatePolicy` не обязательно. Если политика обновления для модуля не определена (отсутствует соответствующий ресурс `ModuleUpdatePolicy`), то настройки обновления соответствуют настройкам обновления самого DKP (параметр [update](../../modules/002-deckhouse/configuration.html#parameters-update) модуля `deckhouse`).

Пример ресурса `ModuleUpdatePolicy`, данная политика обновления разрешает автоматическое обновление модуля по понедельникам и средам с 13:30 до 14:00 UTC:

```yaml
apiVersion: deckhouse.io/v1alpha2
kind: ModuleUpdatePolicy
metadata:
  name: example-update-policy
spec:
  releaseChannel: Alpha
  update:
    mode: Auto
    windows:
    - days:
      - "Mon"
      - "Wed"
      from: "13:30"
      to: "14:00"
```

Политика обновления указывается в поле `updatePolicy` модуль конфига.

## Включение модуля в кластере

Прежде чем включить модуль, проверьте что он доступен для включения. Выполните следующую команду, чтобы вывести список всех доступных модулей DKP:

```shell
kubectl get modules
```

Модуль должен быть в списке.

Пример вывода:

```console
$ kubectl get module
NAME       WEIGHT   SOURCE   PHASE       ENABLED   READY
...
module-one                   Available   False     False                      
module-two                   Available   False     False     
...
```

Вывод показывает, что модуль `module-one` доступен для включения.

Если модуля нет в списке, то проверьте что определен [источник модулей](#источник-модулей) и модуль есть в списке в источнике модулей. Также проверьте [политику обновления](#политика-обновления-модуля) модуля (если она определена). Если политика обновления модуля не определена, то она соответствует политике обновления DKP (параметр [releaseChannel](../../modules/002-deckhouse/configuration.html#parameters-releasechannel) и секция [update](../../modules/002-deckhouse/configuration.html#parameters-update) параметров модуля `deckhouse`).

Включить модуль можно аналогично встроенному модулю DKP любым из следующих способов:
- Выполнить следующую команду (укажите имя модуля):

  ```shell
  kubectl -ti -n d8-system exec svc/deckhouse-leader -c deckhouse -- deckhouse-controller module enable <MODULE_NAME>
  ```

- Создать ресурс `ModuleConfig` с параметром `enabled: true` и настройками модуля.

  Пример [ModuleConfig](../../cr.html#moduleconfig), для включения и настройки модуля `module-one` в кластере:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: module-one
  spec:
    enabled: true
    settings:
      parameter: value
    version: 1
  ```

### Если что-то пошло не так

Если при включении модуля в кластере возникли ошибки, то получить информацию о них можно следующими способами:
- Посмотреть журнал DKP:

  ```shell
  kubectl -n d8-system logs -l app=deckhouse
  ```

- Посмотреть ресурс `Module` подробнее:

  Посмотреть можно следующей командой:

  ```console
  $ kubectl get module module-1 -oyaml
  ```
  
- Посмотреть ресурс `ModuleConfig` модуля:

  Пример вывода информации об ошибке модуля `module-1`:

  ```console
  $ kubectl get moduleconfig module-1
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-1    true                7s    Ignored: unknown module name
  ```

- Посмотреть ресурс `ModuleSource`:
    
  Пример вывода если у источник есть проблемы со скачиванием модуля:

  ```console
  $ kubectl get ms
  NAME        COUNT   SYNC   MSG
  example     2       16s    Some errors occurred. Inspect status for details
  ```

По аналогии [с _DeckhouseRelease_](../../cr.html#deckhouserelease) (ресурсом релиза DKP) у модулей есть аналогичный ресурс — [_ModuleRelease_](../../cr.html#modulerelease). DKP создает ресурсы _ModuleRelease_ исходя из того, что хранится в container registry. 
При поиске проблем с модулем проверьте также доступные в кластере релизы модуля:

```shell
kubectl get mr
```

Пример вывода:

```console
$ kubectl get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for the 'release.deckhouse.io/approved: "true"' annotation
```

В примере вывода показан _ModuleRelease_, когда режим обновления (параметр [update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) ресурса _ModuleUpdatePolicy_ установлен в `Manual`. В этом случае необходимо вручную подтвердить установку новой версии модуля, установив на релиз аннотацию `modules.deckhouse.io/approved="true"`:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```
