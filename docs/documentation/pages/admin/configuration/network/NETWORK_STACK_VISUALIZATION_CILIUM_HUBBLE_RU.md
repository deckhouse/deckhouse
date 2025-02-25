---
title: "Настройка визуализации сетевого стека кластера"
permalink: ru/admin/network/network-stack-visualization-cilium-hubble.html
lang: ru
---

Чтобы настроить визуализацию сетевого стека кластера, для отслеживания сетевых взаимодействий между подами, сервисами и внешними ресурсами, анализа сетевой активности и выявления проблемы с сетью, используйте модуль `cilium-hubble`.

Пример настройки визуализации сетевого стека:

```yaml
piVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cilium-hubble
spec:
  version: 2
  enabled: true
  settings:
    debugLogging: false
```

**Ссылка на кастом ресурс.**
