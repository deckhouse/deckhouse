{%- include getting_started/global/partials/NOTICES_ENVIRONMENT.liquid %}

Deckhouse Kubernetes Platform components interact with Huawei Cloud resources through the Huawei Cloud API. To configure this connection, you need to create a user in the Huawei Cloud IAM service and provide it with the necessary permissions.

## Configuring IAM via the web interface

To configure IAM via the web interface, first create a new user group and assign the necessary permissions. Follow these steps:

1. Go to the "Identity and Access Management (IAM)" section.
1. Open the "User Groups" page and click "Create User Group".
1. In the "Name" field, enter the group name (e.g., `deckhouse`).
1. Click "OK" to create the group.
1. Select the newly created group from the list.
1. On the "Permissions" tab, click "Authorize".
1. Assign the following policies: "ECS Admin", "VPC Administrator", "NAT Admin", "ELB FullAccess", "DEW KeypairFullAccess".
1. Click "Next", then "OK", and complete the setup by clicking "Finish".

Then add a new user. Follow these steps:

1. Go to the "Users" page in the IAM section and click "Create User".
1. In the "Username" field, enter the username (e.g., `deckhouse`).
1. Set "Access type" to "Programmatic access" and make sure "Management console access" is disabled.
1. Select "Access key" as the "Credential Type".
1. Click "Next".
1. Select the previously created user group.
1. Click "Create" to complete the user creation process.
1. Click "OK" to download the `Access Key ID` and `Secret Access Key`. Make sure to save these credentials in a secure location, as they will be needed to access the API.

Obtain the ID of the project where the cluster will be deployed. To do this, follow these steps:

1. Go to the "Identity and Access Management (IAM)" section.
1. In the left-hand menu, select "Projects".
1. From the project list, select the project that corresponds to the region where the cluster will be deployed.
1. On the project page, in the "Basic Information" section, copy the value of the `Project ID` field.
1. Use the obtained value as the `projectID` parameter when configuring the cluster.

{% alert level="info" %}
The `enterpriseProjectID` parameter is optional. The cluster can be deployed without specifying it. Use this parameter if the cluster resources need to be placed in a specific Enterprise Project.
{% endalert %}

To obtain the Enterprise Project ID, follow these steps:

1. In the upper-right corner of the Huawei Cloud console, open the profile menu.
1. Select "Enterprise Management".
1. On the "Enterprise Project Management Service" page, select the Enterprise Project where the cluster resources should be placed.
1. On the selected Enterprise Project page, copy the value of the `ID` field.
1. Use the obtained value as the `enterpriseProjectID` parameter when configuring the cluster.

## JSON policies

Below are the contents of the policies in JSON format:

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
