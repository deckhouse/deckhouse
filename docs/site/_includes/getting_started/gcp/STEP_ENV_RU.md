{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Чтобы Deckhouse Kubernetes Platform смог управлять ресурсами в облаке {{ page.platform_name[page.lang] }}, необходимо создать сервисный аккаунт. Подробная инструкция по этому действию доступна в [документации](/documentation/v1/modules/030-cloud-provider-gcp/environment.html). Здесь мы представим краткую последовательность необходимых действий (выполните их на **персональном компьютере**):

{% alert %}
Список необходимых ролей:
- `roles/compute.admin`
- `roles/iam.serviceAccountUser`
- `roles/networkmanagement.admin`
{% endalert %}

Экспортируйте переменные окружения:
{% snippetcut %}
```shell
export PROJECT_ID=sandbox
export SERVICE_ACCOUNT_NAME=deckhouse
```
{% endsnippetcut %}

Выберите project:
{% snippetcut %}
```shell
gcloud config set project $PROJECT_ID
```
{% endsnippetcut %}

Создайте service account:
{% snippetcut %}
```shell
gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
```
{% endsnippetcut %}

Прикрепите роли к service account:
{% snippetcut %}
```shell
for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do \
  gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com --role=${role}; done
```
{% endsnippetcut %}

Выполните проверку ролей service account:
{% snippetcut %}
```shell
gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' \
    --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
```
{% endsnippetcut %}

Создайте service account key:
{% snippetcut %}
```shell
gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
    ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
```
{% endsnippetcut %}
