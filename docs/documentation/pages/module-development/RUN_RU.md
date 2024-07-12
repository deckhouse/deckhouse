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

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s
```

В случае ошибок синхронизации в столбце `MSG` будет указано общее описание ошибки. Пример:

```console
$ kubectl get ms
NAME        COUNT   SYNC   MSG
example     2       16s    Some errors occurred. Inspect status for details
```

Подробную информацию об ошибках можно получить в поле `status.moduleErrors` ресурса _ModuleSource_.

Пример получения подробной информации об ошибках из источника модулей `example`:

```console
$ kubectl  get ms example -o jsonpath='{range .status.moduleErrors[*]}{.name}{" module error:\n\t"}{.error}{"\n"}{end}'
module-1 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-1/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
module-2 module error:
  fetch image error: GET https://registry.example.com/v2/deckhouse/modules/module-2/release/manifests/stable: MANIFEST_UNKNOWN: manifest unknown; map[Tag:stable]
```

В случае успешной синхронизации поле `.status.modules` ресурса _ModuleSource_ будет содержать список модулей, доступных для включения в кластере.

Пример получения списка модулей, доступных из источника модулей `example`:

```console
$ kubectl get ms example -o jsonpath='{.status.modules[*].name}'
module-1 module-2
```

Полный список модулей, доступных из всех созданных в кластере источников модулей, можно получить с помощью следующей команды:

```shell
kubectl get ms  -o jsonpath='{.items[*].status.modules[*].name}'
```

После создания ресурса `ModuleSource` и успешной синхронизации, в кластере должны начать появляться _релизы модулей_ — ресурсы [ModuleRelease](cr.html#modulerelease) (DKP создает их автоматически, создавать их не нужно). Посмотреть список релизов можно с помощью следующей команды:

```shell
kubectl get mr
```

Пример получения списка релизов модулей:

```console
$ kubectl get mr
NAME                       PHASE        UPDATE POLICY   TRANSITIONTIME   MESSAGE
module-one-v1.21.3         Superseded   deckhouse       33h              
module-one-v1.22.0         Deployed     deckhouse       33h              
module-two-v1.2.0          Superseded   deckhouse       48d              
module-two-v1.2.1          Superseded   deckhouse       48d              
module-two-v1.2.3          Deployed     deckhouse       48d              
module-two-v1.2.4          Superseded   deckhouse       44d              
module-two-v1.2.5          Pending      deckhouse       44d              Waiting for manual approval

```

Если есть релиз модуля в статусе `Deployed`, то такой модуль можно [включить](#включение-модуля) в кластере. Если релиз модуля находится в статусе `Superseded`, это значит что релиз модуля устарел, и есть более новый релиз, который его заменил.

{% alert level="warning" %}
Если релиз модуля находится в статусе Pending, то это значит что он требует ручного подтверждения для установки (смотри далее [про политику обновления модуля](#политика-обновления-модуля)). Подтвердить релиз модуля можно следующей командой (укажите имя _moduleRelease_):

```shell
kubectl annotate mr <module_release_name> modules.deckhouse.io/approved="true"
```

{% endalert %}

### Переключение модуля на другой источник модулей

Если необходимо развернуть модуль из другого источника модулей, выполните следующие шаги:
1. Определите, под какую [политику обновлений](#политика-обновления-модуля) подпадает модуль:

   ```shell
   kubectl get mr
   ```

   Проверьте `UPDATE POLICY` для релизов модуля.

2. Прежде чем удалить эту политику обновления, убедитесь, что нет ожидающих развертывания (в состоянии Pending) релизов, которые подпадают под удаляемую или изменяемую политику (или _labelSelector_, используемый политикой, больше не соответствует вашему модулю):

   ```shell
   kubectl delete mup <POLICY_NAME>
   ```

3. Создайте новый [ресурс ModuleSource](#источник-модулей).

4. Создайте новый [ресурс ModuleUpdatePolicy](#политика-обновления-модуля) с указанием правильных меток (source) для нового _ModuleSource_.

5. Проверьте, что новые _ModuleRelease_ для модуля создаются из нового _ModuleSource_ в соответствии с политикой обновления.

   ```shell
   kubectl get mr
   ```

## Политика обновления модуля

Политика обновления модуля — это правила, по которым DKP обновляет модули в кластере. Она определяется ресурсом [ModuleUpdatePolicy](../../cr.html#moduleupdatepolicy), в котором можно настроить:
- режим обновления модуля (автоматический, ручной, обновления отключены);
- канал стабильности, используемый при обновлении;
- окна автоматического обновления, в пределах которых разрешено обновление модуля.

Создавать ресурс `ModuleUpdatePolicy` не обязательно. Если политика обновления для модуля не определена (отсутствует соответствующий ресурс `ModuleUpdatePolicy`), то настройки обновления соответствуют настройкам обновления самого DKP (параметр [update](../../modules/002-deckhouse/configuration.html#parameters-update) модуля `deckhouse`).

{% alert level="info" %}
Чтобы не скачивать модули, определенные в `ModuleUpdatePolicy`, установите параметр [spec.update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) в `Ignore`.
{% endalert %}

{% alert level="warning" %}
Если какой-либо модуль попадает под несколько политик обновления (условие в параметре `labelSelector`), то новые модуль не будет обновляться до тех пор, пока модуль не будет подпадать под единственную политику обновления.
{% endalert %}

Пример ресурса `ModuleUpdatePolicy`, который определяет политику обновления модуля `module-1` источника модулей `example` (ModuleSource `example`). Политика обновления разрешает автоматическое обновление модуля по понедельникам и средам с 13:30 до 14:00 UTC:

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

### Примеры moduleReleaseSelector

- Применить политику ко всем модулям _ModuleSource_ `deckhouse`:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        source: deckhouse
  ```

