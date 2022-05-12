---
title: "Модуль keepalived: настройки"
---

Модуль по умолчанию **выключен**. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  keepalivedEnabled: "true"
```

Параметров в ConfigMap `deckhouse` **нет**.

Настройка keepalived-кластеров выполняется с помощью [Custom Resource](cr.html).
