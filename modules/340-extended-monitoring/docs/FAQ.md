---
title: "The extended monitoring module: FAQ"
type:
  - instruction
search: extended monitoring, image-availability-exporter
---

{% raw %}

## How to switch to HTTP instead of HTTPS checks of my registry?

To change the protocol used for checking your container registry from HTTPS to HTTP, change the `settings.imageAvailability.registry.scheme` parameter in the module configuration.

For detailed instructions, please refer to the [module configuration documentation](./configuration.html#parameters-imageavailability-registry-scheme).

{% endraw %}
