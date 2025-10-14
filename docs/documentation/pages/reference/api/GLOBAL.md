---
title: "Global configuration"
permalink: en/reference/api/global.html
description: "Deckhouse Kubernetes Platform global settings."
module-kebab-name: global
---

Global configuration settings allow you to customize parameters that are used by default by all modules and components of the Deckhouse Platform Certified Security Edition. Some modules may override some of these parameters (this can be found in the settings section of the respective module's documentation).

The global configuration settings are stored in the ModuleConfig `global`.

{% alert %}
The [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter specifies a DNS name template used by some Deckhouse modules to create Ingress resources. If this parameter is not specified, Ingress resources will not be created.

If you don't have access to wildcard DNS records, you can use [sslip.io](https://sslip.io) or similar services for testing purposes.

The domain specified in the template must not match or be a subdomain of the domain specified in the [`clusterDomain`](/reference/api/cr.html#clusterconfiguration-clusterdomain) parameter. We do not recommend changing the `clusterDomain` value unless absolutely necessary.

For the template to function correctly, you must first configure DNS services both in the networks where the cluster nodes will be located and in the networks from which clients will access the platform’s service web interfaces.

If the template matches the domain of the node network, use only A records to assign addresses of the nodes’ Frontend interfaces to the platform’s service web interfaces.  
For example, if the nodes are registered under the `company.my` zone and the template is `%s.company.my`.
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

{% include module-conversion.liquid %}

## Parameters

{{ site.data.schemas.modules.global.config-values | format_module_configuration: "global" }}
