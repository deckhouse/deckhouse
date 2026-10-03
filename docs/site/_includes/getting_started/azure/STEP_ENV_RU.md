{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-azure/environment.html) модуля `cloud-provider-azure`.

Для управления ресурсами в Microsoft Azure нужны учётная запись Azure и хотя бы одна привязанная [подписка (Subscription)](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

Создайте сервисный аккаунт для Deckhouse Platform. Выполните следующие команды на **персональном компьютере** с помощью Azure CLI.

Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli). Авторизуйтесь в Azure и сохраните идентификатор подписки в переменную окружения `SUBSCRIPTION_ID`:

```shell
export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
```

Команда сохраняет идентификатор первой подписки учётной записи. Если подписок несколько, укажите в переменной идентификатор нужной.

Создайте сервисный аккаунт:

```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```

Значения из вывода команды и идентификатор подписки укажите в секции `provider` конфигурации:

- `appId` — в параметре `clientId`;
- `password` — в параметре `clientSecret`;
- `tenant` — в параметре `tenantId`;
- значение переменной `SUBSCRIPTION_ID` — в параметре `subscriptionId`.

Секрет в параметре [clientSecret](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) действует один год и не продлевается автоматически. Чтобы задать больший срок действия, добавьте к команде `az ad sp create-for-rbac` флаг `--years`. Флаг описан [в справочнике Azure CLI](https://learn.microsoft.com/en-us/cli/azure/ad/sp#az-ad-sp-create-for-rbac).
