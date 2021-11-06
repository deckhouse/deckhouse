You have to create a service account with {{ page.platform_name[page.lang] }} so that Deckhouse Platform can manage cloud resources. The detailed instructions for creating a service account with Microsoft Azure are available in the [documentation](/en/documentation/v1/modules/030-cloud-provider-azure/environment.html).

Below, we will provide a brief overview of the necessary actions (run them on the **[personal computer](step2.html#installation-process)**).

Install the [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) and perform a `login`.

Export the environment variable by substituting the subscription ID instead of the `my-subscription-id`:
{% snippetcut %}
```shell
export SUBSCRIPTION_ID="my-subscription-id"
```
{% endsnippetcut %}

Create a service account:
{% snippetcut %}
```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
{% endsnippetcut %}
