---
title: "Cloud provider — Yandex Cloud: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Yandex Cloud при работе облачного провайдера Deckhouse."
---

Поддерживаются три схемы размещения. Ниже подробнее о каждой их них.

## Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из [выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](../../images/cloud-provider-yandex/layout-standard.png)
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
  replicas: 3
  zones:
  - ru-central1-a
  - ru-central1-b
  - ru-central1-d
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "<ZONE_A_EXTERNAL_IP_MASTER_1>"
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    - <ZONE_D_SUBNET_ID>
    additionalLabels:
      takes: priority
nodeGroups:
- name: worker
  replicas: 2
  zones:
  - ru-central1-a
  - ru-central1-b
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    additionalLabels:
      role: example
labels:
  billing: prod
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```

## WithoutNAT

В данной схеме размещения NAT (любого вида) не используется, а каждому узлу выдается публичный IP-адрес.

> **Внимание!** В модуле `cloud-provider-yandex` нет поддержки групп безопасности (security group), поэтому все узлы кластера будут доступны без ограничения подключения.

![Схема размещения WithoutNAT в Yandex Cloud](../../images/cloud-provider-yandex/layout-withoutnat.png)
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
  replicas: 3
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    externalIPAddresses:
    - "Auto"
    - "Auto"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    - <ZONE_D_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
    - ru-central1-d
nodeGroups:
- name: worker
  replicas: 2
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
    - "<ZONE_A_EXTERNAL_IP_WORKER_1>"
    - "Auto"
    externalSubnetIDs:
    - <ZONE_A_SUBNET_ID>
    - <ZONE_B_SUBNET_ID>
    zones:
    - ru-central1-a
    - ru-central1-b
sshPublicKey: "<SSH_PUBLIC_KEY>"
nodeNetworkCIDR: 192.168.12.13/24
existingNetworkID: <EXISTING_NETWORK_ID>
dhcpOptions:
  domainName: test.local
  domainNameServers:
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```

## WithNATInstance

В данной схеме размещения в отдельной подсети создается NAT-инстанс, а в таблицу маршрутизации подсетей зон добавляется правило с маршрутом на 0.0.0.0/0 с NAT-инстансом в качестве nexthop'а.
Эта отдельная подсеть необходима для избежания петли маршрутизации.

Если задан `withNATInstance.internalSubnetID` — NAT-инстанс будет создан в зоне этого subnet.

Если задан `withNATInstance.internalSubnetCIDR` — тогда будет создана новая внутренняя подсеть. NAT-инстанс будет создан в этой подсети.

Один из параметров, `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`, обязателен

Если `withNATInstance.externalSubnetID` указан в дополнение к предыдущим, NAT-инстанс будет подключен к нему через вторичный интерфейс.

![Схема размещения WithNATInstance в Yandex Cloud](../../images/cloud-provider-yandex/layout-withnatinstance.png)
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
    - "Auto"
    externalSubnetID: <EXTERNAL_SUBNET_ID>
    zones:
    - ru-central1-a
nodeGroups:
- name: worker
  replicas: 1
  instanceClass:
    cores: 4
    memory: 8192
    imageID: <IMAGE_ID>
    coreFraction: 50
    externalIPAddresses:
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
  - <DNS_SERVER_1>
  - <DNS_SERVER_2>
```
