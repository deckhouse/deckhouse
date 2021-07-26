---
title: "Cloud provider — Azure: Preparing environment"
---

You have to create a service account with Microsoft Azure so that Deckhouse can manage cloud resources. The detailed instructions for creating a service account with Microsoft Azure are available in the provider's [documentation](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli). Below, we will provide a brief overview of the necessary actions:
- Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) and perform a `login`;
- Export the environment variable by substituting the Amazon AWS subscription ID instead of the `my-subscription-id`;
  ```shell
export SUBSCRIPTION_ID="my-subscription-id"
```
- Create a service account:
  ```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
