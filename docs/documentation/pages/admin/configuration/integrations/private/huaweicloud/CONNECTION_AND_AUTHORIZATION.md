---
title: Connection and authorization
permalink: en/admin/integrations/private/huaweicloud/authorization.html
---

## Requirements

{% alert level="warning" %}
The provider supports working with only one disk in the virtual machine template. Make sure the template contains only one disk.
{% endalert %}

To ensure proper operation of Deckhouse Kubernetes Platform (DKP) with Huawei Cloud, make sure of the following:

- The `cloud-init` package is installed on the virtual machines.
- After the VMs start, the following services must be active:
  - `cloud-config.service`
  - `cloud-final.service`
  - `cloud-init.service`

## Accessing the Huawei Cloud API

DKP uses the Huawei Cloud API to manage resources.
To configure access, you need to create an IAM user and assign the necessary permissions.

### Creating a user group

To create a user group and assign policies, follow these steps:

1. Go to the **Identity and Access Management (IAM)** section.
1. Open the **User Groups** tab and click **Create User Group**.
1. Specify a group name (for example, `deckhouse`) and click **OK**.
1. Select the created group and go to the **Permissions** tab.
1. Click **Authorize** and assign the following policies:
   - `ECS Admin`
   - `VPC Administrator`
   - `NAT Admin`
   - `ELB FullAccess`
   - `DEW KeypairFullAccess`
1. Confirm your selections by clicking **Next**, then **OK**, and finalize with **Finish**.

### Creating an IAM user

To create an IAM user, follow these steps:

1. Go to the **Users** tab and click **Create User**.
1. Enter a username (for example, `deckhouse`).
1. Under **Access type**, select **Programmatic access**, and ensure that **Management console access** is disabled.
1. Under **Credential Type**, choose **Access key**.
1. Click **Next**, select the previously created group, and then click **Create**.
1. Download the **Access Key ID** and **Secret Access Key**.
   These credentials are required to access the Huawei Cloud API and cannot be recovered later.

{% alert level="info" %}
Make sure the saved keys are securely stored, as they are required to access the cloud API.
{% endalert %}

## JSON policies

Below are the policy contents in JSON format:

{% offtopic title="ECS Admin policy" %}

```json
  {
  "Version": "1.1",
  "Statement": [
  {
      "Action": [
      "ecs:*:*",
      "evs:*:get",
      "evs:*:list",
      "evs:volumes:create",
      "evs:volumes:delete",
      "evs:volumes:attach",
      "evs:volumes:detach",
      "evs:volumes:manage",
      "evs:volumes:update",
      "evs:volumes:use",
      "evs:volumes:uploadImage",
      "evs:snapshots:create",
      "vpc:*:get",
      "vpc:*:list",
      "vpc:networks:create",
      "vpc:networks:update",
      "vpc:subnets:update",
      "vpc:subnets:create",
      "vpc:ports:*",
      "vpc:routers:get",
      "vpc:routers:update",
      "vpc:securityGroups:*",
      "vpc:securityGroupRules:*",
      "vpc:floatingIps:*",
      "vpc:publicIps:*",
      "ims:images:create",
      "ims:images:delete",
      "ims:images:get",
      "ims:images:list",
      "ims:images:update",
      "ims:images:upload"
      ],
      "Effect": "Allow"
  }
  ]
  }
```

{% endofftopic %}

{% offtopic title="VPC Administrator policy" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "vpc:vpcs:*",
                  "vpc:routers:*",
                  "vpc:networks:*",
                  "vpc:subnets:*",
                  "vpc:ports:*",
                  "vpc:privateIps:*",
                  "vpc:peerings:*",
                  "vpc:routes:*",
                  "vpc:lbaas:*",
                  "vpc:vpns:*",
                  "ecs:*:get",
                  "ecs:*:list",
                  "elb:*:get",
                  "elb:*:list"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="NAT Admin policy" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "nat:*:*",
                  "vpc:*:*"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="DEW KeypairFullAccess policy" %}

```json
  {
      "Version": "1.1",
      "Statement": [
          {
              "Action": [
                  "kps:domainKeypairs:*",
                  "ecs:serverKeypairs:*"
              ],
              "Effect": "Allow"
          }
      ]
  }
```

{% endofftopic %}

{% offtopic title="ELB FullAccess policy" %}

```json
  {
    "Version": "1.1",
    "Statement": [
        {
            "Action": [
                "elb:*:*",
                "vpc:*:get*",
                "vpc:*:list*",
                "ecs:*:get*",
                "ecs:*:list*"
            ],
            "Effect": "Allow"
        }
    ]
  }
```

{% endofftopic %}
