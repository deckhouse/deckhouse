---
title: Режимы обновлений
permalink: ru/update/update-mode/
lang: ru
---

Режим обновления минорных версий Deckhouse (обновление релиза). Не влияет на обновление patch-версий (patch-релизов).

Существуют два режима обновления:

1. **Auto (автоматический режим)** — все обновления применяются автоматически.

Обновления минорной версии релиза Deckhouse Kubernetes Platform применяются с учетом заданных [окон обновлений](ссылк) либо, если окна обновлений не заданы, по мере появления обновлений на соответствующем канале обновлений. Автоматический режим выставляется, как  **Окна обновлений не заданы** - кластер обновится сразу после появления новой версии на соответствующем [канале обновлений](https://deckhouse.ru/documentation/deckhouse-release-channels.html) и **Заданные окна обновлений** - кластер обновится в ближайшее доступное окно после появления новой версии на канале обновлений.

2. **Manual (ручной режим)** — для обновления минорной версии релиза Deckhouse Kubernetes Platform в ручном режиме необходимо [ручное подтверждение](modules/002-deckhouse/usage.html#ручное-подтверждение-обновлений).

Чтобы подтвердить обновления в соответствующем кастомном ресурсе *DeckhouseRelease* установите поле `approved` в `true`.

**Отключение обновления**

Чтобы полностью отключить механизм обновления Deckhouse Kubernetes Platform, удалите в [конфигурации](modules/002-deckhouse/configuration.html) модуля `deckhouse` параметр [releaseChannel](modules/002-deckhouse/configuration.html#parameters-releasechannel).

В этом случае Deckhouse не проверяет обновления и даже обновление на patch-релизы не выполняется.

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

**Подтверждение потенциально опасных (disruptive) обновлений**

При необходимости возможно включить подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

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

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление Deckhouse (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления Deckhouse `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true