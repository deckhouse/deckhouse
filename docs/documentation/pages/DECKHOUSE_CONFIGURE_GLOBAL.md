---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
description: "Deckhouse Kubernetes Platform global settings."
---

The global Deckhouse settings are stored in the `ModuleConfig/global` resource (see [Deckhouse configuration](./#deckhouse-configuration)).

{% alert %}
The [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter specifies a DNS name template used by some Deckhouse modules to create Ingress resources.

If you don't have access to wildcard DNS records, you can use [sslip.io](https://sslip.io) or similar services for testing purposes.

The domain specified in the template must not match the domain set in the [clusterDomain](installing/configuration.html#clusterconfiguration-clusterdomain) parameter, nor the domain of the internal service network zone.  
For example, if `clusterDomain` is set to `cluster.local` and the internal zone is `central1.internal`, then publicDomainTemplate must not be `%s.cluster.local`.
{% endalert %}

Example of the `ModuleConfig/global`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: global
spec:
  version: 2
  settings: # <-- Module parameters from the "Parameters" section below.
    defaultClusterStorageClass: 'default-fast'
    modules:
      publicDomainTemplate: '%s.kube.company.my'
      resourcesRequests:
        controlPlane:
          cpu: 1000m
          memory: 500M
      placement:
        customTolerationKeys:
        - dedicated.example.com
      storageClass: 'default-fast'
```

## Parameters

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
