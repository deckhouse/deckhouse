Чтобы **Deckhouse Platform {% if page.revision == 'ee' %}Enterprise Edition{% else %}Community Edition{% endif %}** смог управлять ресурсами в облаке Google, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации провайдера](https://cloud.google.com/iam/docs/service-accounts). Здесь мы представим краткую последовательность необходимых действий:

> Список необходимых ролей:
> - `roles/compute.admin`
> - `roles/iam.serviceAccountUser`
> - `roles/networkmanagement.admin`

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
