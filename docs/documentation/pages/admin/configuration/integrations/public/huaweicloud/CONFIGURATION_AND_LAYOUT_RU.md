---
title: Схемы размещения и настройка
permalink: ru/admin/integrations/public/huaweicloud/huawei-layout.html
lang: ru
---

Данный раздел описывает схемы размещения кластера в инфраструктуре Huawei Cloud и связанные с ними параметры.

## Standard

- Создаётся внутренняя сеть кластера со шлюзом к публичной сети.
- Master-узлу можно назначить Elastic IP.
- Узлы, управляемые Cluster API, не имеют публичных IP-адресов.

Пример конфигурации:

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

- Используется существующая сеть и подсеть VPC.
- Виртуальные машины подключаются к заданной подсети.

Пример конфигурации:

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

