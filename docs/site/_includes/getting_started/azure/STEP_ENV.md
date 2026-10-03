{%- include getting_started/global/partials/NOTICES.liquid %}
{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Prepare the {{ page.platform_name[page.lang] }} environment so that Deckhouse Platform can manage cloud resources. The full procedure is described on the [environment preparation page](/modules/cloud-provider-azure/environment.html) of the `cloud-provider-azure` module.

To manage resources in Microsoft Azure, you need an Azure account and at least one [subscription](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

Create a service account for Deckhouse Platform. Run the following commands on the **personal computer** using the Azure CLI.

Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli). Log in to Azure and save the subscription ID to the `SUBSCRIPTION_ID` environment variable:

```shell
export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
```

The command saves the ID of the first subscription of the account. If the account has several subscriptions, set the ID of the required one in the variable.

Create a service account:

```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```

Use the values from the command output and the subscription ID in the `provider` section of the configuration:

- `appId`: The `clientId` parameter.
- `password`: The `clientSecret` parameter.
- `tenant`: The `tenantId` parameter.
- The value of the `SUBSCRIPTION_ID` variable: The `subscriptionId` parameter.

The secret in the [clientSecret](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) parameter is valid for one year and is not renewed automatically. To set a longer validity period, add the `--years` flag to the `az ad sp create-for-rbac` command. The flag is described in the [Azure CLI reference](https://learn.microsoft.com/en-us/cli/azure/ad/sp#az-ad-sp-create-for-rbac).
