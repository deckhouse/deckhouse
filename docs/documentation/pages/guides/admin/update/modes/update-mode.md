---
title: Режимы обновлений Deckhouse Kubernetes Platform
permalink: ru/update/modes/update-mode/
lang: ru
---

{% alert level="info" %}
Модуль `deckhouse` обеспечивает механизмы обновления платформы. Убедитесь, что он включен.
{% endalert %}

Для кластера можно определить режим обновлений минорных версий платформы, тогда Deckhouse Kubernetes Platform (DKP) будет либо обновлять кластер, либо ждать подтверждения обновления со стороны администратора. Patch-версии устанавливаются автоматически. В случае необходимости можно включить [подтверждение потенционально опасных обновлений](#подтверждение-потенциально-опасных-обновлений) или [отключить обновления в кластере полностью](#отключение-механизма-обновлений).

Существуют два режима минорных обновлений:

1. [**Автоматический режим**](#автоматический-режим): минорные обновления применяются автоматически либо в заданные окна обновлений, либо сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html).

2. [**Ручной режим**](#ручной-режим): каждое минорное обновление релиза DKP нужно подтверждать вручную.

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` DKP будет каждую минуту проверять данные о релизе на канале обновлений.

## Автоматический режим

При автоматическом режиме обновлений:

1. После появления нового релиза на выбранном канале обновлений DKP скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* DKP обновляет кластер на новую версию согласно установленным [параметрам обновлений](modules/002-deckhouse/configuration.html#parameters-update). По умолчанию — автоматически, сразу после появления обновления.

Чтобы посмотреть список и состояние всех релизов, выполните:

```shell
kubectl get deckhousereleases
```

## Подтверждение потенциально опасных обновлений

В DKP можно включить ручное подтверждение потенциально опасных (disruptive) обновлений в автоматическом режиме[для кластера](#для-кластера) или отдельно [для группы узлов](#для-группы-узлов). Тогда DKP будет проверять каждое обновление и просить ручное подтверждение для потенциально опасных.

### Для кластера

DKP будет запрашивать подтверждение обновления, если оно меняет значения по умолчанию или поведение модулей.

Чтобы включить подтверждение, добавьте параметр `disruptionApprovalMode: Manual` в *ModuleConfig*:

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

Подтвердите минорное обновления DKP, например, до версии `v1.58.8`:

```shell
kubectl patch DeckhouseRelease v1.58.8 --type=merge -p='{"approved": true}'
```

### Для группы узлов

DKP будет запрашивать подтверждение обновления, если оно требует прерывание работы узла.

Чтобы включить подтверждение, добавьте параметр `approvalMode: Manual` в *NodeGroup*:

```yaml
# NodeGroup for cloud nodes in AWS.
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
...
  disruptions:
    approvalMode: Manual
```

Подтвердите обновление узла, установив аннотацию `update.node.deckhouse.io/disruption-approved: ""` на узел:

```shell
kubectl annotate node <имя узла> update.node.deckhouse.io/disruption-approved=
```

## Ручной режим

В ручном режиме нужно подтверждать каждое минорное обновление DKP.

Чтобы включить ручной режим, в ресурсе *ModuleConfig* с именем `deckhouse` установите параметр `spec.update.mode: Manual`:

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

Подтвердите минорное обновления DKP, например, до версии `v1.58.8`:

```shell
kubectl patch DeckhouseRelease v1.58.8 --type=merge -p='{"approved": true}'
```

<!-- **Срочное обновление**???

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
``` -->

## Отключение механизма обновлений

Механизм обновления DKP можно отключить полностью. Для этого удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [`releaseChannel`](modules/002-deckhouse/configuration.html#parameters-releasechannel). В этом случае Deckhouse не проверяет обновления, и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Полное отключение обновлений может привести к сбоям в работе кластера. Обновления на patch-релизы содержат исправления криических уязвимостей и ошибок.
{% endalert %}