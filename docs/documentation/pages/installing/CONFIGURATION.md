---
title: "Installing: configuration"
permalink: en/installing/configuration.html
---

Reference of the resources used during [Deckhouse installation](./).

{% alert level="danger" %}
Do not change the parameters `internalNetworkCIDRs`, `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` in a running cluster. Deploy a new cluster, if you have to change these parameters.
{% endalert %}

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
