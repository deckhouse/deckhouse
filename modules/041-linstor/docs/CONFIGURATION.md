---
title: "The linstor module: configuration"
force_searchable: true
description: Settings of the linstor Deckhouse module.
---

{% alert level="danger" %}
The current version of the module is outdated and is no longer supported. Switch to using the [sds-drbd](https://deckhouse.io/modules/sds-drbd/beta/) module.
{% endalert %}

{% alert level="warning" %}
The module is guaranteed to work only in the following cases:
- when using the stock kernels that come with [supported distributions](../../supported_versions.html#linux);
- when using a 10 Gbps network.

In all other cases, the module may work, but its full functionality is not guaranteed.
{% endalert %}

<!-- SCHEMA -->
