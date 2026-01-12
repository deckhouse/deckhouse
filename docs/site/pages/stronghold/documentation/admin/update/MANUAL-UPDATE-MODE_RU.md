---
title: "Ручное подтверждение обновлений"
permalink: ru/stronghold/documentation/admin/update/manual-update-mode.html
lang: ru
---

Для ручного подтверждения обновлений установите этот режим в конфигурации:

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

В этом режиме необходимо подтверждать каждое минорное обновление платформы (без учета patch-версий).

Пример подтверждения обновления на версию `v1.43.2`:

```shell
d8 k patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений, которые меняют значения по умолчанию или поведение некоторых модулей:

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

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление платформы (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#deckhouserelease).

Пример подтверждения минорного потенциально опасного обновления платформы `v1.36.4`:

```shell
d8 k annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```
