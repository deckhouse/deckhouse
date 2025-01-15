---
title: "Cloud provider — Huawei Cloud: Preparing environment"
description: "Configuring Huawei Cloud for Deckhouse cloud provider operation."
---

{% include notice_envinronment.liquid %}

To manage resources in an HuaweiCloud cloud, Deckhouse components connects to the HuaweiCloud API. To do this, you need to create a user in the HuaweiCloud IAM service and provide it with the necessary permissions.

## Configuring IAM via the web interface

In order to configure IAM via the web interface, first create a new user group and assign necessary permissions to it:

1. Open `Identity and Access Management (IAM)`.
1. Open the `User Groups` page and click `Create User Group`.
1. Enter a group name in the `Name` field (e.g., `deckhouse`).
1. Click `OK`.
1. Click on the created group.
2. Click `Authorize` on the `Permissions` tab.
3. Select `ECS Admin`, `VPC Administrator`, `NAT Admin`, and `DEW KeypairFullAccess` policies.
4. Click `Next`, then `OK`, and then `Finish`.

Then add a new user:

1. Open the `Users` page of IAM and click `Create user`.
1. Enter a name in the `Username` field (e.g., `deckhouse`).
1. Select `Access type - Programmatic access`, ensure that `Access type - Management console access` is disabled.
1. Select `Credential Type - Access key`.
1. Click `Next`.
2. Select the group created above.
3. Click `Create`.
1. Click `OK` to download the `Access Key Id` and `Secret Access Key`.

## JSON Policies

Content of the policies in JSON format:

ECS Admin policy:
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

VPC Administrator policy:
```
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

NAT Admin policy:
```
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

DEW KeypairFullAccess policy:
```
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
