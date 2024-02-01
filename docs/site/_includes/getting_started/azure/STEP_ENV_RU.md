{%- include getting_started/global/partials/NOTICES.liquid %}
{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Для управления облаком Microsoft Azure необходимо иметь соответствующую учётную запись и хотя бы одну привязанную [подписку (Subscription)](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/documentation/v1/modules/030-cloud-provider-azure/environment.html). Далее представлена краткая последовательность действий (выполняйте их на **персональном компьютере**), которую необходимо выполнить с помощью консольной утилиты Azure CLI.

Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) и выполните `login`.

Экспортируйте переменную окружения, подставив вместо значения `my-subscription-id` идентификатор подписки:
{% snippetcut %}
```shell
export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
```
{% endsnippetcut %}

Создайте service account:
{% snippetcut %}
```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
{% endsnippetcut %}
