---
title: Автоматический режим обновлений
permalink: ru/update/modes/automatic-mode/
lang: ru
---

{% alert level="danger" %}
Крайне не рекомендуется отключать автоматическое обновление! Это заблокирует обновления на patch-релизы, которые могут содержать исправления критических уязвимостей и ошибок.
{% endalert %}

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse Kubernetes Platform каждую минуту будет проверять данные о релизе на канале обновлений.

1. При появлении нового релиза Deckhouse Kubernetes Platform скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* в кластере Deckhouse Kubernetes Platform выполняет обновление на соответствующую версию согласно установленным [параметрам обновления](modules/002-deckhouse/configuration.html#parameters-update) (по умолчанию — автоматически, в любое время).

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

> Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен. Patch-версии, например, обновления с `1.29.1` до `1.29.2` устанавливаются без подтверждения и без учета окон обновлений.

### Подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно в в кастомном ресурсе [NodeGroup](../040-node-manager/cr.html#nodegroup) (параметр `disruptions.automatic.windows`), следующим образом:

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
```
