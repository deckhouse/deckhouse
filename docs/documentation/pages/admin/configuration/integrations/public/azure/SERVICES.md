---
title: Integration with Microsoft Azure services
permalink: en/admin/integrations/public/azure/services.html
---

## Support for Service Endpoints

Deckhouse Platform supports connections to Azure services via Service Endpoints.
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
