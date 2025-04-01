---
title: "Ручное подтверждение обновлений"
permalink: ru/virtualization-platform/documentation/admin/update/manual-update-mode.html
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

### Ручное подтверждение обновлений с потенциальным прерыванием трафика (disruption updates)

При необходимости возможно включить ручное подтверждение обновлений с потенциальным прерыванием трафика (disruption updates), которые меняют значения по умолчанию или поведение некоторых модулей:

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

В этом режиме необходимо подтверждать каждое минорное обновление с потенциальным прерыванием трафика (disruption update) платформы (без учета patch-версий) с помощью аннотации `release.deckhouse.io/disruption-approved=true` на соответствующем ресурсе [DeckhouseRelease](../../../reference/cr/deckhouserelease.html).

Пример подтверждения минорного обновления с потенциальным прерыванием трафика платформы `v1.36.4`:

```shell
d8 k annotate DeckhouseRelease v1.36.4 release.deckhouse.io/disruption-approved=true
```
