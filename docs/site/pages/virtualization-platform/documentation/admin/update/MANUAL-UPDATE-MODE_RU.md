---
title: "Обновление платформы"
permalink: ru/virtualization-platform/documentation/admin/update/manual-update-mode.html
lang: ru
---

### Ручное подтверждение обновлений

При необходимости возможно включить ручное подтверждение обновлений. Сделать это можно следующим образом:

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
kubectl patch DeckhouseRelease v1.43.2 --type=merge -p='{"approved": true}'
```

### Ручное подтверждение потенциально опасных (disruptive) обновлений

При необходимости возможно включить ручное подтверждение потенциально опасных (disruptive) обновлений (которые меняют значения по умолчанию или поведение некоторых модулей). Сделать это можно следующим образом:

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

В этом режиме необходимо подтверждать каждое минорное потенциально опасное (disruptive) обновление платформы (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](todo).

Пример подтверждения минорного потенциально опасного обновления платформы `v1.36.4`:

```shell
kubectl annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```