---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

The global Deckhouse settings are stored in the `ModuleConfig/global` resource (see [Deckhouse configuration](./#deckhouse-configuration)).

{% alert %}
The [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter defines the DNS names template some Deckhouse modules use to create Ingress resources.

You can use the [sslip.io](https://sslip.io/) service (or similar) for testing if wildcard DNS records are unavailable to you for some reason.
{% endalert %}

Example of the `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 1
  settings: # <-- Module parameters from the "Parameters" section below.
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M      
      placement:
        customTolerationKeys:
        - dedicated.example.com
    storageClass: sc-fast
```

## Parameters

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
