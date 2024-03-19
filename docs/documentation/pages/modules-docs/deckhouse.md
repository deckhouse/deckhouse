---
title: "How to publish module in the DKP claster?"
permalink: en/modules-docs/module-anatomy/deckhouse/
---

В этой разделе рассмотрен процесс публикации настроенного модуля в кластере Deckhouse Kubernetes Platform, а также представлена информация, где можно просмотреть результаты.

## Создайте ресурс ModuleSource

Чтобы выложить модули в кластер, создайте ресурс *ModuleSource*. Этот ресурс определяет registry контейнера, откуда Deckhouse Kubernetes Platform будет загружать модули.

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

Проверьте, что ресурс создан.

```sh
kubectl get ms
```

Пример ответа:

```none
NAME        COUNT   SYNC   MSG
example     2       16s
```

Как только ресурс будет создан, проверьте список модулей, которые находятся в подключенном *ModuleSource*:

```shell
kubectl get ms example -o yaml
```

Пример ответа:

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

> Deckhouse обновляет список модулей и версий один раз в 3 минуты.

На этом этапе модули еще не установлены, так как не хватает модуля *ModuleUpdatePolicy*. Необходимо установить этот модуль.

## Создайте ресурс ModuleUpdatePolicy

Ресурс *ModuleUpdatePolicy* используется для определения списка модулей, которые будут установлены. 

Политика релизного канала и обновлений может быть ручная, автоматическая или автоматизированная с техническими окнами обслуживания (Manual/Auto/Auto with maintenance windows). Если настройки *ModuleUpdatePolicy* для *ModuleSource* не будут указаны, то используются настройки релизного канала и обновлений, установленные для Deckhouse Kubernetes Platform.

Также, можно установить `mode: Ignore` для того, чтобы не скачивать модули.

В следующем примере с *ModuleUpdatePolicy*, обратите внимание на параметр `labelSelector`, который ограничивает действие политики модулем `module-1`, полученным из `example` источника модулей *ModuleSource*:

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

## Создайте ресурс ModuleRelease

По аналогии с [*DeckhouseRelease*](../../../../modules/002-deckhouse/cr.html#deckhouserelease), у модулей тоже есть релизы.

> Модули из источника модулей имеют собственный цикл обновлений в отличии от Deckhouse Kubernetes Platform. Для исправления бага в модуле не нужно ждать нового релиза Deckhouse Kubernetes Platform.

Deckhouse Kubernetes Platform самостоятельно создает ресурсы *ModuleRelease* исходя из того, что хранится в registry контейнеров.

```shell
kubectl get mr
```

Пример ответа:

```none
NAME                 PHASE        UPDATE POLICY          TRANSITIONTIME   MESSAGE
module-1-v1.23.2     Pending      example-update-policy  3m               Waiting for manual approval
```

Так как в *ModuleSource* был указан канал обновления `alpha`, были загружены новые версии модулей. Так как режим обновления политики установлен в `Manual`, необходимо вручную подтвердить установку новой версии. Для этого добавьте аннотацию к указанному релизу:

```shell
kubectl annotate mr module-1-v1.23.2 modules.deckhouse.io/approved="true"
```

Если используется автоматический режим обновлений (Auto), будет установлен автоматический релиз при ближайшем релизном окне или при фактической загрузке, если окна не указаны.

## Создайте ресурс Module

После загрузки и установки можно проверить, доступны ли модули для использования. Для этого выведите список всех доступных модулей Deckhouse Kubernetes Platform:

```sh
kubectl get modules | grep example
```

Пример ответа:

```none
NAME                                  WEIGHT   STATE      SOURCE
module-1                              900      Disabled   example
module-2                              900      Disabled   example
```

Готово, модули стали доступны.

## Создайте ресурс ModuleConfig

Теперь можно работать с модулями, как с обычными модулями Deckhouse Kubernetes Platform. Создайте *ModuleConfig*, чтобы включить `module-1`.

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

Если появятся проблемы с модулем, то Deckhouse Kubernetes Platform запишет ошибку в *ModuleConfig*. Проверьте, что ошибка не отображается:

```shell
kubectl get moduleconfig module-1
```

Пример ответа:

```none
NAME              STATE     VERSION   AGE   TYPE                  STATUS
module-1          Enabled   1         3m    example
```
