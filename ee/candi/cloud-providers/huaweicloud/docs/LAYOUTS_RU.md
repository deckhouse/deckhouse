---
title: "Cloud provider — Huawei Cloud: подготовка окружения"
description: "Настройка окружения Huawei Cloud для работы облачного провайдера Deckhouse."
---

## Standard

* Создается внутренняя сеть кластера со шлюзом к публичной сети.
* Elastic IP-адрес может быть назначен мастер-узле.
* Узлы, управляемые Cluster API, не имеют публичных IP-адресов.

![resources](https://docs.google.com/drawings/d/e/2PACX-1vSUznz9tfsUtLqC7r2nHHndLdbTYN5LIwFnP68-pxZY1wZaIrG6Mxj0kvyIZV-jKDDidp8sfB0UMTdz/pub?w=812&h=655)
<!--- Source: https://docs.google.com/drawings/d/1sB_V7NhDiit8Gok2pq_8syQknCdC4GicpG3L2YF5QIU/edit --->

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
