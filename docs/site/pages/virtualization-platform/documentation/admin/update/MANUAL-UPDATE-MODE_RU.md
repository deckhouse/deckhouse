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
