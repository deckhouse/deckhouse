---
title: "The ingress-nginx module: configuration"
---

> Pay attention to the global parameter [publicDomainTemplate](../../deckhouse-configure-global.html#parameters), if you are turning the module on. If the parameter is not specified, the Ingress resources for Deckhouse service components (dashboard, user-auth, grafana, upmeter, etc.) will not be created.

Ingress controllers are configured using the [IngressNginxController](cr.html#ingressnginxcontroller) Custom Resource.

<!-- SCHEMA -->