- Применить политику к модулю `deckhouse-admin` независимо от _ModuleSource_:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        module: deckhouse-admin
  ```

- Применить политику к модулю `deckhouse-admin` из _ModuleSource_ `deckhouse`:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchLabels:
        module: deckhouse-admin
        source: deckhouse
  ```

- Применить политику только к модулям `deckhouse-admin` и `secrets-store-integration` в _ModuleSource_ `deckhouse`:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchExpressions:
      - key: module
        operator: In
        values:
        - deckhouse-admin
        - secrets-store-integration
      matchLabels:
        source: deckhouse
  ```

- Применить политику ко всем модулям _ModuleSource_ `deckhouse`, кроме `deckhouse-admin`:

  ```yaml
  moduleReleaseSelector:
    labelSelector:
      matchExpressions:
      - key: module
        operator: NotIn
        values:
        - deckhouse-admin
      matchLabels:
        source: deckhouse
  ```

## Включение модуля в кластере

Прежде чем включить модуль, проверьте что он доступен для включения. Выполните следующую команду, чтобы вывести список всех доступных модулей DKP:

```shell
kubectl get modules
```

Модуль должен быть в списке.

Пример вывода:

```console
$ kubectl get modules
NAME                                  WEIGHT   STATE      SOURCE
...
module-test                           900      Disabled   example
...
```

Вывод показывает, что модуль `module-test` доступен для включения.

Если модуля нет в списке, то проверьте что определен [источник модулей](#источник-модулей) и модуль есть в списке в источнике модулей. Также проверьте [политику обновления](#политика-обновления-модуля) модуля (если она определена). Если политика обновления модуля не определена, то она соответствует политике обновления DKP (параметр [releaseChannel](../../modules/002-deckhouse/configuration.html#parameters-releasechannel) и секция [update](../../modules/002-deckhouse/configuration.html#parameters-update) параметров модуля `deckhouse`).

Включить модуль можно аналогично встроенному модулю DKP любым из следующих способов:
- Выполнить следующую команду (укажите имя модуля):

  ```shell
  kubectl -ti -n d8-system exec deploy/deckhouse -- deckhouse-controller module enable <MODULE_NAME>
  ```

- Создать ресурс `ModuleConfig` с параметром `enabled: true` и настройками модуля.

  Пример [ModuleConfig](../../cr.html#moduleconfig), для включения и настройки модуля `module-1` в кластере:

  ```yaml
  apiVersion: deckhouse.io/v1alpha1
  kind: ModuleConfig
  metadata:
    name: module-1
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

- Посмотреть ресурс `ModuleConfig` модуля:

  Пример вывода информации об ошибке модуля `module-1`:

  ```shell
  $ kubectl get moduleconfig module-1
  NAME        ENABLED   VERSION   AGE   MESSAGE
  module-1    true                7s    Ignored: unknown module name
  ```

По аналогии [с _DeckhouseRelease_](../../cr.html#deckhouserelease) (ресурсом релиза DKP) у модулей есть аналогичный ресурс — [_ModuleRelease_](../../cr.html#modulerelease). DKP создает ресурсы _ModuleRelease_ исходя из того, что хранится в container registry. При поиске проблем с модулем проверьте также доступные в кластере релизы модуля:

```shell
kubectl get mr
```

Пример вывода:

```shell
$ kubectl get mr
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for manual approval
```

В примере вывода показан _ModuleRelease_, когда режим обновления (параметр [update.mode](../../cr.html#moduleupdatepolicy-v1alpha1-spec-update-mode) ресурса _ModuleUpdatePolicy_ установлен в `Manual`. В этом случае необходимо вручную подтвердить установку новой версии модуля, установив на релиз аннотацию `modules.deckhouse.io/approved="true"`:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```
