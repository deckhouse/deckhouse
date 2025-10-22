---
title: "Cloud provider — GCP: Preparing environment"
description: "Configuring GCP for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

You need to create a service account so that Deckhouse can manage resources in the Google Cloud. Below is a brief sequence of steps to create a service account. If you need detailed instructions, you can find them in the [provider's documentation](https://cloud.google.com/iam/docs/service-accounts).

{% alert level="warning" %}
**Note!** The created `service account key` cannot be restored, you can only delete and create a new one.
{% endalert %}

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

## Setup using Google Cloud Console

Follow this [link](https://console.cloud.google.com/iam-admin/serviceaccounts), select your project and create a new service account or select an existing one.

The account must be assigned several necessary roles:

```text
Compute Admin
Service Account User
Network Management Admin
```

You can add roles when creating a service account or edit them [here](https://console.cloud.google.com/iam-admin/iam).

To create a `service account key` in JSON format, click on [three vertical dots](https://console.cloud.google.com/iam-admin/serviceaccounts) in the Actions column and select `Manage keys`. Next, click on `Add key` -> `Create new key` -> `Key type` -> `JSON`.

## Setup using gcloud CLI

To configure via the command line interface, follow these steps:

1. Export environment variables:

   ```shell
   export PROJECT_ID=sandbox
   export SERVICE_ACCOUNT_NAME=deckhouse
   ```

2. Select a project:

   ```shell
   gcloud config set project $PROJECT_ID
   ```

3. Create a service account:

   ```shell
   gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
   ```

4. Connect roles to the service account:

   ```shell
   for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin;
   do gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
      --role=${role}; done
   ```

   List of roles required:

   ```text
   roles/compute.admin
   roles/iam.serviceAccountUser
   roles/networkmanagement.admin
   ```

5. Verify service account roles:

   ```shell
   gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' \
         --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
   ```

6. Create a `service account key`:

   ```shell
   gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
         ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
   ```
