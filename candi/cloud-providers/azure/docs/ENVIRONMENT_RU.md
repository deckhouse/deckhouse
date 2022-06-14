---
title: "Cloud provider — Azure: подготовка окружения"
---

> **Внимание!** Поддерживаются только [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region), в которых доступны `Availability Zones`.

Для управления облаком Microsoft Azure необходимо иметь соответствующую учётную запись и хотя бы одну привязанную [подписку (Subscription)](https://docs.microsoft.com/en-us/azure/cost-management-billing/manage/create-subscription).

Для управления ресурсами в облаке Microsoft Azure средствами Deckhouse необходимо создать service account. Для этого:
1. Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli), авторизуйтесь и получите `Subscription ID`:

  ```shell
  export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
  ```

2. Создайте service account:

  ```shell
  az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
  ```

Для дальнейшей работы с утилитой `az` необходимо авторизоваться, используя данные (login, password, tenant) созданного service account:

```shell
az login --service-principal -u <username> -p <password> --tenant <tenant>
```
