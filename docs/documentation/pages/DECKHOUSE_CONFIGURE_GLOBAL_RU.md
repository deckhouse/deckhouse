---
title: "Глобальная конфигурация"
permalink: ru/deckhouse-configure-global.html
lang: ru
---

## Что нужно настроить?

Желательно настроить `modules.publicDomainTemplate`.

```yaml
global: |
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

## Параметры

{{ site.data.schemas.global.config-values | format_configuration }}
