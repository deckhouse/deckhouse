---
title: "Cloud provider — Azure: подготовка окружения"
---

> **Внимание!** Поддерживаются только [регионы](https://docs.microsoft.com/ru-ru/azure/availability-zones/az-region) в которых доступны `Availability Zones`.

Чтобы Deckhouse смог управлять ресурсами в облаке Microsoft Azure, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://docs.microsoft.com/en-us/cli/azure/create-an-azure-service-principal-azure-cli). Ниже представлена краткая последовательность действий, которую необходимо выполнить с помощью консольной утилиты Azure CLI:
- Установите [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli) и выполните `login`;
- Экспортируйте переменную окружения, подставив вместо значения `my-subscription-id` идентификатор подписки Microsoft Azure:
  ```shell
export SUBSCRIPTION_ID="my-subscription-id"
```
- Создайте service account, выполнив команду:
  ```shell
az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "account_name"
```
