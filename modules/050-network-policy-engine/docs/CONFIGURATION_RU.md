---
title: "Модуль network-policy-engine: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  networkPolicyEngineEnabled: "true"
```

