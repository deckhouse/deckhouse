---
title: "Global configuration"
permalink: en/deckhouse-configure-global.html
---

The global Deckhouse settings are stored in the `global` parameter of the [Deckhouse configuration](./#deckhouse-configuration).

> The [publicDomainTemplate](#parameters-modules-publicdomaintemplate) parameter defines the template some Deckhouse modules use to create Ingress resources. To access them, you can either configure your DNS or add the DNS mappings locally (e.g., in the `/etc/hosts` file in Linux).
>
> You can use the nip.io service (or similar) for testing if wildcard DNS records are unavailable to you for some reason.
> Pay attention to some [nuances](./#deckhouse-configuration) of ConfigMap `deckhouse`.

## Parameters

{{ site.data.schemas.global.config-values | format_configuration }}
