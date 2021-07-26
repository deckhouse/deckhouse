---
title: "Cloud provider — GCP: подготовка окружения"
---

Чтобы Deckhouse смог управлять ресурсами в облаке Google, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://cloud.google.com/iam/docs/service-accounts). Далее представлена краткая последовательность необходимых действий:

> **Внимание!** `service account key` невозможно восстановить, только удалить и создать новый.

## Настройка через Google cloud console

Переходим по [ссылке](https://console.cloud.google.com/iam-admin/serviceaccounts) , выбираем проект и создаем новый сервис-аккаунт или выбираем существующий.

Список необходимых ролей:
```
Compute Admin
Service Account User
Network Management Admin
```

Роли можно прикрепить на этапе создания сервис-аккаунта, либо изменить на [странице](https://console.cloud.google.com/iam-admin/iam).

Чтобы получить `service account key` в JSON-формате, на [странице](https://console.cloud.google.com/iam-admin/serviceaccounts) в колонке Actions необходимо кликнуть на три вертикальные точки и выбрать Create key, тип ключа JSON.

## Настройка через gcloud CLI

Список необходимых ролей:
```
roles/compute.admin
roles/iam.serviceAccountUser
roles/networkmanagement.admin
```

- Экспортируйте переменные окружения:

  ```shell
  export PROJECT=sandbox
  export SERVICE_ACCOUNT_NAME=deckhouse
  ```
- Выберите project:

  ```shell
  gcloud config set project $PROJECT
  ```
- Создайте service account:

  ```shell
  gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
  ```
- Прикрепите роли к service account:

  ```shell
  for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do gcloud projects add-iam-policy-binding ${PROJECT} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com --role=${role}; done
  ```
- Выполните проверку ролей service account:

  ```shell
  gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' \
      --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
  ```
- Создайте service account key:

  ```shell
  gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com \
      ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
  ```
