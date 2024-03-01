{%- include getting_started/global/partials/NOTICES.liquid %}
{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

To rule the Microsoft Azure cloud, you need an account and at least a single [Subscription connected to id](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

You have to create a service account with {{ page.platform_name[page.lang] }} so that Deckhouse Kubernetes Platform can manage cloud resources. The detailed instructions for creating a service account with Microsoft Azure are available in the [documentation](/documentation/v1/modules/030-cloud-provider-azure/environment.html). Below, we will provide a brief overview of the necessary actions (run them on the **personal computer**).

Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) and perform a `login`.

Export the environment variable by substituting the subscription ID instead of the `my-subscription-id`:
{% snippetcut %}
```shell
export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
```
{% endsnippetcut %}

Create a service account:

{% snippetcut %}
```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
{% endsnippetcut %}

At this step, a service account will be created, with a secret (used in the [clientSecret](/documentation/v1/modules/030-cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) parameter of the `AzureClusterConfiguration` resource) validity period of one year without automatic renewal. Refer to the [official documentation](https://learn.microsoft.com/en-us/azure/app-service/configure-ssl-app-service-certificate?tabs=portal#renew-an-app-service-certificate) to create a service account with a longer secret expiration date.
