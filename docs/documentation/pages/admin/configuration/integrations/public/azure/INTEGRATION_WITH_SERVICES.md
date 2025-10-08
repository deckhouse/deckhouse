---
title: Integration with Microsoft Azure services
permalink: en/admin/integrations/public/azure/services.html
---

Deckhouse Kubernetes Platform (DKP) leverages Azure cloud features for full integration with Kubernetes.
When running a cluster on Azure, it automatically:

- Creates network routes for the PodNetwork.
- Configures external LoadBalancers for Kubernetes Service objects.
- Removes nodes from the cluster that no longer exist in the cloud.
- Updates node metadata to reflect the current configuration.
- Provisions disks for nodes via CSI.
- Connects the required CNI network (a simple bridge is used).
- Makes [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass) definitions available for virtual machines, which can be used later in [NodeGroup](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) configurations.

{% alert level="info" %}
All outgoing traffic from the cluster comes through LoadBalancers.
If no LoadBalancer is configured to handle UDP traffic, all outgoing UDP traffic will be blocked,
which can affect NTP utilities such as `ntpdate`, `chrony`, etc.
To resolve this, manually add a rule for any UDP port to an existing LoadBalancer or create a Kubernetes Service of the LoadBalancer type with a UDP port.
{% endalert %}

## Support for Service Endpoints

DKP supports connections to Azure services via Service Endpoints.
These endpoints:

- Allow access to Azure services without using public IP addresses.
- Route traffic over optimized Azure backbone infrastructure.
- Simplify access control and improve security.

List of supported Service Endpoints:

```console
Microsoft.AzureActiveDirectory
Microsoft.AzureCosmosDB
Microsoft.ContainerRegistry
Microsoft.CognitiveServices
Microsoft.EventHub
Microsoft.KeyVault
Microsoft.ServiceBus
Microsoft.Sql
Microsoft.Storage
Microsoft.Storage.Global
Microsoft.Web
```

Specify the required services using the [`serviceEndpoints`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-serviceendpoints) parameter in the AzureClusterConfiguration object.
