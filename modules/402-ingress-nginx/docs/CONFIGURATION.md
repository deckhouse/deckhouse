---
title: "The ingress-nginx module: configuration"
---

{% alert level="info" %}
Pay attention to the global parameter [publicDomainTemplate](/products/kubernetes-platform/documentation/v1/reference/api/global.html#parameters), if you are turning the module on. If the parameter is not specified, the Ingress resources for DKP service components (dashboard, user-auth, grafana, upmeter, etc.) will not be created.
{% endalert %}

Ingress controllers are configured using the [IngressNginxController](cr.html#ingressnginxcontroller) Custom Resource.

<!-- SCHEMA -->
