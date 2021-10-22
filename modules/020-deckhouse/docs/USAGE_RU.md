---
title: "Модуль deckhouse: примеры конфигурации"
---

## Пример конфигурации модуля

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

## Настройка режима обновления

Если в автоматическом режиме окна обновлений не заданы, Deckhouse обновится как только новый релиз станет доступен.

Patch-версии (например, обновления с `1.26.1` до `1.26.2`) устанавливаются без подтверждения и без учета окон обновлений.

### Конфигурация окон обновлений
Обновление каждый день с 8:00 до 15:00 и с 20:00 до 23:00:
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "8:00"
        to: "15:00"
      - from: "20:00"
        to: "23:00"
```

Обновление по вторникам и субботам с 13:00 до 18:30:
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    windows: 
      - from: "13:00"
        to: "18:30"
        days:
          - Tue
          - Sat
```

### Ручное подтверждение обновлений
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```
В этом режиме необходимо будет подтверждать каждое минорное обновление Deckhouse (без учёта patch-версий). 

Пример подтверждения обновления на версию `v1.26.0-alpha.6`:
```shell
kubectl patch DeckhouseRelease v1-26-0-alpha-6 --type=merge -p='{"approved": true}'
```
