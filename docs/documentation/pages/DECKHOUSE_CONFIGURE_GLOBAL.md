---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

The global Deckhouse settings are stored in the `ModuleConfig/global` resource (see [Deckhouse configuration](./#deckhouse-configuration)).

> The [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter defines the DNS names template some Deckhouse modules use to create Ingress resources.
>
> You can use the [sslip.io](https://sslip.io/) service (or similar) for testing if wildcard DNS records are unavailable to you for some reason.

## Parameters

{{ site.data.schemas.global.config-values | format_module_configuration: "global" }}
