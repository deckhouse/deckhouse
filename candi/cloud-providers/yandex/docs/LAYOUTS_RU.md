---
title: "Cloud provider — Yandex Cloud: схемы размещения"
description: "Описание схем размещения и взаимодействия ресурсов в Yandex Cloud при работе облачного провайдера Deckhouse."
---

Поддерживаются три схемы размещения. Ниже подробнее о каждой их них.

## Standard

{% alert level="danger" %}
В данной схеме размещения узлы не будут иметь публичных IP-адресов и будут выходить в интернет через NAT-шлюз (NAT Gateway) Yandex Cloud. NAT-шлюз (NAT Gateway) использует случайные публичные IP-адреса из [выделенных диапазонов](https://yandex.cloud/ru/docs/overview/concepts/public-ips#virtual-private-cloud). Из-за этого невозможно добавить в белый список (whitelist) адреса облачных ресурсов, находящихся за конкретным NAT-шлюзом, на стороне других сервисов.
{% endalert %}

![Схема размещения Standard в Yandex Cloud](images/yandex-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10422&t=IvETjbByf1MSQzcm-0 --->

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

![Схема размещения WithoutNAT в Yandex Cloud](images/yandex-withoutnat.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10557&t=IvETjbByf1MSQzcm-0 --->

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
Подсеть выделяется для предотвращения петли маршрутизации и не должна пересекаться с другими сетями, используемыми в кластере.

Для размещения NAT-инстанса в существующей подсети используйте параметр `withNATInstance.internalSubnetID` — инстанс будет создан в зоне, соответствующей этой подсети.

Если необходимо создать новую подсеть, укажите параметр `withNATInstance.internalSubnetCIDR` — в ней будет размещён NAT-инстанс.

Обязателен один из параметров: `withNATInstance.internalSubnetID` или `withNATInstance.internalSubnetCIDR`.

Если `withNATInstance.externalSubnetID` указан в дополнение к предыдущим, NAT-инстанс будет подключен к нему через вторичный интерфейс.

![Схема размещения WithNATInstance в Yandex Cloud](images/yandex-withnatinstance.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10034&t=IvETjbByf1MSQzcm-0 --->

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
