---
title: "Installing: configuration"
permalink: en/installing/configuration.html
---

Reference of the resources used during [Deckhouse installation](./).

{% alert level="danger" %}
We do not recommend changing the parameters `internalNetworkCIDRs`, `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` in a running cluster. It is recommended to create a new cluster to change these parameters.
{% endalert %}

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
