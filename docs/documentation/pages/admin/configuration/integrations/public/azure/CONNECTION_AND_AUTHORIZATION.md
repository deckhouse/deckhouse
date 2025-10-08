---
title: Connection and authorization
permalink: en/admin/integrations/public/azure/authorization.html
description: "Configure Azure connection and authorization for Deckhouse Kubernetes Platform. Service principal setup, credentials configuration, and Azure integration requirements for cloud deployment."
---

## Requirements

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

To ensure Deckhouse Kubernetes Platform (DKP) works correctly with Microsoft Azure, the following conditions must be met:

- The selected region must support Availability Zones.
- All virtual machines must have the `cloud-init` package installed.
  After the VMs start, the following services must be active:
  - `cloud-config.service`
  - `cloud-final.service`
  - `cloud-init.service`

## Accessing the Azure API

To manage Azure resources from DKP, you need a service account with the `Contributor` role within the appropriate subscription.
Follow these steps:

1. Install Azure CLI and log in:

   ```shell
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

1. Create a service account:

   ```shell
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
   ```

   Example output:

   ```yaml
   {
    "appId": "<CLIENT_ID>",
    "displayName": "DeckhouseCANDI",
    "password": "<CLIENT_SECRET>",
    "tenant": "<TENANT_ID>"
   }
   ```

   Specify the output values in the [AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration) object:

   | Field           | Value from the output |
   |----------------|-----------------------------|
   | `clientId`     | `appId`                     |
   | `clientSecret` | `password`                  |
   | `tenantId`     | `tenant`                    |

1. Authenticate in Azure CLI using the created service account's credentials:

   ```shell
   az login --service-principal -u <CLIENT_ID> -p <CLIENT_SECRET> --tenant <TENANT_ID>
   ```

{% alert level="info" %}
By default, the `clientSecret` is valid for 1 year, and no automatic renewal is supported.
To set a longer expiration period, refer to the [official Azure documentation](https://azure.microsoft.com/en-us/).
{% endalert %}
