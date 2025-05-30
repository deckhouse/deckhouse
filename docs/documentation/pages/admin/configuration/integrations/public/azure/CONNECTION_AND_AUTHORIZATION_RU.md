---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/azure/azure-authorization.html
lang: ru
---

## Требования

Для корректной работы Deckhouse с Microsoft Azure необходимо:

- Убедиться, что используемый регион поддерживает Availability Zones.
- На всех виртуальных машинах должен быть установлен пакет `cloud-init`. После запуска ВМ должны быть активны службы:

  - `cloud-config.service`;
  - `cloud-final.service`;
  - `cloud-init.service`.

## Доступ к Azure API

Для управления ресурсами Azure из Deckhouse требуется сервисный аккаунт с ролью `Contributor` в рамках нужной подписки. Выполните следующие шаги:

1. Установите Azure CLI и авторизуйтесь:

   ```console
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

1. Создайте сервисный аккаунт:

   ```console
   az ad sp create-for-rbac --role="Contributor" --scopes="/subscriptions/$SUBSCRIPTION_ID" --name "DeckhouseCANDI"
   ```

   Пример вывода команды:

   ```yaml
   {
    "appId": "<CLIENT_ID>",
    "displayName": "DeckhouseCANDI",
    "password": "<CLIENT_SECRET>",
    "tenant": "<TENANT_ID>"
   }
   ```

   Эти параметры необходимо указать в объекте AzureClusterConfiguration:

   | Поле           | Значение из вывода команды |
   |----------------|-----------------------------|
   | `clientId`     | `appId`                     |
   | `clientSecret` | `password`                  |
   | `tenantId`     | `tenant`                    |

1. Авторизуйтесь в Azure CLI под созданным сервисным аккаунтом:

   ```console
   az login --service-principal -u <CLIENT_ID> -p <CLIENT_SECRET> --tenant <TENANT_ID>
   ```

> Срок действия `clientSecret` по умолчанию — 1 год. Автоматическое продление не поддерживается. Чтобы задать больший срок действия, воспользуйтесь официальной документацией Azure.
