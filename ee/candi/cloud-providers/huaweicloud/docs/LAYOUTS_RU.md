---
title: "Cloud provider — Huawei Cloud: схемы размещения"
description: "Схемы размещения Huawei Cloud для работы облачного провайдера Deckhouse."
---

## Standard

* Создается внутренняя сеть кластера со шлюзом к публичной сети.
* Elastic IP-адрес можно назначить master-узлу.
* Узлы, управляемые Cluster API, не имеют публичных IP-адресов.

![Схема размещения Standard](images/huawei-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-10811&t=IvETjbByf1MSQzcm-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: Standard
sshPublicKey: "<Public SSH key>"
standard:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 192.168.199.0/24
  internalNetworkSecurity: true
  enableEIP: true
provider:
  cloud: hc.sbercloud.ru
  region: ru-moscow-1
  accessKey: "<Access key>"
  secretKey: "<Secret key>"
  projectID: "<Project ID>"
masterNodeGroup:
  replicas: 1
  instanceClass:
    imageName: alt-p11
    flavorName: s7n.xlarge.2
    rootDiskSize: 50
  serverGroup:
    policy: AntiAffinity
  volumeTypeMap:
    ru-moscow-1a: SSD
```

## VpcPeering

![Схема размещения VpcPeering](images/huawei-vpc-peering-ru.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11715&t=IvETjbByf1MSQzcm-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: VpcPeering
sshPublicKey: "<Public SSH key>"
vpcPeering:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 10.221.128.0/24
  internalNetworkSecurity: true
  subnet: subnet-43b4
```
