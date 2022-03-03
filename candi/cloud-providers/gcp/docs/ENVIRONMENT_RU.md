---
title: "Cloud provider — GCP: подготовка окружения"
---

Для того, чтобы Deckhouse смог управлять ресурсами в Google Cloud, в последнем необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна [в документации провайдера](https://cloud.google.com/iam/docs/service-accounts).

> **Внимание!** Созданный `service account key` невозможно восстановить, только удалить и создать новый.

Далее представлена краткая последовательность необходимых действий.

## Настройка через Google cloud console

Перейдите по [ссылке](https://console.cloud.google.com/iam-admin/serviceaccounts), выберите проект и создайте новый сервисный аккаунт (также можно выбрать уже существующий).

Аккаунту должны быть присвоены несколько необходимых ролей:
```
Compute Admin
Service Account User
Network Management Admin
```

Роли можно присвоить на этапе создания сервисного аккаунта, либо изменить [на этой странице](https://console.cloud.google.com/iam-admin/iam).

Чтобы получить `service account key` в JSON-формате, [на странице](https://console.cloud.google.com/iam-admin/serviceaccounts) в колонке Actions нажмите  на три вертикальные точки и выберите `Manage keys`. Затем нажмите `Add key` -> `Create new key` -> `Key type` -> `JSON`.

## Настройка через gcloud CLI

Для настройки через интерфейс командной строки выполните следующие шаги:

1\. Экспортируйте переменные окружения:

```shell
export PROJECT=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```
2\. Выберите project:

```shell
gcloud config set project $PROJECT
```
3\. Создайте service account:

```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```
4\. Присвойте роли созданному service account:

```shell
for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do gcloud projects add-iam-policy-binding ${PROJECT} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com --role=${role}; done
```
    
Список необходимых ролей:
```
roles/compute.admin
roles/iam.serviceAccountUser
roles/networkmanagement.admin
```

5\. Выполните проверку ролей service account:

```shell
gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' \
      --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
```
6\. Создайте `service account key`:

```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com \
      ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
```
