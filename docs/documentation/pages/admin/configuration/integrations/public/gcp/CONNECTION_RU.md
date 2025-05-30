---
title: Подключение и авторизация
permalink: ru/admin/integrations/public/gcp/сonnection-and-authorization.html
lang: ru
---

Для управления ресурсами в Google Cloud с помощью Deckhouse Kubernetes Platform необходимо создать service account.

Далее представлена краткая последовательность действий по созданию и использованию service account.

## Создание Service Account

Подробную инструкцию по созданию Service Account можно найти [в документации провайдера](https://cloud.google.com/iam/docs/service-accounts).

{% alert level="warning" %}
**Внимание!** Созданный `service account key` невозможно восстановить, только удалить и создать новый.
{% endalert %}

### Настройка через Google Cloud Console

Перейдите [по ссылке](https://console.cloud.google.com/iam-admin/serviceaccounts), выберите проект и создайте новый service account (также можно выбрать уже существующий).

Созданному service account'у должны быть присвоены несколько необходимых ролей:

```text
Compute Admin
Service Account User
Network Management Admin
```

Роли можно присвоить на этапе создания service account'а либо изменить [на этой странице](https://console.cloud.google.com/iam-admin/iam).

Чтобы получить `service account key` в JSON-формате, [на странице](https://console.cloud.google.com/iam-admin/serviceaccounts) в колонке «Actions» нажмите  на три вертикальные точки и выберите «Manage keys». Затем нажмите «Add key» -> «Create new key» -> «Key type» -> «JSON».

### Настройка через gcloud CLI

Установите и инициализируйте gcloud CLI, следуя [официальной инструкции](https://cloud.google.com/sdk/docs/install-sdk).

Для создания service account через интерфейс командной строки выполните следующие шаги:

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

## Использование созданного Service Account

Полученный `service account key` указывается в секции `provider.serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"` ресурса [GCPClusterConfiguration](TODO).

Пример:

```yaml
apiVersion: deckhouse.io/v1
kind: GCPClusterConfiguration
layout: WithoutNAT
sshKey: "<SSH_PUBLIC_KEY>"
subnetworkCIDR: 10.36.0.0/24
masterNodeGroup:
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
nodeGroups:
- name: static
  replicas: 1
  zones:
  - europe-west3-b
  instanceClass:
    machineType: n1-standard-4
    image: projects/ubuntu-os-cloud/global/images/ubuntu-2404-noble-amd64-v20240523a
    diskSizeGb: 50
    additionalNetworkTags:
    - tag1
    additionalLabels:
      kube-node: static
provider:
  region: europe-west3
  serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"
```
