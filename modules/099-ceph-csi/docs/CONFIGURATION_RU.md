---
title: "Модуль ceph-csi: настройки"
---

Модуль не требует конфигурации и **выключен** по умолчанию. Для включения добавьте в ConfigMap `deckhouse`:

```yaml
data:
  cephCsiEnabled: "true"
```
