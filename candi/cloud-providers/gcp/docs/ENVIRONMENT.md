---
title: "Cloud provider — GCP: Preparing environment"
---

You need to create a service account so that Deckhouse can manage resources in the Google Cloud. The detailed instructions for creating a service account are available in the [provider's documentation](https://cloud.google.com/iam/docs/service-accounts). Below is a brief sequence of required actions:

> **Note!** 'service account key` cannot be restored, you can only delete and create a new one.

## Setup using Google cloud console

Follow this [link](https://console.cloud.google.com/iam-admin/serviceaccounts), select your project and create a new service account or select an existing one.

List of roles required:
```
Compute Admin
Service Account User
Network Management Admin
```

You can add roles when creating a service account or edit them [here](https://console.cloud.google.com/iam-admin/iam).

To create a `service account key` in JSON format, click on [three vertical dots](https://console.cloud.google.com/iam-admin/serviceaccounts) in the Actions column and select Manage keys. Next, click on Add key -> Create new key -> Key type -> JSON.

## Setup using gcloud CLI

List of roles required:
```
roles/compute.admin
roles/iam.serviceAccountUser
roles/networkmanagement.admin
```

- Export environment variables:

  ```shell
  export PROJECT=sandbox
  export SERVICE_ACCOUNT_NAME=deckhouse
  ```
- Select a project:

  ```shell
  gcloud config set project $PROJECT
  ```
- Create a service account:

  ```shell
  gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
  ```
- Connect roles to the service account:

  ```shell
  for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin; do gcloud projects add-iam-policy-binding ${PROJECT} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com --role=${role}; done
  ```
- Verify service account roles:

  ```shell
  gcloud projects get-iam-policy ${PROJECT} --flatten="bindings[].members" --format='table(bindings.role)' \
      --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com"
  ```
- Create a service account key:

  ```shell
  gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT}.iam.gserviceaccount.com \
      ~/service-account-key-${PROJECT}-${SERVICE_ACCOUNT_NAME}.json
  ```
