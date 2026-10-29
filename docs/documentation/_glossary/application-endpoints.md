---
title: Application endpoints
description: The addresses of the Ingresses of an application that Deckhouse Platform publishes in the `status.urls` field of the [Application](reference/api/cr.html#application). The web interface shows them as links to the application.
lang: en
---

To publish an endpoint, the package adds the `packages.deckhouse.io/application-endpoint-description` annotation to the Ingress: its value is the endpoint description, `"true"` adds the endpoint without a description, `"false"` excludes the Ingress. Deckhouse Platform builds a URL for every `host` and `path` pair of the Ingress rules, with the `https` scheme if the host is listed in `spec.tls`, and updates `status.urls` of the [application instance](glossary.html#application-instance) after every successful apply of the templates.

For details, see [Templates](architecture/marketplace/templates.html).
