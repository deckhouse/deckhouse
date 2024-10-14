---
title: "Cloud provider — Yandex Cloud: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Yandex Cloud при работе облачного провайдера Deckhouse."
---

Поддерживаются три схемы размещения. Ниже подробнее о каждой их них.

## Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов, а будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](../../images/030-cloud-provider-yandex/layout-standard.png)
<!--- Исходник: https://docs.google.com/drawings/d/1WI8tu-QZYcz3DvYBNlZG4s5OKQ9JKyna7ESHjnjuCVQ/edit --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: YandexClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
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
    imageID: testtest
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
    imageID: testtest
    coreFraction: 50
    externalIPAddresses:
    - "198.51.100.5"
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    additionalLabels:
      toy: example
labels:
  billing: prod
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - 213.177.96.1
  - 231.177.97.1
```

## WithoutNAT

В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдается публичный IP-адрес.

> **Внимание!** В модуле `cloud-provider-yandex` пока нет поддержки групп безопасности (security group), поэтому все узлы кластера будут смотреть *наружу*.

![Схема размещения WithoutNAT в Yandex Cloud](../../images/030-cloud-provider-yandex/layout-withoutnat.png)
<!--- Исходник: https://docs.google.com/drawings/d/1I7M9DquzLNu-aTjqLx1_6ZexPckL__-501Mt393W1fw/edit --->

Пример конфигурации схемы размещения:

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

В данной схеме размещения создается NAT-инстанс, а в таблицу маршрутизации добавляется правило на 0.0.0.0/0 с NAT-инстанса nexthop'ом.

Если задан `withNATInstance.externalSubnetID` — NAT-инстанс будет создан в зоне этого subnet.

Если `withNATInstance.externalSubnetID` не задан, а `withNATInstance.internalSubnetID` задан — NAT-инстанс будет создан в зоне этого subnet.

Если ни `withNATInstance.externalSubnetID`, ни `withNATInstance.internalSubnetID` не заданы — NAT-инстанс создастся в зоне `ru-central1-a`.

Если IP-адрес NAT-инстанса не имеет значения, можно передать пустой объект `withNATInstance: {}`, тогда необходимые сети и динамический IP-адрес будут созданы автоматически.

![Схема размещения WithNATInstance в Yandex Cloud](../../images/030-cloud-provider-yandex/layout-withnatinstance.png)
<!--- Исходник: https://docs.google.com/drawings/d/1oVpZ_ldcuNxPnGCkx0dRtcAdL7BSEEvmsvbG8Aif1pE/edit --->

Пример конфигурации схемы размещения:

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
