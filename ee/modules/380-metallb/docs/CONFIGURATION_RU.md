---
title: "Модуль metallb: настройки"
---


Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  metallbEnabled: "true"
```

## Параметры

<!-- SCHEMA -->
