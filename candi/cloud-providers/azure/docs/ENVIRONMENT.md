---
title: "Cloud provider — Azure: Preparing environment"
---

> **Caution!** Only [regions](https://docs.microsoft.com/en-us/azure/availability-zones/az-region) where `Availability Zones` are available are supported.

To rule the Microsoft Azure cloud, you need an account and at least a single [Subscription connected to id](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

You have to create a service account with Microsoft Azure so that Deckhouse can manage cloud resources:
1. Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli), login and get Subscription ID:

  ```shell
  export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
  ```

2. Create the service account:

  ```shell
  az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
  ```

You have to be logged in for further work with the `az` tool. Use the service account username, password, and tenant to log in:

```shell
az login --service-principal -u <username> -p <password> --tenant <tenant>
```
