---
title: "Cloud provider — Azure: подготовка окружения"
description: "Настройка Azure для работы облачного провайдера Deckhouse."
---

{% include notice_envinronment.liquid %}

{% alert level="warning" %}
**Внимание!** Поддерживаются только регионы, в которых доступны `Availability Zones`.
{% endalert %}

Для управления облаком Microsoft Azure необходимо иметь соответствующую учетную запись и хотя бы одну привязанную подписку (Subscription).

Для управления ресурсами в облаке Microsoft Azure средствами Deckhouse необходимо создать service account. Для этого:
1. Установите Azure CLI, авторизуйтесь и получите `Subscription ID`:

   ```shell
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

2. Создайте service account:

   ```shell
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
   ```

   Пример вывода команды:

   ```console
   {
     "appId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxx",     <-- используется в параметре clientId ресурса AzureClusterConfiguration 
     "displayName": "DeckhouseCANDI",
     "password": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", <-- используется в параметре clientSecret ресурса AzureClusterConfiguration
     "tenant": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"    <-- используется в параметре tenantId ресурса AzureClusterConfiguration
   }
   ```

   > По умолчанию срок действия секрета созданного service account (используется в параметре [clientSecret](cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) ресурса `AzureClusterConfiguration`) — один год без автоматического продления. Чтобы создать service account с большим сроком действия секрета обратитесь к официальной документации.

Для дальнейшей работы с утилитой `az` необходимо авторизоваться, используя данные (login, password, tenant) созданного service account:

```shell
az login --service-principal -u <username> -p <password> --tenant <tenant>
```
