---
title: "Cloud provider — Yandex Cloud: Layouts"
description: "Schemes of placement and interaction of resources in Yandex Cloud when working with the Deckhouse cloud provider."
---

Three layouts are supported. Below is more information about each of them.

## Standard

{% alert level="danger" %}
In this placement strategy, nodes do not have public IP addresses allocated to them; they use NAT gateway service in Yandex Cloud to connect to the Internet.
{% endalert %}

![Yandex Cloud Standard Layout scheme](../../images/030-cloud-provider-yandex/layout-standard.png)
<!--- Source: https://docs.google.com/drawings/d/1WI8tu-QZYcz3DvYBNlZG4s5OKQ9JKyna7ESHjnjuCVQ/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }
masterNodeGroup:
  replicas: 1
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd8nb7ecsbvj76dfaa8b
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    additionalLabels:
      takes: priority
nodeGroups:
- name: worker
  replicas: 1
  zones:
  - ru-central1-a
  instanceClass:
    cores: 4
    memory: 8192
    imageID: fd8nb7ecsbvj76dfaa8b
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    additionalLabels:
      toy: example
labels:
  billing: prod
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 213.177.96.1
  - 231.177.97.1
```

## WithoutNAT

In this layout, NAT (of any kind) is not used, and each node is assigned a public IP.

> **Caution!** Currently, the cloud-provider-yandex module does not support Security Groups; thus, is why all cluster nodes connect directly to the Internet.

![Yandex Cloud WithoutNAT Layout scheme](../../images/030-cloud-provider-yandex/layout-withoutnat.png)
<!--- Source: https://docs.google.com/drawings/d/1I7M9DquzLNu-aTjqLx1_6ZexPckL__-501Mt393W1fw/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithoutNAT
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }    
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: testtest
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 8.8.8.8
  - 8.8.4.4
```

## WithNATInstance

In this placement strategy, Deckhouse creates a NAT instance and adds a rule to a route table containing a route to 0.0.0.0/0 with a NAT instance as the next hop.

If the `withNATInstance.externalSubnetID` parameter is set, the NAT instance will be created in this subnet.

IF the `withNATInstance.externalSubnetID` parameter is not set and `withNATInstance.internalSubnetID` is set, the NAT instance will be created in this last subnet.

If neither `withNATInstance.externalSubnetID` nor `withNATInstance.internalSubnetID` is set, the NAT instance will be created in the  `ru-central1-a` zone.

If the IP address of the NAT-instance does not matter, you can pass an empty object `withNATInstance: {}`, then the necessary networks and dynamic IP will be created automatically.

![Yandex Cloud WithNATInstance Layout scheme](../../images/030-cloud-provider-yandex/layout-withnatinstance.png)
<!--- Source: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: WithNATInstance
withNATInstance:
  natInstanceExternalAddress: <NAT_INSTANCE_EXTERNAL_ADDRESS>
  internalSubnetID: <INTERNAL_SUBNET_ID>
  externalSubnetID: <EXTERNAL_SUBNET_ID>
provider:
  cloudID: <CLOUD_ID>
  folderID: <FOLDER_ID>
  serviceAccountJSON: |
    {
    "id": "id",
    "service_account_id": "service_account_id",
    "key_algorithm": "RSA_2048",
    "public_key": "-----BEGIN PUBLIC KEY-----\nMIIwID....AQAB\n-----END PUBLIC KEY-----\n",
    "private_key": "-----BEGIN PRIVATE KEY-----\nMIIE....1ZPJeBLt+\n-----END PRIVATE KEY-----\n"
    }    
masterNodeGroup:
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "1.1.1.1"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "1.1.1.1"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 8.8.8.8
  - 8.8.4.4
```
