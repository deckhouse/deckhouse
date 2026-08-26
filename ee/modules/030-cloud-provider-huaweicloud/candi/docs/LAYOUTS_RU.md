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
sshPublicKey: "<SSH_PUBLIC_KEY>"
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

## Группы безопасности

При [`internalNetworkSecurity: true`](cluster_configuration.html#huaweicloudclusterconfiguration-standard-internalnetworksecurity) (значение по умолчанию) модуль создаёт группу безопасности с именем префикса кластера и назначает её узлам.

В созданной группе по умолчанию разрешены следующие входящие соединения:

* TCP/22 (SSH) из `0.0.0.0/0` по умолчанию;
* ICMP из `0.0.0.0/0`;
* TCP-порты NodePort `30000–32767` из `0.0.0.0/0` (UDP NodePort по умолчанию не открываются).

В отличие от OpenStack, правило «весь входящий трафик от узлов той же группы безопасности» в Huawei Cloud по умолчанию не создаётся.

Модуль не добавляет в управляемую группу правила для HTTP/HTTPS и других прикладных портов. Такие разрешения нужно создавать вручную в отдельной группе безопасности и подключать её к узлам.

Для CloudEphemeral-узлов дополнительные группы безопасности задаются в ресурсе [`HuaweiCloudInstanceClass`](cr.html#huaweicloudinstanceclass) параметром [`spec.securityGroups`](cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). Они применяются вместе с группой безопасности, созданной модулем.

## VpcPeering

![Схема размещения VpcPeering](images/huawei-vpc-peering-ru.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11715&t=IvETjbByf1MSQzcm-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: HuaweiCloudClusterConfiguration
layout: VpcPeering
sshPublicKey: "<SSH_PUBLIC_KEY>"
vpcPeering:
  internalNetworkDNSServers:
    - 8.8.8.8
  internalNetworkCIDR: 10.221.128.0/24
  internalNetworkSecurity: true
  subnet: subnet-43b4
```
