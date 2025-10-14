---
title: Connection and authorization
permalink: en/admin/integrations/public/gcp/connection-and-authorization.html
description: "Configure GCP connection and authorization for Deckhouse Kubernetes Platform. Service Account setup, credentials configuration, and Google Cloud integration requirements for cloud deployment."
---

To manage Google Cloud resources using Deckhouse Kubernetes Platform, you need to create a Service Account.

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

## Creating a service account

You can find detailed instructions on how to create a service account in the [official Google Cloud documentation](https://cloud.google.com/iam/docs/service-accounts).

{% alert level="warning" %}
A created `service account key` cannot be recovered.
If lost, it must be deleted and recreated.
{% endalert %}

### Setup via Google Cloud console

Go to the [Google Cloud console](https://console.cloud.google.com/iam-admin/serviceaccounts), select your project,
and create a new service account (you can also choose an existing one).

The created service account must be assigned the following roles:

```text
Compute Admin
Service Account User
Network Management Admin
```

You can assign roles during service account creation or [modify](https://console.cloud.google.com/iam-admin/iam) them later.

To generate the `service account key` in JSON format, on the [service accounts page](https://console.cloud.google.com/iam-admin/serviceaccounts),
click the three vertical dots in the **Actions** column and select **Manage keys**.
Then click **Add key** → **Create new key** → **Key type** → **JSON**.

### Setup via Google Cloud CLI

Install and initialize the Google Cloud CLI by following the [official instructions](https://cloud.google.com/sdk/docs/install-sdk).

To create a service account using the CLI, follow these steps:

1. Export environment variables:

   ```shell
   export PROJECT_ID=sandbox
   export SERVICE_ACCOUNT_NAME=deckhouse
   ```

1. Set the project:

   ```shell
   gcloud config set project $PROJECT_ID
   ```

1. Create a service account:

   ```shell
   gcloud iam service-accounts create $SERVICE_ACCOUNT_NAME
   ```

1. Assign roles to the created service account:

   ```shell
   for role in roles/compute.admin roles/iam.serviceAccountUser roles/networkmanagement.admin;
   do gcloud projects add-iam-policy-binding ${PROJECT_ID} --member=serviceAccount:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
      --role=${role}; done
   ```

   A list of required roles:

   ```text
   roles/compute.admin
   roles/iam.serviceAccountUser
   roles/networkmanagement.admin
   ```

1. Verify the assigned roles:

   ```shell
   gcloud projects get-iam-policy ${PROJECT_ID} --flatten="bindings[].members" --format='table(bindings.role)' \
         --filter="bindings.members:${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"
   ```

1. Generate a `service account key`:

   ```shell
   gcloud iam service-accounts keys create --iam-account ${SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com \
         ~/service-account-key-${PROJECT_ID}-${SERVICE_ACCOUNT_NAME}.json
   ```

## Using the service account

The generated `service account key` must be specified in the `provider.serviceAccountJSON: "<SERVICE_ACCOUNT_JSON>"` section
of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.

Example:

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
