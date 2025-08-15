---
title: "Module registry: configuration"
description: ""
---

{% alert level="warning" %}
{% endalert %}

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

To configure parameters for working with the container registry, use the [`registry`](../deckhouse/configuration.html#parameters-registry) section of the `deckhouse` module configuration.

It specifies:

- Mode for accessing the container registry with Deckhouse images.
- Parameters for the `Direct` access mode:
  - Root CA certificate.
  - Address of the container registry repository.
  - License key for accessing the container registry.
  - Password for authenticating with the container registry.
  - Protocol to use for connecting to the registry.
  - Username for authenticating with the container registry.
