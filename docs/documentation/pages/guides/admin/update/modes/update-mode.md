---
title: Режимы обновлений Deckhouse Kubernetes Platform
permalink: ru/update/modes/update-mode/
lang: ru
---

{% alert level="info" %}
Модуль `deckhouse` обеспечивает механизмы обновления платформы. Убедитесь, что он включен.
{% endalert %}

Для кластера можно определить режим обновлений минорных версий платформы, тогда Deckhouse Kubernetes Platform (DKP) либо будет обновлять кластер, либо будет ждать подтверждения обновления со стороны администратора. Patch-версии устанавливаются автоматически.

Существуют два режима минорных обновлений:

1. [**Автоматический режим**](#автоматический-режим): минорные обновления применяются автоматически либо в заданные окна обновлений, либо сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html). В случае необходимости можно включить в автоматическом режиме [подтверждение потенционально опасных обновлений](#подтверждение-потенциально-опасных-обновлений).

2. [**Ручной режим**](#ручной-режим): каждое минорное обновление релиза DKP нужно подтверждать вручную.

Если в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` указан параметр [`releaseChannel`](modules/002-deckhouse/configuration.html#parameters-releasechannel), то DKP будет каждую минуту проверять данные о релизе на канале обновлений. При желании можно отключить обновления в кластере, удалив из модуля `deckhouse` параметр `releaseChannel`. В этом случае Deckhouse не будет проверять и скачивать обновления, и даже обновления на patch-релизы не будут выполняться.

{% alert level="danger" %}
Полное отключение обновлений может привести к сбоям в работе кластера. Обновления на patch-релизы содержат исправления криических уязвимостей и ошибок.
{% endalert %}

## Автоматический режим

1. После появления нового релиза на выбранном [канале обновлений](../channels-and-windows/) DKP скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* DKP обновляет кластер на новую версию согласно установленным [параметрам обновлений](modules/002-deckhouse/configuration.html#parameters-update). По умолчанию — автоматически, сразу после появления обновления.

Чтобы посмотреть список и состояние всех релизов, выполните:

```shell
kubectl get deckhousereleases
```

### Подтверждение потенциально опасных обновлений

В DKP можно в автоматическом режиме включить ручное подтверждение потенциально опасных (disruptive) обновлений [для кластера](#для-кластера) или [для группы узлов](#для-группы-узлов).

#### Для кластера

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

#### Для группы узлов

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

В ручном режиме каждое минорное обновление DKP требует подтверждения администратора.

Чтобы включить ручной режим, в ресурсе *ModuleConfig* с именем `deckhouse` установите параметр `update.mode: Manual`:

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
