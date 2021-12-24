---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

## What do I need to configure?

We recommend specifying the `modules.publicDomainTemplate` parameter.

```yaml
global: |
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

## Parameters

{{ site.data.schemas.global.config-values | format_configuration }}
