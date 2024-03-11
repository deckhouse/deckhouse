---
title: "Как запустить модуль в кластере DKP?"
permalink: ru/module-development/run/
lang: ru
---

В этой разделе рассмотрен процесс запуска настроенного модуля в кластере Deckhouse Kubernetes Platform (DKP).

## Создайте источник модулей

Чтобы выложить модуль в кластер, создайте источник модулей — ресурс [*ModuleSource*](../../cr.html#modulesource). Этот ресурс определяет container registry, откуда DKP будет загружать модули.

Пример *ModuleSource*:

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

Проверьте, что ресурс создан:

```sh
kubectl get ms
```

Пример вывода:

```yaml
NAME        COUNT   SYNC   MSG
example     2       16s
```

Как только ресурс будет создан, проверьте список модулей, которые находятся в подключенном *ModuleSource*:

```shell
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

DKP обновляет список модулей и версий один раз в 3 минуты.

## Создайте политику обновлений модуля

Ресурс [*ModuleUpdatePolicy*](../../cr.html#moduleupdatepolicy) определит список модулей, которые будут установлены. Если настройки *ModuleUpdatePolicy* для *ModuleSource* не указаны, то используются настройки релизного канала и обновлений, установленные для DKP.

Чтобы не скачивать модули, установите режим `mode: Ignore`.

В примере *ModuleUpdatePolicy* параметр `labelSelector` ограничивает действие политики модулем `module-1`, полученным из источника модулей с именем `example`, режим обновления выбран `Manual`:

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

## Проверьте релизы модуля

По аналогии с [*DeckhouseRelease*](../../modules/002-deckhouse/cr.html#deckhouserelease), у модулей есть релизы. Для исправления бага в модуле не нужно ждать нового релиза DKP.

DKP создает ресурсы [*ModuleRelease*](../../cr.html#modulerelease) исходя из того, что хранится в container registry.

Проверьте доступные релизы модуля:

```shell
kubectl get mr
```

Пример вывода:

```yaml
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for manual approval
```

Так как в *ModuleSource* был указан канал обновления `alpha`, были загружены новые версии модулей.

Режим обновления *ModuleUpdatePolicy* установлен в `Manual`, поэтому необходимо вручную подтвердить установку новой версии. Для этого добавьте аннотацию к указанному релизу:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```

Если используется автоматический режим обновлений (`Auto`), будет установлен автоматический релиз при ближайшем релизном окне или при фактической загрузке, если окна не указаны.

## Проверьте доступность модулей

После загрузки и установки можно проверить, доступны ли модули для использования. Для этого выведите список всех доступных модулей DKP:

```sh
kubectl get modules | grep example
```

Пример вывода:

```yaml
NAME                                  WEIGHT   STATE      SOURCE
module-1                              900      Disabled   example
module-2                              900      Disabled   example
```

Готово, модули стали доступны.

## Включите модуль в кластере

Теперь можно работать с модулями, как с обычными модулями DKP.

Создайте [*ModuleConfig*](../../cr.html#moduleconfig), чтобы включить `module-1` в кластере:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: module-1
spec:
  enabled: true
  settings: \{}
  version: 1
```

Если появятся проблемы с модулем, то DKP запишет ошибку в *ModuleConfig*. Проверьте, что ошибка не отображается:

```shell
kubectl get moduleconfig module-1
```

Пример вывода:

```yaml
NAME              STATE     VERSION   AGE   TYPE                  STATUS
module-1          Enabled   1         3m    example
```
