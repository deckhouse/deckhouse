---
title: "Node-manager module: custom resources"
---

{% alert level="info" %}
The [NodeConfig](#nodeconfig) resource describes a [Deckhouse Engine](/products/kubernetes-platform/documentation/v1/reference/deckhouse-engine.html) node. It is created automatically during installation. Only changes to the machine settings (`spec.network` and `spec.storage`) persist. The `internal.deckhouse.io/v1alpha1` API version and the NodeConfig schema may change between releases.
{% endalert %}

<!-- SCHEMA -->
