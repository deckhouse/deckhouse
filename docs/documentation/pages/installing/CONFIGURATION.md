---
title: "Installing: configuration"
permalink: en/installing/configuration.html
---

Reference of the resources used during [Deckhouse installation](./).

{% alert level="danger" %}
Do not change the `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` parameters in a running cluster. If you do need to change them, deploy a new cluster.
{% endalert %}

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
