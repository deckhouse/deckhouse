---
title: "Запуск и проверка модуля в кластере"
permalink: ru/module-development/run/
lang: ru
---

В этом разделе рассмотрен процесс запуска модуля в кластере Deckhouse Kubernetes Platform (DKP), а также подключение Deckhouse Module Tools для проверки модуля и сбора метрик.

## Запуск модуля в кластере DKP

Чтобы запустить модуль в кластере, необходимо выполнить следующие шаги:

- [Определить источник модулей](#источник-модулей) (ресурс [ModuleSource](../../cr.html#modulesource)).
- _(не обязательно)_ Определить [политику обновления модуля](#политика-обновления-модуля) (ресурс [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy)).
- [Включить модуль в кластере](#включение-модуля-в-кластере) (ресурс [ModuleConfig](../../cr.html#moduleconfig)).

### Источник модулей

Чтобы указать в кластере источник, откуда нужно загружать информацию о модулях, необходимо создать ресурс [ModuleSource](../../cr.html#modulesource). В этом ресурсе указывается адрес container registry, откуда DKP будет загружать модули, параметры аутентификации и другие настройки.

Пример ресурса ModuleSource:

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

После создания ресурса ModuleSource DKP начнет выполнять периодическую (раз в три минуты) синхронизацию данных с источником модулей (загружать информацию о модулях, доступны в источнике).

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

Подробную информацию об ошибках можно получить в поле `pullError` в статусе ресурса ModuleSource.

Пример получения подробной информации об ошибках из источника модулей `example`:

```console
$ kubectl get ms example -o jsonpath='{range .status.modules[*]}{.name}{" module error:\n\t"}{.pullError}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

В случае успешной синхронизации, поле `.status.modules` ресурса ModuleSource будет содержать список модулей, доступных для включения в кластере.

Пример получения списка модулей, доступных из источника модулей `example`:

```console
$ kubectl get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

Полный список модулей, доступных из всех созданных в кластере источников модулей, можно получить с помощью следующей команды:

```shell
kubectl get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

После создания ресурса ModuleSource и успешной синхронизации, в кластере должны начать появляться _модули_ — ресурсы [Module](../../cr.html#module) (DKP создает их автоматически, создавать их не нужно).
Посмотреть список модулей можно с помощью следующей команды:

```shell
kubectl get module
```

Пример получения списка модулей:

```console
$ kubectl get module
NAME       STAGE    SOURCE   PHASE       ENABLED   READY
module-one                   Available   False     False                      
module-two                   Available   False     False                      
```

Чтобы получить дополнительную информацию о модуле, выполните следующую команду:

```shell
kubectl get module module-one -oyaml
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

В Module указаны доступные источники из которых его можно скачать (в примере он только один).

Далее нужно включить модуль. Для этого нужно создать ModuleConfig с названием модуля.

За включение модуля отвечает параметр `enabled` ModuleConfig. Если модуль доступен из нескольких источников (ресурс ModuleSource), необходимый источник можно указать в параметре `source`.

Политику обновления (имя ModuleUpdatePolicy) можно указать в параметре `updatePolicy`. Политику обновления можно не указывать, — в этом случае она будет унаследована от параметров обновления Deckhouse. :

Пример ModuleConfig для включения модуля `module-one` из источника `example`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-one
spec:
  enabled: true
  source: example
```

{% alert level="warning" %}
Если в конфигурации модуля есть обязательные параметры, и модуль включён без их указания, произойдёт ошибка валидации конфигурации. В этом случае сработает алерт `D8DeckhouseModuleValidationError`, а модуль не будет успешно активирован.

Для получения подробной информации используйте команду:

```shell
kubectl get mr -l module=<MODULE_NAME>
```

Убедитесь, что вы указываете необходимые параметры конфигурации в `ModuleConfig` согласно документации модуля.
{% endalert %}

После включения модуля, он должен перейти в фазу скачивания (`Downloading`):

```shell
$ kubectl get module module-one
NAME        STAGE    SOURCE   PHASE         ENABLED   READY
module-one           example  Downloading   False     False
```

{% alert level="warning" %}
Если модуль не перешел в фазу скачивания, проверьте источник модуля (ModuleSource), возможно модуль не может скачаться.
{% endalert %}

После успешного скачивания модуль перейдет в фазу установки (`Installing`):

```shell
$ kubectl get module module-one
NAME        STAGE    SOURCE   PHASE         ENABLED   READY
module-one           example  Installing    False     False
```

Если модуль успешно установился, то он перейдет в фазу готовности (`Ready`):

```shell
$ kubectl get module module-one
NAME        STAGE   SOURCE   PHASE  ENABLED  READY
module-one          example  Ready  True     True
```

Пример объекта Module в кластере, когда модуль успешно установился:

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

В Module можно увидеть текущую установленную версию модуля, его вес, источник откуда он скачался, зависимости и релизный канал.

При возникновении каких либо ошибок, модуль перейдет в фазу ошибки (`Error`):

```console
$ kubectl get module module-one
NAME        STAGE    SOURCE   PHASE  ENABLED  READY
module-one           example  Error  True     Error
```

Если у включенного модуля есть несколько доступных источников, и в его ModuleConfig явно не выбран источник модуля, модуль перейдет в фазу конфликта (`Conflict`):

```console
$ kubectl get module module-one
NAME        STAGE    SOURCE   PHASE     ENABLED  READY
module-one                    Conflict  False    False
```

Чтобы разрешить конфликт, укажите источник модуля (имя ModuleSource) явно в ModuleConfig.

После скачивания модуля в кластере появятся релизы модуля — объекты ModuleRelease.

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

Если релиз модуля находится в статусе `Superseded`, это значит что релиз модуля устарел, и есть более новый релиз, который его заменил.

{% alert level="warning" %}
Если релиз модуля находится в статусе `Pending`, то это значит что он требует ручного подтверждения для установки (смотри далее [про политику обновления модуля](#политика-обновления-модуля)). Подтвердить релиз модуля можно следующей командой (укажите имя moduleRelease):

```shell
kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Пропуск промежуточных релизов (from-to)

Механизм `from-to` позволяет пропускать [пошаговые обновления модуля](../development/#логика-автообновления-модулей). Если текущая текущая установленная версия модуля (в статусе `Deployed`) попадает в диапазон `from-to`, DKP пропускает промежуточные релизы и устанавливает последнюю доступную версию в пределах `to`.

Чтобы включить механизм, задайте правила перехода в конфигурации модуля (`module.yaml`). Пример:

```yaml
update:
  versions:
    - from: "1.67"
      to:   "1.75"
    - from: "1.99"
      to:   "2.0" # Переход между версиями задаётся в формате X.0.
```

DKP сравнивает текущую версию модуля со значением `from`. Если версия не ниже `from`, правило применяется: берётся `to`, и устанавливается последняя доступная версия в пределах `to`.

Если одновременно подходят несколько правил, выбирается более дальний путь (самый длинный прыжок). Обновление выполняется только в релиз модуля, для которого заданы update-настройки (параметр [update.versions](../../cr.html#modulerelease-v1alpha1-spec-update)). Правила `from-to` могут находиться в разных объектах ModuleRelease этого же модуля. Если ни одно правило не подошло, обновление идёт как обычно — без пропуска промежуточных версий.

{% alert level="info" %}
Если у релиза есть правила `from-to`, DKP не требует ставить версии по порядку. Такой релиз просто появляется в списке как есть, и вы можете сразу подтвердить нужный релиз из ветки `to` — промежуточные версии будут помечены как `Skipped`.
{% endalert %}

![Логика механизма `from-to`](../../images/module-development/from_to_ru.svg)

Проверить доступные релизы (ModuleRelease) можно с помощью команды:

```shell
d8 k get mr
```

Пример вывода, если текущая версия модуля — `0.3.33`, а в module.yaml задано правило `from: "0.3" → to: "0.7"`:

```console
p-o-test-v0.1.0    Superseded
p-o-test-v0.2.22   Superseded
p-o-test-v0.3.33   Deployed
p-o-test-v0.4.1    Pending      Release is waiting for the 'modules.deckhouse.io/approved: "true"' annotation
p-o-test-v0.5.27   Pending      awaiting for v0.4.1 release to be deployed
p-o-test-v0.6.11   Pending      awaiting for v0.4.1 release to be deployed
p-o-test-v0.7.25   Pending      Release is waiting for the 'modules.deckhouse.io/approved: "true"' annotation
```

В этом примере текущая установленная версия модуля — `0.3.33` (в статусе `Deployed`).
В `module.yaml` задано правило `from: 0.3 → 0.7`. Из `0.3` доступны два варианта обновления: `0.4` и `0.7`.

DKP определяет, что `0.3.33` не ниже `0.3`, и выбирает к установке последнюю доступную версию в пределах `0.7 — p-o-test-v0.7.25` (в статусе `Pending`).

После подтверждения релиз модуля версии `0.4.1`, `0.5.27` и `0.6.11` становятся в статусе `Skipped`, а `p-o-test-v0.7.25` в статусе `Deployed`.

Подтвердить релиз модуля можно применив аннотацию с помощью команды:

```shell
d8 k annotate mr p-o-test-v0.7.25 modules.deckhouse.io/approved="true"
```

Пример вывода после подтверждения:

```console
p-o-test-v0.4.1    Skipped
p-o-test-v0.5.27   Skipped
p-o-test-v0.6.11   Skipped
p-o-test-v0.7.25   Deployed
```

В данном примере вывода промежуточные релизы помечаются как `Skipped`, а релиз модуля `0.7.25` устанавливается в кластере (`Deployed`).

### Переключение модуля на другой источник модулей

Если необходимо развернуть модуль из другого источника модулей, выполните следующие шаги:

1. Создайте новый [ресурс ModuleSource](#источник-модулей).

1. Укажите его в поле `source` в ModuleConfig.

1. Проверьте, что новые релизы модуля (объекты ModuleRelease) создаются из нового источника модулей в соответствии с политикой обновления:

   ```shell
   kubectl get mr
   ```

### Политика обновления модуля

Политика обновления модуля — это правила, по которым DKP обновляет модули в кластере. Она определяется ресурсом [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy), в котором можно настроить:
- режим обновления модуля (автоматический, ручной, обновления отключены);
- канал стабильности, используемый при обновлении;
- окна автоматического обновления, в пределах которых разрешено обновление модуля.

Создавать ресурс ModuleUpdatePolicy не обязательно. Если политика обновления для модуля не определена (отсутствует соответствующий ресурс ModuleUpdatePolicy), то настройки обновления соответствуют настройкам обновления самого DKP (параметр [update](../../modules/deckhouse/configuration.html#parameters-update) модуля `deckhouse`).

Пример ресурса ModuleUpdatePolicy, политика обновления которого разрешает автоматическое обновление модуля по понедельникам и средам с 13:30 до 14:00 UTC:

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

Политика обновления указывается в поле `updatePolicy` в ModuleConfig.

### Включение модуля в кластере

Прежде чем включить модуль, проверьте что он доступен для включения. Выполните следующую команду, чтобы вывести список всех доступных модулей DKP:

```shell
kubectl get modules
```

Модуль должен быть в списке.

Пример вывода:

```console
$ kubectl get module
NAME       STAGE    SOURCE   PHASE       ENABLED   READY
...
module-one                   Available   False     False                      
module-two                   Available   False     False     
...
```

Вывод показывает, что модуль `module-one` доступен для включения.

Если модуля нет в списке, то проверьте что определен [источник модулей](#источник-модулей) и модуль есть в списке в источнике модулей. Также проверьте [политику обновления](#политика-обновления-модуля) модуля (если она определена). Если политика обновления модуля не определена, то она соответствует политике обновления DKP (параметр [releaseChannel](../../modules/deckhouse/configuration.html#parameters-releasechannel) и секция [update](../../modules/deckhouse/configuration.html#parameters-update) параметров модуля `deckhouse`).

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
- Посмотреть журнал Deckhouse:

  ```shell
  kubectl -n d8-system logs -l app=deckhouse
  ```

- Посмотреть объект Module подробнее:

  ```console
  kubectl get module module-one -oyaml
  ```
  
- Посмотреть объект ModuleConfig модуля:

  Пример вывода информации об ошибке модуля `module-one`:

  ```console
  $ kubectl get moduleconfig module-one
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-one  true                7s    Ignored: unknown module name
  ```

- Посмотреть объект ModuleSource:

  Пример вывода если у источника модуля есть проблемы со скачиванием модуля:

  ```console
  $ kubectl get ms
  NAME        COUNT   SYNC   MSG
  example     2       16s    Some errors occurred. Inspect status for details
  ```

По аналогии [с DeckhouseRelease](../../cr.html#deckhouserelease) (ресурсом релиза DKP) у модулей есть аналогичный ресурс — [ModuleRelease](../../cr.html#modulerelease). DKP создает ModuleRelease исходя из того, что хранится в container registry.
При поиске проблем с модулем проверьте также доступные в кластере ModuleRelease:

```shell
kubectl get mr
```

Пример вывода:

```console
$ kubectl get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for the 'release.deckhouse.io/approved: "true"' annotation
```

В примере вывода показан ModuleRelease, когда режим обновления (параметр [update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) ModuleUpdatePolicy установлен в `Manual`. В этом случае необходимо вручную подтвердить установку новой версии модуля, установив на ModuleRelease аннотацию `modules.deckhouse.io/approved="true"`:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```

## Подключение Deckhouse Module Tools для проверки модуля

Для автоматической проверки структуры модуля и, при необходимости, отправки статистики, в сборку можно подключить Deckhouse Module Tools (DMT).

### Для GitHub-проектов

Для GitHub доступен отдельный [GitHub Action](https://github.com/deckhouse/modules-actions/blob/main/lint/action.yml) для подключения DMT к модулю.

Чтобы подключить DMT, в конфигурации workflow сборки `[project].github/workflows/build.yml` добавьте шаг для выполнения проверки:

{% raw %}

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    continue-on-error: true
    name: Linting
    steps:
      - uses: actions/checkout@v4
      - uses: deckhouse/modules-actions/lint@main
      env:
         DMT_METRICS_URL: ${{ secrets.DMT_METRICS_URL }}
         DMT_METRICS_TOKEN: ${{ secrets.DMT_METRICS_TOKEN }}
```

{% endraw %}

Переменные `DMT_METRICS_URL` и `DMT_METRICS_TOKEN` – необязательные. При их наличии DMT будет отправлять телеметрию на указанный адрес.

> Если модуль находится в GitHub-группе `deckhouse`, значения этих переменных будут автоматически получены из настроенных секретов.

Пример полной конфигурации можно увидеть в файле [build_dev.yml](https://github.com/deckhouse/csi-nfs/blob/main/.github/workflows/build_dev.yml#L39C1-L42C62).

Для упрощения настройки сборки воспользуйтесь [шаблонами конфигурации](https://github.com/deckhouse/modules-actions/blob/main/.examples/build.yml).

### Для GitLab-проектов

Для GitLab также доступны готовые шаблоны, которые можно подключить в `.gitlab-ci.yml`  для автоматической настройки процессов сборки и проверки корректности:

- **Setup**: [Шаблон конфигурации для настройки](https://github.com/deckhouse/modules-gitlab-ci/blob/main/templates/Setup.gitlab-ci.yml).
- **Build**: [Шаблон конфигурации для процесса сборки](https://github.com/deckhouse/modules-gitlab-ci/blob/main/templates/Build.gitlab-ci.yml).

#### Шаги для подключения

1. В файле `.gitlab-ci.yml` вашего проекта добавьте ссылки на шаблоны:

    ```yaml
    include:
      - remote: https://raw.githubusercontent.com/deckhouse/modules-gitlab-ci/refs/heads/main/templates/Setup.gitlab-ci.yml
      - remote: https://raw.githubusercontent.com/deckhouse/modules-gitlab-ci/refs/heads/main/templates/Build.gitlab-ci.yml
    ```

    Пример добавления ссылок расположен в [GitLab](https://fox.flant.com/deckhouse/flant-integration/-/blob/main/.gitlab-ci.yml?ref_type=heads#L2).

1. После подключения шаблонов, в той же конфигурации `.gitlab-ci.yml` добавьте шаг для выполнения проверки:

    ```yaml
    Lint:
      extends: .lint
    ```

    Пример добавления шага проверки расположен в [GitLab](https://fox.flant.com/deckhouse/flant-integration/-/blob/main/.gitlab-ci.yml?ref_type=heads#L48).

> Если проект находится в группе [https://fox.flant.com/deckhouse](https://fox.flant.com/deckhouse), переменные для отправки метрик уже заданы. Дополнительно ничего конфигурировать не требуется.
