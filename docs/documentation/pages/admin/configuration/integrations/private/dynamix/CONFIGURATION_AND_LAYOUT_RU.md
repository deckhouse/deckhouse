---
title: Схемы размещения Basis Dynamix
permalink: ru/admin/integrations/private/dynamix/layout.html
lang: ru
---

Deckhouse Platform поддерживает две [схемы размещения](../../../../glossary.html#layout) в облаке Basis Dynamix:

- Standard — схема с использованием только внешней сети;
- StandardWithInternalNetwork — схема с внутренней (приватной) сетью и DNS-серверами.

Обе схемы позволяют управлять размещением узлов кластера, настройкой сетей, образом ОС и хранилищем данных.

## Standard

![resources](../../../../images/cloud-provider-dynamix/dynamix-standard.png)
<!--- Исходник: https://www.figma.com/design/T3ycFB7P6vZIL359UJAm7g/%D0%98%D0%BA%D0%BE%D0%BD%D0%BA%D0%B8-%D0%B8-%D1%81%D1%85%D0%B5%D0%BC%D1%8B?node-id=995-11150&t=Qb5yyWumzPiTBtfL-0 --->

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"
location: dynamix
account: acc_user
storagePolicy: storage_policy01
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    externalNetwork: "<external network>"
```

## StandardWithInternalNetwork

Пример конфигурации схемы размещения:

```yaml
apiVersion: deckhouse.io/v1
kind: DynamixClusterConfiguration
layout: StandardWithInternalNetwork
sshPublicKey: "<SSH_PUBLIC_KEY>"
location: dynamix
account: acc_user
storagePolicy: storage_policy01
nodeNetworkCIDR: "10.241.32.0/24"
nameservers:
  - "10.0.0.10"
provider:
  controllerUrl: "<controller url>"
  oAuth2Url: "<oAuth2 url>"
  appId: "<app id>"
  appSecret: "<app secret>"
  insecure: true
masterNodeGroup:
  replicas: 1
  instanceClass:
    numCPUs: 6
    memory: 16384
    rootDiskSizeGb: 50
    imageName: "<image name>"
    externalNetwork: "<external network>"
```

[Настройка интеграции](configuration.html).
