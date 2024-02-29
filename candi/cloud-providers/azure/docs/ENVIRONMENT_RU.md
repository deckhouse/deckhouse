---
title: "Cloud provider — Azure: подготовка окружения"
description: "Настройка Azure для работы облачного провайдера Deckhouse."
---

> **Внимание!** Поддерживаются только [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region), в которых доступны `Availability Zones`.

Для управления облаком Microsoft Azure необходимо иметь соответствующую учетную запись и хотя бы одну привязанную [подписку (Subscription)](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

Для управления ресурсами в облаке Microsoft Azure средствами Deckhouse необходимо создать service account. Для этого:
1. Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli), авторизуйтесь и получите `Subscription ID`:

   ```shell
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

2. Создайте service account:

   ```shell
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
   ```

   > На этом этапе выдается [clientSecret](https://deckhouse.ru/documentation/v1/modules/030-cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-provider-clientsecret) по умолчанию на 1 год. Официальная документация для [обновления сертификата](https://learn.microsoft.com/en-us/azure/app-service/configure-ssl-app-service-certificate?tabs=portal#renew-an-app-service-certificate)

Для дальнейшей работы с утилитой `az` необходимо авторизоваться, используя данные (login, password, tenant) созданного service account:

```shell
az login --service-principal -u <username> -p <password> --tenant <tenant>
```
