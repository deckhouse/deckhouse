---
title: "Модуль istio: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  istioEnabled: "true"
```

## Параметры

<!-- SCHEMA -->
