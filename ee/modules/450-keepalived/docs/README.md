---
title: "The keepalived module"
description: "Management of keepalived clusters on Deckhouse Kubernetes Platform nodes."
---

{% alert level="warning" %}
This module does not work with the <a href="../cni-cilium/">cilium</a> module.
{% endalert %}

This module (managed via custom resources) is intended for configuring keepalived clusters on nodes.
