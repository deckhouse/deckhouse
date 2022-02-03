---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

## What do I need to configure?

We recommend specifying template of the DNS records in the [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter and configuring DNS accordingly.

Deckhouse creates Ingress resources for some modules based on the `publicDomainTemplate` parameter. Monitoring, authentication, and other Deckhouse functionality may not work without specifying the `publicDomainTemplate` parameter and DNS configuration.

> You can use the nip.io service (or similar) for testing if wildcard DNS records are unavailable to you for some reason.
> 
> Pay attention to some [configuration nuance](./#example-of-the-deckhouse-configmap). 

The example of specifying a DNS template in the `publicDomainTemplate` parameter:

```yaml
global: |
  modules:
    publicDomainTemplate: "%s.kube.company.my"
```

## Parameters

{{ site.data.schemas.global.config-values | format_configuration }}
