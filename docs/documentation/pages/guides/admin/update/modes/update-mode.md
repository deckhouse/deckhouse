---
title: Режимы обновлений
permalink: ru/update/modes/update-mode/
lang: ru
---

Patch-версии обновляются без простоя автоматически. Определите режим обновлений минорных версий Deckhouse — обновление релиза.

Существуют два режима минорных обновлений:

1. **Автоматический режим**: минорные обновления применяются автоматически.

  Обновления минорной версии релиза Deckhouse Kubernetes Platform (DKP) применяются с учетом заданных окон обновлений. Если окна обновлений не заданы, то кластер обновится сразу после появления новой версии на соответствующем канале обновлений.

2. **Ручной режим**: нужно подтверждать обновления минорной версии релиза DKP вручную.

  Чтобы подтвердить обновления, в соответствующем кастомном ресурсе *DeckhouseRelease* установите параметр`approved: true`.

## Как отключить обновления?

Чтобы полностью отключить механизм обновления DKP, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [`releaseChannel`](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае DKP не проверяет обновления. Обновление на patch-релизы также не выполняется.

{% alert level="danger" %}
Полное отключение обновлений может привести к сбоям в работе кластера. Обновления на patch-релизы содержат исправления криических уязвимостей и ошибок.
{% endalert %}

## Как включить подтверждение потенциально опасных (disruptive) обновлений?

Потенциально опасных (disruptive) обновления меняют значения по умолчанию или поведение некоторых модулей.

Чтобы включить подтверждение таких обновлений добавьте параметр `disruptionApprovalMode: Manual` в *ModuleConfig*:

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

В этом режиме вы подтверждаете каждое минорное потенциально опасное обновление DKP с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [*DeckhouseRelease*](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления DKP `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true