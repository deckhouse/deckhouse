---
title: "Cloud provider — GCP: подготовка окружения"
description: "Настройка GCP для работы облачного провайдера Deckhouse."
---

{% include notice_envinronment.liquid %}

Чтобы Deckhouse мог управлять ресурсами, в Google Cloud необходимо создать service account. Далее представлена краткая последовательность действий по созданию service account. Если вам необходима более подробная инструкция, вы можете найти ее [в документации провайдера](https://cloud.google.com/iam/docs/service-accounts).

{% alert level="warning" %}
**Внимание!** Созданный `service account key` невозможно восстановить, только удалить и создать новый.
{% endalert %}

{% alert level="warning" %}
Провайдер поддерживает работу только с одним диском в шаблоне виртуальной машины. Убедитесь, что шаблон содержит только один диск.
{% endalert %}

## Настройка через Google Cloud Console

Перейдите [по ссылке](https://console.cloud.google.com/iam-admin/serviceaccounts), выберите проект и создайте новый service account (также можно выбрать уже существующий).

Созданному service account'у должны быть присвоены несколько необходимых ролей:

```text
Compute Admin
Service Account User
Network Management Admin
```

Роли можно присвоить на этапе создания service account'а либо изменить [на этой странице](https://console.cloud.google.com/iam-admin/iam).

Чтобы получить `service account key` в JSON-формате, [на странице](https://console.cloud.google.com/iam-admin/serviceaccounts) в колонке Actions нажмите  на три вертикальные точки и выберите `Manage keys`. Затем нажмите `Add key` -> `Create new key` -> `Key type` -> `JSON`.

## Настройка через gcloud CLI

Для настройки через интерфейс командной строки выполните следующие шаги:

1. Экспортируйте переменные окружения:

   ```shell
   export PROJECT_ID=sandbox
   export SERVICE_ACCOUNT_NAME=deckhouse
   ```

2. Выберите project:

   ```shell
   gcloud config set project $PROJECT_ID
   ```

3. Создайте service account:

   ```shell
   gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
   ```

4. Присвойте роли созданному service account:

   ```shell
   for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin;
   do gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
      --role=${role}; done
   ```

   Список необходимых ролей:

   ```text
   roles/compute.admin
   roles/iam.serviceAccountUser
   roles/networkmanagement.admin
   ```

5. Выполните проверку ролей service account:

   ```shell
   gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' \
         --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
   ```

6. Создайте `service account key`:

   ```shell
   gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
         ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
   ```
