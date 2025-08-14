---
title: "Module registry: configuration"
description: ""
---

{% alert level="warning" %}
{% endalert %}

{% include module-alerts.liquid %}

{% include module-bundle.liquid %}

To configure registry parameters, use the [`registry`](../deckhouse/configuration.html#parameters-registry) section of the `deckhouse` module configuration.

It specifies:

- The mode for accessing the container registry with Deckhouse images.
- Parameters for the `Direct` access mode:
  - The root CA certificate.
  - The address of the container registry repository.
  - The license key for accessing the container registry.
  - The password for authenticating with the container registry.
  - The protocol to use for connecting to the registry.
  - The username for authenticating with the container registry.
