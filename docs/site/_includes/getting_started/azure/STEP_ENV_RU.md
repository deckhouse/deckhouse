Чтобы Deckhouse Platform смогла управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/ru/documentation/v1/modules/030-cloud-provider-azure/environment.html).

Далее представлена краткая последовательность действий, которую необходимо выполнить с помощью консольной утилиты Azure CLI.

Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) и выполните `login`.

Экспортируйте переменную окружения, подставив вместо значения `my-subscription-id` идентификатор подписки:
{% snippetcut %}
```shell
export SUBSCRIPTION_ID="my-subscription-id"
```
{% endsnippetcut %}

Создайте service account, выполнив команду:
{% snippetcut %}
```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
{% endsnippetcut %}
