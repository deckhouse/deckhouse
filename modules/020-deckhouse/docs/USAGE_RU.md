---
title: "Модуль deckhouse: примеры конфигурации"
---

### Пример конфигурации модуля

```yaml
deckhouse: |
  logLevel: Debug
  bundle: Minimal
  releaseChannel: RockSolid
```

### Конфигурация окон обновлений

Обновление каждый день с 8:00 до 15:00 и с 20:00 до 23:00
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

Обновление по вторникам и субботам с 13:00 до 18:30
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

Если окна обновлений не заданы - Deckhouse обновиться, как только новый релиз станет доступен

---
Ручное подтверждение обновлений
```yaml
deckhouse: |
  ...
  releaseChannel: Stable
  update:
    mode: Manual
```

После этого нужно будет потверждать каждое обновление (кроме patch) Deckhouse. Например:
```sh
kubectl patch DeckhouseRelease v1-25-0 --type=merge -p='{"approved": true}'
```

*Внимание* Patch-версии (1.25.1, 1.25.2, 1.25.3, и т.д.) устанавливаются без потверждения и вне окон обновлений
