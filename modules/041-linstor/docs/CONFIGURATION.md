---
title: "The linstor module: configuration"
force_searchable: true
description: Settings of the linstor Deckhouse module.
---

{% alert level="danger" %}
This version of the module is deprecated and is no longer supported. Use the [sds-drbd](https://deckhouse.io/modules/sds-drbd/beta/) module instead.
{% endalert %}

{% alert level="warning" %}
The module is guaranteed to work only in the following cases:
- when using the stock kernels that come with [supported distributions](../../supported_versions.html#linux);
- when using a 10 Gbps network.

In all other cases, the module may work, but its full functionality is not guaranteed.
{% endalert %}

<!-- SCHEMA -->
