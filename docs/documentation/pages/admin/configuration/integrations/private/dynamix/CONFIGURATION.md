---
title: Integration configuration in Basis Dynamix
permalink: en/admin/integrations/private/dynamix/configuration.html
lang: en
---

## Required parameters

- `layout`: [Layout](../../../../glossary.html#layout)
- `sshPublicKey`: Public key for accessing the nodes
- `location`: Name of the Basis Dynamix location where the cluster resource group is created
- `account`: Account name in the cloud
- `storagePolicy`: Name of the storage policy used by default for [virtual machine disks](../../../../glossary.html#vm-disk)
- `provider.controllerUrl`, `oAuth2Url`, `appId`, `appSecret`: API access parameters
- `imageName`: OS image name
- `externalNetwork`: External network name
- `nodeNetworkCIDR`: Address space of the internal network (only for the `StandardWithInternalNetwork` layout)

{% alert level="warning" %}
Changing `storagePolicy` recreates nodes of the `CloudEphemeral` type. The nodes that are recreated are listed in the [Changing the storage policy](load-balancing-and-storage.html#changing-the-storage-policy) section.
{% endalert %}

To apply the parameter changes, run [`dhctl converge`](../../../configuration/#modifying-cluster-configuration).

## DNS servers

If the external network doesn't provide DNS server addresses to the nodes, set them in the [`nameservers`](/modules/cloud-provider-dynamix/cluster_configuration.html#dynamixclusterconfiguration-nameservers) parameter.
