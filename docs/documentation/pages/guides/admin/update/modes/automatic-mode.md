---
title: Автоматический режим обновлений
permalink: ru/update/modes/automatic-mode/
lang: ru
---

При указании в конфигурации модуля `deckhouse` параметра `releaseChannel` Deckhouse Kubernetes Platform будет каждую минуту проверять данные о релизе на канале обновлений.

{% alert %}
Patch-релизы, например, обновление на версию `1.30.2` при установленной версии `1.30.1`, устанавливаются без учета режима и окон обновления, то есть при появлении на канале обновления patch-релиза - он всегда будет установлен.
{% endalert %}

При автоматическом режиме обновления:

1. При появлении нового релиза DKP скачает его в кластер и создаст кастомный ресурс [*DeckhouseRelease*](modules/002-deckhouse/cr.html#deckhouserelease).

2. После появления кастомного ресурса *DeckhouseRelease* в кластере DKP выполняет обновление на соответствующую версию согласно установленным [параметрам обновлений](modules/002-deckhouse/configuration.html#parameters-update). По умолчанию — автоматически, в любое время.

Чтобы посмотреть список и состояние всех релизов, воспользуйтесь командной:

```shell
kubectl get deckhousereleases
```

Если в автоматическом режиме окна обновлений не заданы, Deckhouse Kubernetes Platform обновится сразу, как только новый релиз станет доступен.

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

В этом режиме вы подтверждаете каждое минорное потенциально опасное обновление DKP с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [*DeckhouseRelease*](cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления DKP `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```
