---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/azure/authorization.html
lang: ru
---

## Требования

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

Для корректной работы Deckhouse Kubernetes Platform (DKP) с Microsoft Azure необходимо:

- Убедиться, что используемый регион поддерживает Availability Zones.
- На всех виртуальных машинах должен быть установлен пакет `cloud-init`. После запуска ВМ должны быть активны службы:

  - `cloud-config.service`;
  - `cloud-final.service`;
  - `cloud-init.service`.

## Доступ к Azure API

Для управления ресурсами Azure из DKP требуется сервисный аккаунт с ролью `Contributor` в рамках нужной подписки. Выполните следующие шаги:

1. Установите Azure CLI и авторизуйтесь:

   ```shell
   export SUBSCRIPTION_ID=$(az login | jq -r '.[0].id')
   ```

1. Создайте сервисный аккаунт:

   ```shell
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

   Эти параметры необходимо указать в [объекте AzureClusterConfiguration](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration):

   | Поле           | Значение из вывода команды |
   |----------------|-----------------------------|
   | `clientId`     | `appId`                     |
   | `clientSecret` | `password`                  |
   | `tenantId`     | `tenant`                    |

1. Авторизуйтесь в Azure CLI под созданным сервисным аккаунтом:

   ```shell
   az login --service-principal -u <CLIENT_ID> -p <CLIENT_SECRET> --tenant <TENANT_ID>
   ```

{% alert level="info" %}
Срок действия `clientSecret` по умолчанию — 1 год. Автоматическое продление не поддерживается. Чтобы задать больший срок действия, воспользуйтесь [официальной документацией Azure](https://azure.microsoft.com/ru-ru/).
{% endalert %}
