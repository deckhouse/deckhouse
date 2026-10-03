{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Подготовьте окружение {{ page.platform_name[page.lang] }}, чтобы Deckhouse Platform мог управлять ресурсами в облаке. Полная инструкция приведена [на странице подготовки окружения](/modules/cloud-provider-gcp/environment.html) модуля `cloud-provider-gcp`.

Создайте сервисный аккаунт для Deckhouse Platform. Выполните следующие команды на **персональном компьютере**.

{% alert %}
Список необходимых ролей:
- `roles/compute.admin`
- `roles/iam.serviceAccountUser`
- `roles/networkmanagement.admin`
{% endalert %}

Экспортируйте переменные окружения:

```shell
export PROJECT_ID=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```

Выберите проект:

```shell
gcloud config set project $PROJECT_ID
```

Создайте сервисный аккаунт:

```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```

Назначьте роли сервисному аккаунту:

```shell
for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role=${role}; done
```

Проверьте роли сервисного аккаунта:

```shell
gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```

Создайте ключ сервисного аккаунта:

```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
```
