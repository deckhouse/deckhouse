---
title: "Глобальная конфигурация"
permalink: ru/deckhouse-configure-global.html
lang: ru
---

## Что нужно настроить?

Первым делом рекомендуется настроить параметр `modules.publicDomainTemplate`:

```yaml
global: |
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

Подробнее про этот и другие параметры ниже.

## Параметры

{{ site.data.schemas.global.config-values | format_configuration }}
