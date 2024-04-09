---
title: Режимы обновлений Deckhouse Kubernetes Platform
permalink: ru/update/modes/update-mode/
lang: ru
---

{% alert level="info" %}
Модуль `deckhouse` обеспечивает механизмы обновления платформы. Убедитесь, что он включен.
{% endalert %}

Для кластера можно определить режим обновлений минорных версий Deckhouse Kubernetes Platform (DKP), тогда DKP будет либо обновлять кластер по настроенным параметрам, либо ждать подтверждения со стороны администратора. Patch-версии устанавливаются автоматически. В случае необходимости можно включить [подтверждение потенционально опасных обновлений](#подтверждение-потенциально-опасных-обновлений) или [отключить обновления в кластере полностью](#отключение-механизма-обновлений).

Существуют два режима минорных обновлений:

1. [**Автоматический режим**](#автоматический-режим): минорные обновления применяются автоматически либо в заданные окна обновлений, либо сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).

2. [**Ручной режим**](#ручной-режим): каждое минорное обновление релиза DKP нужно подтверждать вручную.

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` DKP будет каждую минуту проверять данные о релизе на канале обновлений.

## Автоматический режим

При автоматическом режиме обновления происходит следующее:

1. При появлении нового релиза DKP скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* DKP обновляет кластер на соответствующую версию согласно установленным [параметрам обновлений](modules/002-deckhouse/configuration.html#parameters-update). По умолчанию — автоматически, сразу после появления обновления.

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

<!-- Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен. -->

## Подтверждение потенциально опасных обновлений

Чтобы включить подтверждение потенциально опасных (disruptive) обновлений, добавьте параметр `disruptionApprovalMode: Manual` в *ModuleConfig*:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  version: 1
  settings:
    releaseChannel: Stable
    update:
      disruptionApprovalMode: Manual
```

<!-- Вот этот сценарий тоже отдельный. Как его установить? Для критических изменений, из-за которых обновление невозможно, настроены алерты. Например:

D8NodeHasDeprecatedOSVersion - на нодах установлена устаревшая операционная система;
HelmReleasesHasResourcesWithDeprecatedVersions - в helm-релизах используются устаревшие ресурсы;
KubernetesVersionEndOfLife - текущая версия Kubernetes больше не поддерживается.-->

В этом режиме можно подтвердить каждое минорное потенциально опасное обновление DKP на соответствующем ресурсе [*DeckhouseRelease*](cr.html#deckhouserelease).

ДЛЯ КЛАСТЕРА И ДЛЯ УЗЛОВ!

Пример подтверждения минорного обновления DKP на версию `v1.43.2`:

   ```shell
   kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
   ```

1. При необходимости, выполните обновление модуля без определенного времени.

   > Применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями.

1. Установите в соответствующем ресурсе [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`.

   Пример команды пропуска окон обновлений для версии `v1.56.2`:

   ```shell
   kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
   ```

   Пример ресурса с установленной аннотацией пропуска окон обновлений:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: DeckhouseRelease
   metadata:
     annotations:
       release.deckhouse.io/apply-now: "true"
   ...
   ```

## Ручной режим

1. Включите ручное подтверждение обновлений в ресурсе *ModuleConfig/deckhouse* с помощью параметра `update.mode`:

   ```yaml
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: deckhouse
   spec:
     version: 1
     settings:
       releaseChannel: Stable
       update:
         mode: Manual
  ```

В этом режиме необходимо подтверждать каждое минорное обновление Deckhouse Kubernetes Platform (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

**Срочное обновление**???

Обновление без окна обновлений позволяет выполнить обновление модуля вне определенного для этого времени. Это необходимо в случае срочного ручного обновления. 

> Применение обновлений без соблюдения определенного для этого времени может вызвать проблемы стабильности системы или конфликты с работающими приложениями. Поэтому используйте только в случае действительной необходимости.

Установите в соответствующем ресурсе [DeckhouseRelease](modules/002-deckhouse/cr.html#deckhouserelease) аннотацию `release.deckhouse.io/apply-now: "true"`, как показано напримерах ниже:

Пример команды установки аннотации пропуска окон обновлений для версии `v1.56.2`:

```shell
kubectl annotate deckhousereleases v1.56.2 release.deckhouse.io/apply-now="true"
```

Пример ресурса с установленной аннотацией пропуска окон обновлений:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: DeckhouseRelease
metadata:
  annotations:
    release.deckhouse.io/apply-now: "true"
...
```

## Отключение механизма обновлений

Механизм обновления DKP можно отключить полностью. Для этого удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [`releaseChannel`](modules/002-deckhouse/configuration.html#parameters-releasechannel). В этом случае Deckhouse не проверяет обновления, и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Полное отключение обновлений может привести к сбоям в работе кластера. Обновления на patch-релизы содержат исправления криических уязвимостей и ошибок.
{% endalert %}