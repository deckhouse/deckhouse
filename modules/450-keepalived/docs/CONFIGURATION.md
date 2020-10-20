---
title: "Модуль keepalived: конфигурация"
---

Модуль по умолчанию **выключен**. Для включения добавьте в CM `deckhouse`:

```yaml
data:
  keepalivedEnabled: "true"
```

Параметров в ConfigMap `deckhouse` **нет**.

Настройка keepalived-кластеров выполняется с помощью [Custom Resource](cr.html).
