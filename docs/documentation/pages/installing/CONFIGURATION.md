---
title: "Installing: configuration"
permalink: en/installing/configuration.html
---

Reference of the resources used during [Deckhouse installation](./).

{% alert level="warning" %}Attention!{% endalert %}

Any changes to the parameters `internalNetworkCIDRs`, `serviceSubnetCIDR`, `podSubnetNodeCIDRPrefix`, `podSubnetCIDR` in an operational cluster are not recommended, as they lead to destructive changes in the cluster. It is recommended to create a new cluster from scratch to modify these parameters.

To restore the cluster's functionality after such changes, it requires manual recovery and configuration updates of etcd, regeneration of all certificates, and restarting all pods, which results in significant downtime and may lead to data loss.

{{ site.data.schemas.global.cluster_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.init_configuration | format_cluster_configuration }}

{{ site.data.schemas.global.static_cluster_configuration | format_cluster_configuration }}
