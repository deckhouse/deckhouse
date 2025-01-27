---
title: "Cloud provider — Azure: Preparing environment"
description: "Configuring Azure for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
**Caution!** Only regions where `Availability Zones` are available are supported.
{% endalert %}

To rule the Microsoft Azure cloud, you need an account and at least a single Subscription connected to id.

You have to create a service account with Microsoft Azure so that Deckhouse can manage cloud resources:
1. Install the Azure CLI, login and get Subscription ID:

   ```shell
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

2. Create the service account:

   ```shell
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
   ```

   Example output of the command:

   ```console
   {
     "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxx",     <-- used in the clientId parameter of the AzureClusterConfiguration resource 
     "displayName": "DeckhouseCANDI",
     "password": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", <-- used in the clientSecret parameter of the AzureClusterConfiguration resource
     "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"    <-- used in the tenantId parameter of the AzureClusterConfiguration resource
   }
   ```

   > By default, service account will be created with a secret (used in the [clientSecret](cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) parameter of the `AzureClusterConfiguration` resource) validity period of one year without automatic renewal. Refer to the official documentation to create a service account with a longer secret expiration date.

You have to be logged in for further work with the `az` tool. Use the service account username, password, and tenant to log in:

```shell
az login --service-principal -u <username> -p <password> --tenant <tenant>
```
